package service_test

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"api-students/app/model"
	"api-students/app/service"
	"api-students/config"
	"api-students/helper"
	"api-students/routes"

	"github.com/gofiber/fiber/v2"
)

const (
	courseBackend = 1
	courseOther   = 2
	studentA      = 7
	studentB      = 8
)

type testEnv struct {
	app *fiber.App
	jwt *helper.JWTManager
}

func newTestEnv(t *testing.T) testEnv {
	t.Helper()

	reviews := newFakeReviewRepo()
	courses := &fakeCourseRepo{
		courses: map[int]model.Course{
			courseBackend: {ID: courseBackend, Code: "IF101", Name: "Pemrograman Backend Lanjut"},
			courseOther:   {ID: courseOther, Code: "IF102", Name: "Basis Data Lanjut"},
		},
		enrolled: map[[2]int]bool{{courseBackend, studentA}: true, {courseBackend, studentB}: true},
	}

	jwtManager := helper.NewJWTManager("secret-khusus-test-minimal-32-karakter", "test", time.Minute)
	perms := helper.NewPermissionSet(map[string][]string{"admin": {"review:moderate"}})

	app := config.NewApp(slog.New(slog.NewTextHandler(io.Discard, nil)), nil, routes.Dependencies{
		JWT:           jwtManager,
		Permissions:   perms,
		ReviewService: service.NewReviewService(reviews, courses, perms),
	})

	return testEnv{app: app, jwt: jwtManager}
}

func (e testEnv) do(t *testing.T, userID int, method, path, body string) (int, map[string]any) {
	t.Helper()

	token, err := e.jwt.GenerateAccess(model.User{
		ID: userID, Username: "mhs" + strconv.Itoa(userID), Role: "user",
	})
	if err != nil {
		t.Fatalf("membuat token: %v", err)
	}

	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.app.Test(req, -1)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()

	var parsed map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		t.Fatalf("%s %s: response bukan JSON: %v", method, path, err)
	}
	return resp.StatusCode, parsed
}

const validReview = `{"rating": 5, "comment": "Materinya runtut dan contohnya nyata.", "tags": ["Seru", "seru"]}`

func TestListReviews_NewCourseIsEmpty(t *testing.T) {
	env := newTestEnv(t)

	status, body := env.do(t, studentA, http.MethodGet, "/api/v1/courses/1/reviews", "")
	if status != http.StatusOK {
		t.Fatalf("status = %d, seharusnya 200", status)
	}
	if data := body["data"].([]any); len(data) != 0 {
		t.Errorf("jumlah ulasan = %d, seharusnya 0", len(data))
	}
}

func TestCreateReview_Success(t *testing.T) {
	env := newTestEnv(t)

	status, body := env.do(t, studentA, http.MethodPost, "/api/v1/courses/1/reviews", validReview)
	if status != http.StatusCreated {
		t.Fatalf("status = %d, seharusnya 201 (body: %v)", status, body)
	}
	data := body["data"].(map[string]any)
	if tags := data["tags"].([]any); len(tags) != 1 || tags[0] != "seru" {
		t.Errorf("tags = %v, seharusnya [seru] setelah dinormalisasi", tags)
	}

	_, list := env.do(t, studentA, http.MethodGet, "/api/v1/courses/1/reviews", "")
	if n := len(list["data"].([]any)); n != 1 {
		t.Errorf("jumlah ulasan setelah dibuat = %d, seharusnya 1", n)
	}
}

func TestCreateReview_Rejected(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		body       string
		wantStatus int
		wantCode   string
	}{
		{"bukan peserta kelas", "/api/v1/courses/2/reviews", validReview, http.StatusForbidden, "NOT_ENROLLED"},
		{"kelas tidak ada", "/api/v1/courses/99/reviews", validReview, http.StatusNotFound, "NOT_FOUND"},
		{"rating di luar 1..5", "/api/v1/courses/1/reviews", `{"rating": 9, "comment": "Komentar yang cukup panjang."}`, http.StatusUnprocessableEntity, "VALIDATION_ERROR"},
		{"body bukan JSON", "/api/v1/courses/1/reviews", `{rating:`, http.StatusBadRequest, "BAD_REQUEST"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := newTestEnv(t)
			status, body := env.do(t, studentA, http.MethodPost, tt.path, tt.body)
			if status != tt.wantStatus || body["code"] != tt.wantCode {
				t.Errorf("dapat %d %v, seharusnya %d %s", status, body["code"], tt.wantStatus, tt.wantCode)
			}
		})
	}
}

func TestCreateReview_Twice(t *testing.T) {
	env := newTestEnv(t)

	if status, _ := env.do(t, studentB, http.MethodPost, "/api/v1/courses/1/reviews", validReview); status != http.StatusCreated {
		t.Fatalf("ulasan pertama: status = %d, seharusnya 201", status)
	}
	status, body := env.do(t, studentB, http.MethodPost, "/api/v1/courses/1/reviews", validReview)
	if status != http.StatusConflict || body["code"] != "ALREADY_REVIEWED" {
		t.Errorf("ulasan kedua: dapat %d %v, seharusnya 409 ALREADY_REVIEWED", status, body["code"])
	}
}
