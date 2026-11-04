package service

import (
	"errors"
	"testing"

	"api-students/app/model"
)

func TestBuildMessageUntukSemuaEvent(t *testing.T) {
	user := model.User{Username: "mhsuji", Email: "mhsuji@student.unair.ac.id"}
	event := model.EnrollmentEvent{
		CourseCode: "IF102", CourseName: "Basis Data Lanjut",
		FromCourseCode: "IF101", FromCourseName: "Algoritma",
	}

	cases := []struct {
		eventType string
		subject   string
	}{
		{model.EventEnrollmentCreated, "KRS: berhasil terdaftar di IF102 Basis Data Lanjut"},
		{model.EventEnrollmentDropped, "KRS: keluar dari IF102 Basis Data Lanjut"},
		{model.EventEnrollmentSwapped, "KRS: pindah dari IF101 Algoritma ke IF102 Basis Data Lanjut"},
	}
	for _, tc := range cases {
		msg, err := BuildMessage(tc.eventType, event, user)
		if err != nil {
			t.Fatalf("%s: %v", tc.eventType, err)
		}
		if msg.Subject != tc.subject {
			t.Fatalf("%s: subjek %q, harus %q", tc.eventType, msg.Subject, tc.subject)
		}
		if msg.To != user.Email {
			t.Fatalf("%s: penerima %q, harus %q", tc.eventType, msg.To, user.Email)
		}
	}
}

func TestBuildMessageEventTidakDikenal(t *testing.T) {
	_, err := BuildMessage("course.unknown", model.EnrollmentEvent{}, model.User{})
	if !errors.Is(err, errUnknownEvent) {
		t.Fatalf("harus errUnknownEvent, dapat %v", err)
	}
}
