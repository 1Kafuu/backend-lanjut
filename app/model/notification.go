package model

import (
	"encoding/json"
	"time"
)

// Nama event KRS yang dicatat ke outbox.
const (
	EventEnrollmentCreated  = "enrollment.created"
	EventEnrollmentDropped  = "enrollment.dropped"
	EventEnrollmentSwapped  = "enrollment.swapped"
	EventCourseQuotaChanged = "course.quota_changed"
)

// TaskSendNotification = nama jenis task, QueueNotifications = nama queue.
const (
	TaskSendNotification = "notification:send"
	QueueNotifications   = "notifications"
)

// EnrollmentEvent adalah isi event KRS, nama kelas disalin agar notifikasi memakai nama yang dilihat mahasiswa.
type EnrollmentEvent struct {
	UserID         int    `json:"user_id"`
	CourseID       int    `json:"course_id"`
	CourseCode     string `json:"course_code"`
	CourseName     string `json:"course_name"`
	FromCourseCode string `json:"from_course_code,omitempty"`
	FromCourseName string `json:"from_course_name,omitempty"`

	// NewQuota dipakai event course.quota_changed.
	NewQuota int `json:"new_quota,omitempty"`

	// UserIDs dipakai event course.quota_changed: seluruh peserta kelas
	// saat kuota berubah. Relay memecahnya jadi satu task per peserta.
	UserIDs []int `json:"user_ids,omitempty"`
}

// OutboxEvent adalah satu baris tabel outbox.
type OutboxEvent struct {
	ID        int64
	EventType string
	Payload   []byte
	CreatedAt time.Time
}

// NotificationJob adalah isi task yang dikirim, OutboxID adalah idempotency key worker.
type NotificationJob struct {
	OutboxID  int64           `json:"outbox_id"`
	EventType string          `json:"event_type"`
	UserID    int             `json:"user_id,omitempty"`
	Payload   json.RawMessage `json:"payload"`
}

// Message adalah pesan siap kirim.
type Message struct {
	To      string
	Subject string
	Body    string
}

// Notification adalah notifikasi yang tercatat untuk mahasiswa.
type Notification struct {
	ID       int64     `json:"id"`
	OutboxID int64     `json:"outbox_id"`
	UserID   int       `json:"user_id"`
	Channel  string    `json:"channel"`
	Subject  string    `json:"subject"`
	Body     string    `json:"body"`
	SentAt   time.Time `json:"sent_at"`
}

// QueueStats merangkum keadaan outbox dan queue untuk endpoint admin.
type QueueStats struct {
	OutboxPending     int        `json:"outbox_pending"`
	OutboxPublished   int        `json:"outbox_published"`
	OldestPendingAge  string     `json:"oldest_pending_age"`
	NotificationsSent int        `json:"notifications_sent"`
	Queue             *QueueInfo `json:"queue,omitempty"`
	QueueError        string     `json:"queue_error,omitempty"`
}

// QueueInfo menyalin sebagian informasi queue Asynq agar package model tidak mengimpor library luar.
type QueueInfo struct {
	Name      string `json:"name"`
	Pending   int    `json:"pending"`
	Active    int    `json:"active"`
	Scheduled int    `json:"scheduled"`
	Retry     int    `json:"retry"`
	Archived  int    `json:"archived"`
	Completed int    `json:"completed"`
	Processed int    `json:"processed_today"`
	Failed    int    `json:"failed_today"`
}
