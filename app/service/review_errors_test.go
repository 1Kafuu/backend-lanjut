package service

import (
	"errors"
	"fmt"
	"testing"

	"api-students/app/repository"
	"api-students/helper"
)

func TestTranslateReviewError(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{"id tidak valid", repository.ErrInvalidID, 400, helper.CodeBadRequest},
		{"ulasan tidak ada", repository.ErrNotFound, 404, helper.CodeNotFound},
		{"sudah ada ulasan", repository.ErrDuplicate, 409, "ALREADY_REVIEWED"},
		{"bukan pemilik", repository.ErrNotOwner, 403, helper.CodeForbidden},
		{"nilai ulasan sendiri", repository.ErrOwnReview, 409, "OWN_REVIEW"},
		{"sudah menilai", repository.ErrAlreadyVoted, 409, "ALREADY_VOTED"},
		{"batas balasan", repository.ErrReplyLimit, 409, "REPLY_LIMIT"},
		{"error dibungkus", fmt.Errorf("lapisan luar: %w", repository.ErrNotFound), 404, helper.CodeNotFound},
		{"error lain", errors.New("koneksi MongoDB putus"), 500, helper.CodeInternal},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var appErr *helper.AppError
			if !errors.As(translateReviewError(tt.err), &appErr) {
				t.Fatalf("hasil bukan *helper.AppError")
			}
			if appErr.Status != tt.wantStatus || appErr.Code != tt.wantCode {
				t.Errorf("dapat %d %s, seharusnya %d %s",
					appErr.Status, appErr.Code, tt.wantStatus, tt.wantCode)
			}
		})
	}
}
