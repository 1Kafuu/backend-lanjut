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
	task  *asynq.Task
	tasks []*asynq.Task
}

func (e *enqueuerPalsu) EnqueueContext(ctx context.Context, task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error) {
	e.task = task
	e.tasks = append(e.tasks, task)
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
	if id := taskID(42, 0); id != "outbox-42" {
		t.Fatalf("TaskID %q, harus %q", id, "outbox-42")
	}
}

func TestSplitEventKuotaJadiSatuJobPerPeserta(t *testing.T) {
	event := model.OutboxEvent{
		ID:        90,
		EventType: model.EventCourseQuotaChanged,
		Payload:   json.RawMessage(`{"course_code":"IF102","new_quota":45,"user_ids":[7,8,9]}`),
	}

	jobs, err := SplitEvent(event)
	if err != nil {
		t.Fatalf("SplitEvent: %v", err)
	}
	if len(jobs) != 3 {
		t.Fatalf("job %d, harus 3", len(jobs))
	}
	for i, want := range []int{7, 8, 9} {
		if jobs[i].UserID != want {
			t.Fatalf("job %d: user %d, harus %d", i, jobs[i].UserID, want)
		}
		var ev model.EnrollmentEvent
		if err := json.Unmarshal(jobs[i].Payload, &ev); err != nil {
			t.Fatalf("job %d: payload rusak: %v", i, err)
		}
		if ev.UserID != want {
			t.Fatalf("job %d: payload user %d, harus %d", i, ev.UserID, want)
		}
		if len(ev.UserIDs) != 0 {
			t.Fatalf("job %d: payload masih membawa daftar peserta", i)
		}
	}
}

func TestPublishEventKuotaSatuTaskPerPeserta(t *testing.T) {
	queue := &enqueuerPalsu{}
	relay := NewOutboxRelay(nil, nil, queue, nil, 0, 0)

	event := model.OutboxEvent{
		ID:        91,
		EventType: model.EventCourseQuotaChanged,
		Payload:   json.RawMessage(`{"course_code":"IF102","new_quota":45,"user_ids":[7,8]}`),
	}
	if err := relay.publish(context.Background(), event); err != nil {
		t.Fatalf("publish: %v", err)
	}
	if len(queue.tasks) != 2 {
		t.Fatalf("task %d, harus 2", len(queue.tasks))
	}
	if id := taskID(91, 7); id != "outbox-91-7" {
		t.Fatalf("TaskID %q, harus %q", id, "outbox-91-7")
	}
	if id := taskID(91, 8); id != "outbox-91-8" {
		t.Fatalf("TaskID %q, harus %q", id, "outbox-91-8")
	}
}
