package config

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"api-students/app/model"

	"github.com/hibiken/asynq"
)

func RedisOpt() asynq.RedisClientOpt {
	return asynq.RedisClientOpt{
		Addr:     GetENV("REDIS_ADDR", "localhost:6379"),
		Password: GetENV("REDIS_PASSWORD", ""),
		DB:       GetENVInt("REDIS_DB", 0),

		// Timeout pendek: saat Redis mati, segera tahu, bukan menunggu lama.
		DialTimeout:  2 * time.Second,
		ReadTimeout:  2 * time.Second,
		WriteTimeout: 2 * time.Second,
	}
}

// NewWorkerServer menyiapkan server Asynq, "Fiber"-nya worker.
func NewWorkerServer(logger *slog.Logger) *asynq.Server {
	return asynq.NewServer(RedisOpt(), asynq.Config{
		// Jumlah task yang dikerjakan bersamaan. Setiap task notifikasi
		// memegang satu koneksi PostgreSQL selama pengiriman, sehingga angka
		// ini harus lebih kecil dari DB_MAX_CONNS.
		Concurrency: GetENVInt("WORKER_CONCURRENCY", 5),

		// Worker HANYA mengambil task dari queue yang disebut di sini.
		// Nama queue harus sama persis dengan yang dipakai relay:
		// model.QueueNotifications, BUKAN model.TaskSendNotification.
		Queues: map[string]int{model.QueueNotifications: 1},

		// Delay sebelum retry. n adalah jumlah retry yang SUDAH terjadi,
		// dimulai dari 0, sehingga delay-nya 2, 4, 6, 8, 10 detik.
		RetryDelayFunc: func(n int, err error, t *asynq.Task) time.Duration {
			return time.Duration((n+1)*2) * time.Second
		},

		// Saat worker dimatikan, task yang sedang-running diberi 10 detik
		// untuk selesai. Yang belum selesai dikembalikan ke queue dan akan
		// dikerjakan ulang, alasan lain handler harus idempoten.
		ShutdownTimeout: 10 * time.Second,

		ErrorHandler: asynq.ErrorHandlerFunc(func(ctx context.Context, t *asynq.Task, err error) {
			retry, _ := asynq.GetRetryCount(ctx)
			maxRetry, _ := asynq.GetMaxRetry(ctx)
			logger.Warn("task_failed",
				slog.String("type", t.Type()),
				slog.String("attempt", fmt.Sprintf("%d/%d", retry+1, maxRetry+1)),
				slog.String("error", err.Error()),
			)
		}),

		Logger:   newAsynqLogger(logger),
		LogLevel: asynq.WarnLevel,
	})
}

// asynqLogger menyalurkan log internal Asynq ke slog, agar seluruh aplikasi
// tetap satu format dan satu tujuan.
type asynqLogger struct{ l *slog.Logger }

func newAsynqLogger(l *slog.Logger) asynqLogger {
	return asynqLogger{l: l.With(slog.String("component", "asynq"))}
}

func (a asynqLogger) Debug(args ...any) { a.l.Debug(fmt.Sprint(args...)) }
func (a asynqLogger) Info(args ...any)  { a.l.Info(fmt.Sprint(args...)) }
func (a asynqLogger) Warn(args ...any)  { a.l.Warn(fmt.Sprint(args...)) }
func (a asynqLogger) Error(args ...any) { a.l.Error(fmt.Sprint(args...)) }

// Fatal wajib menghentikan proses: itulah kontrak interface Logger milik Asynq.
func (a asynqLogger) Fatal(args ...any) {
	a.l.Error(fmt.Sprint(args...))
	os.Exit(1)
}
