package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"api-students/app/model"
	"api-students/app/repository"

	"github.com/hibiken/asynq"
)

// TaskEnqueuer adalah kemampuan melakukan enqueue task.
// *asynq.Client memenuhi interface ini.
type TaskEnqueuer interface {
	EnqueueContext(ctx context.Context, task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error)
}

// OutboxRelay memindahkan event dari tabel outbox ke queue Redis.
//
// Relay adalah satu-satunya komponen yang membaca outbox. API tidak pernah
// berbicara dengan Redis; ia cukup menulis ke PostgreSQL. Karena itu mahasiswa
// tetap dapat mengisi KRS walaupun Redis sedang mati: notifikasinya tertunda,
// tidak hilang.
type OutboxRelay struct {
	outbox   repository.OutboxRepository
	tx       *repository.TxManager
	queue    TaskEnqueuer
	logger   *slog.Logger
	interval time.Duration
	batch    int
}

func NewOutboxRelay(
	outbox repository.OutboxRepository,
	tx *repository.TxManager,
	queue TaskEnqueuer,
	logger *slog.Logger,
	interval time.Duration,
	batch int,
) *OutboxRelay {
	return &OutboxRelay{
		outbox: outbox, tx: tx, queue: queue,
		logger: logger, interval: interval, batch: batch,
	}
}

// Run berjalan terus sampai ctx dibatalkan.
func (r *OutboxRelay) Run(ctx context.Context) {
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	for {
		n, err := r.RelayOnce(ctx)
		if err != nil {
			r.logger.Warn("relay_delayed",
				slog.Int("published", n), slog.String("error", err.Error()))
		} else if n > 0 {
			r.logger.Info("relay_done", slog.Int("published", n))
		}

		// Batch penuh berarti kemungkinan besar masih ada sisa: langsung
		// lanjutkan tanpa menunggu. Selain itu, tunggu giliran berikutnya.
		if err == nil && n == r.batch && ctx.Err() == nil {
			continue
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// RelayOnce mem-publish paling banyak satu batch. Mengembalikan jumlah event
// yang berhasil di-publish.
func (r *OutboxRelay) RelayOnce(ctx context.Context) (int, error) {
	var published int
	var enqueueErr error

	err := r.tx.WithTxOnce(ctx, func(tx repository.DBTX) error {
		repo := r.outbox.WithTx(tx)

		events, err := repo.FetchUnpublished(ctx, r.batch)
		if err != nil {
			return err
		}

		ids := make([]int64, 0, len(events))
		for _, e := range events {
			// Berhenti pada kegagalan pertama. Bila Redis mati, event
			// berikutnya pun pasti gagal; yang belum di-publish tetap NULL
			// dan diambil lagi pada putaran berikutnya.
			if err := r.publish(ctx, e); err != nil {
				enqueueErr = err
				break
			}
			// ID baru masuk daftar SETELAH publish berhasil, supaya hanya
			// event yang benar-benar ada di Redis yang ditandai published.
			ids = append(ids, e.ID)
		}

		// Hanya event yang SUDAH berhasil di-enqueue yang ditandai.
		if err := repo.MarkPublished(ctx, ids); err != nil {
			return err
		}
		published = len(ids)
		return nil
	})
	if err != nil {
		return 0, err
	}
	return published, enqueueErr
}

// SplitEvent memecah satu event outbox menjadi daftar job notifikasi.
//
// Event KRS biasa menjangkau satu mahasiswa, jadi hasilnya satu job.
// Event course.quota_changed menjangkau seluruh peserta kelas, jadi
// hasilnya satu job per peserta. Karena itu email ke-7 yang gagal hanya
// me-retry task peserta ke-7, bukan mengulang email peserta ke-1..ke-6.
func SplitEvent(e model.OutboxEvent) ([]model.NotificationJob, error) {
	if e.EventType != model.EventCourseQuotaChanged {
		return []model.NotificationJob{{
			OutboxID: e.ID, EventType: e.EventType, Payload: e.Payload,
		}}, nil
	}

	var ev model.EnrollmentEvent
	if err := json.Unmarshal(e.Payload, &ev); err != nil {
		return nil, fmt.Errorf("mengurai event kuota %d: %w", e.ID, err)
	}

	jobs := make([]model.NotificationJob, 0, len(ev.UserIDs))
	for _, uid := range ev.UserIDs {
		one := ev
		one.UserID = uid
		one.UserIDs = nil
		payload, err := json.Marshal(one)
		if err != nil {
			return nil, fmt.Errorf("mengemas event kuota %d: %w", e.ID, err)
		}
		jobs = append(jobs, model.NotificationJob{
			OutboxID: e.ID, EventType: e.EventType, UserID: uid, Payload: payload,
		})
	}
	return jobs, nil
}

// taskID menyusun TaskID task dari id outbox dan penerima.
//
// TaskID dari id outbox: enqueue ulang event yang sama ditolak Redis.
// Ini menutup celah relay mati SETELAH enqueue tetapi SEBELUM commit.
// Event kuota memakai satu TaskID per peserta.
func taskID(outboxID int64, userID int) string {
	if userID != 0 {
		return fmt.Sprintf("outbox-%d-%d", outboxID, userID)
	}
	return fmt.Sprintf("outbox-%d", outboxID)
}

// publish meng-enqueue satu event sebagai task notifikasi. Event yang
// menjangkau banyak peserta dipecah jadi satu task per peserta.
func (r *OutboxRelay) publish(ctx context.Context, e model.OutboxEvent) error {
	jobs, err := SplitEvent(e)
	if err != nil {
		return err
	}

	for _, job := range jobs {
		// Payload task harus []byte, jadi struct di-marshal menjadi JSON dulu.
		payload, err := json.Marshal(job)
		if err != nil {
			return fmt.Errorf("mengemas job outbox %d: %w", e.ID, err)
		}

		task := asynq.NewTask(model.TaskSendNotification, payload,
			asynq.TaskID(taskID(e.ID, job.UserID)),
			asynq.Queue(model.QueueNotifications),
			asynq.MaxRetry(5),
			asynq.Timeout(30*time.Second),
			// Task yang selesai disimpan 24 jam, sehingga TaskID tetap tercatat
			// dan perlindungan di atas tetap berlaku.
			asynq.Retention(24*time.Hour),
		)
		if _, err := r.queue.EnqueueContext(ctx, task); err != nil {
			if errors.Is(err, asynq.ErrTaskIDConflict) {
				// Sudah pernah di-enqueue pada putaran sebelumnya. Bagi relay
				// itu sama dengan berhasil.
				continue
			}
			return fmt.Errorf("enqueue outbox %d: %w", e.ID, err)
		}
	}
	return nil
}
