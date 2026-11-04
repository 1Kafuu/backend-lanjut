package service

import (
	"context"
	"encoding/json"
	"testing"

	"api-students/app/model"

	"github.com/hibiken/asynq"
)

// enqueuerPalsu menangkap task yang dikirim tanpa menyentuh Redis.
type enqueuerPalsu struct {
	task *asynq.Task
}

func (e *enqueuerPalsu) EnqueueContext(ctx context.Context, task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error) {
	e.task = task
	return &asynq.TaskInfo{}, nil
}

func TestPublishMengirimPayloadJSON(t *testing.T) {
	queue := &enqueuerPalsu{}
	relay := NewOutboxRelay(nil, nil, queue, nil, 0, 0)

	event := model.OutboxEvent{
		ID:        42,
		EventType: model.EventEnrollmentCreated,
		Payload:   json.RawMessage(`{"user_id":7,"course_code":"IF102"}`),
	}
	if err := relay.publish(context.Background(), event); err != nil {
		t.Fatalf("publish: %v", err)
	}
	if queue.task.Type() != model.TaskSendNotification {
		t.Fatalf("tipe task %q, harus %q", queue.task.Type(), model.TaskSendNotification)
	}

	var job model.NotificationJob
	if err := json.Unmarshal(queue.task.Payload(), &job); err != nil {
		t.Fatalf("payload bukan JSON job yang valid: %v", err)
	}
	if job.OutboxID != 42 || job.EventType != model.EventEnrollmentCreated {
		t.Fatalf("job salah: %+v", job)
	}
}
