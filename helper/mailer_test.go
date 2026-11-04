package helper

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"api-students/app/model"
)

func TestLogMailerMenulisLog(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mail.log")
	mailer, err := NewLogMailer(path, 0, 0)
	if err != nil {
		t.Fatalf("NewLogMailer: %v", err)
	}
	if err := mailer.Send(context.Background(), model.Message{
		To: "mhsuji@student.unair.ac.id", Subject: "KRS: berhasil terdaftar",
	}); err != nil {
		t.Fatalf("Send: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.Contains(string(data), "mhsuji@student.unair.ac.id") {
		t.Fatalf("baris log tidak memuat alamat: %q", string(data))
	}
}

func TestLogMailerGagalPermanenUntukDomainInvalid(t *testing.T) {
	mailer, err := NewLogMailer(filepath.Join(t.TempDir(), "mail.log"), 0, 0)
	if err != nil {
		t.Fatalf("NewLogMailer: %v", err)
	}
	err = mailer.Send(context.Background(), model.Message{To: "salah@kampus.invalid"})
	if !errors.Is(err, ErrPermanentDelivery) {
		t.Fatalf("harus ErrPermanentDelivery, dapat %v", err)
	}
}

func TestLogMailerGagalSetiapNAttempt(t *testing.T) {
	mailer, err := NewLogMailer(filepath.Join(t.TempDir(), "mail.log"), 0, 3)
	if err != nil {
		t.Fatalf("NewLogMailer: %v", err)
	}
	for attempt := 1; attempt <= 4; attempt++ {
		err = mailer.Send(context.Background(), model.Message{To: "mhsuji@student.unair.ac.id"})
		wantTemporary := attempt%3 == 0
		if wantTemporary && !errors.Is(err, ErrTemporaryDelivery) {
			t.Fatalf("attempt %d harus ErrTemporaryDelivery, dapat %v", attempt, err)
		}
		if !wantTemporary && err != nil {
			t.Fatalf("attempt %d harus sukses, dapat %v", attempt, err)
		}
	}
}

func TestLogMailerBatalSaatContextBatal(t *testing.T) {
	mailer, err := NewLogMailer(filepath.Join(t.TempDir(), "mail.log"), time.Second, 0)
	if err != nil {
		t.Fatalf("NewLogMailer: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := mailer.Send(ctx, model.Message{To: "mhsuji@student.unair.ac.id"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("harus context.Canceled, dapat %v", err)
	}
}
