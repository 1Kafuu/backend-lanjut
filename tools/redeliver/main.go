// Alat uji idempotensi Modul 11. Jalankan dari root proyek:
//
//	go run ./tools/redeliver -outbox <id> -copies 5
//
// Setiap salinan memakai TaskID berbeda supaya Redis menerima semuanya;
// TaskID yang sama akan ditolak dengan ErrTaskIDConflict.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"time"

	"api-students/app/model"
	"api-students/config"
	"api-students/database"

	"github.com/hibiken/asynq"
)

func main() {
	config.LoadENV()
	outboxID := flag.Int64("outbox", 0, "id baris outbox yang akan disalin")
	copies := flag.Int("copies", 5, "jumlah salinan task")
	flag.Parse()
	if *outboxID == 0 || *copies < 1 {
		log.Fatal("flag -outbox dan -copies wajib diisi")
	}

	ctx := context.Background()
	pool, err := database.NewPool(ctx)
	if err != nil {
		log.Fatalf("gagal terhubung ke PostgreSQL: %v", err)
	}
	defer pool.Close()

	var eventType string
	var payload []byte
	err = pool.QueryRow(ctx,
		"SELECT event_type, payload FROM outbox WHERE id = $1", *outboxID,
	).Scan(&eventType, &payload)
	if err != nil {
		log.Fatalf("membaca outbox %d: %v", *outboxID, err)
	}

	body, err := json.Marshal(model.NotificationJob{
		OutboxID: *outboxID, EventType: eventType, Payload: payload,
	})
	if err != nil {
		log.Fatalf("mengemas job: %v", err)
	}

	client := asynq.NewClient(config.RedisOpt())
	defer client.Close()

	for i := 0; i < *copies; i++ {
		task := asynq.NewTask(model.TaskSendNotification, body,
			asynq.TaskID(fmt.Sprintf("redeliver-%d-%d", *outboxID, i)),
			asynq.Queue(model.QueueNotifications),
			asynq.MaxRetry(5),
			asynq.Timeout(30*time.Second),
		)
		if _, err := client.EnqueueContext(ctx, task); err != nil {
			log.Fatalf("enqueue salinan %d: %v", i, err)
		}
	}
	fmt.Printf("enqueue %d salinan task untuk outbox %d\n", *copies, *outboxID)
}
