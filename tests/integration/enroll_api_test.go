//go:build integration

package integration_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"api-students/app/model"
	"api-students/app/repository"
	"api-students/app/service"
	"api-students/config"
	"api-students/helper"
	"api-students/routes"

	"github.com/gofiber/fiber/v2"
)

// envAPI menyusun aplikasi dengan repository production (PostgreSQL asli)
// supaya test benar-benar menguji database, bukan fake.
func envAPI(t *testing.T) (*fiber.App, *helper.JWTManager) {
	t.Helper()

	courses := repository.NewCourseRepository(pool)
	outbox := repository.NewOutboxRepository(pool)
	tx := repository.NewTxManager(pool)
	jwtManager := helper.NewJWTManager("secret-khusus-test-minimal-32-karakter", "test", time.Minute)
	perms := helper.NewPermissionSet(map[string][]string{"admin": {"course:manage"}})

	app := config.NewApp(slog.New(slog.NewTextHandler(io.Discard, nil)), pool, routes.Dependencies{
		JWT:           jwtManager,
		Permissions:   perms,
		CourseService: service.NewCourseService(courses, outbox, tx),
	})
	return app, jwtManager
}

func tokenFor(t *testing.T, j *helper.JWTManager, u model.User) string {
	t.Helper()
	tok, err := j.GenerateAccess(u)
	if err != nil {
		t.Fatalf("token: %v", err)
	}
	return tok
}

func call(t *testing.T, app *fiber.App, token, method, path, body string) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent {
		return resp.StatusCode, nil
	}
	var parsed map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		t.Fatalf("%s %s: response bukan JSON: %v", method, path, err)
	}
	return resp.StatusCode, parsed
}

// seedEnrollment mendaftarkan mahasiswa ke kelas secara langsung, lalu
// menyelaraskan enrolled_count supaya sama dengan keadaan produksi.
func seedEnrollment(t *testing.T, courseID, userID int) {
	t.Helper()
	ctx := context.Background()
	if _, err := pool.Exec(ctx,
		`INSERT INTO enrollments (course_id, user_id) VALUES ($1, $2)`, courseID, userID); err != nil {
		t.Fatalf("seed enrollment: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`UPDATE courses SET enrolled_count = enrolled_count + 1 WHERE id = $1`, courseID); err != nil {
		t.Fatalf("update enrolled_count: %v", err)
	}
}

func enrolledCount(t *testing.T, courseID int) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(),
		`SELECT enrolled_count FROM courses WHERE id = $1`, courseID).Scan(&n); err != nil {
		t.Fatalf("cek enrolled_count: %v", err)
	}
	return n
}

func outboxCount(t *testing.T, userID int, eventType string) int {
	t.Helper()
	var n int
	err := pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM outbox
		 WHERE (payload->>'user_id')::INT = $1 AND event_type = $2`, userID, eventType).Scan(&n)
	if err != nil {
		t.Fatalf("hitung outbox: %v", err)
	}
	return n
}

// TestEnrollEndpoint_WritesOutboxOnlyOnSuccess memastikan event outbox hanya
// tercatat bila seluruh langkah enrollment berhasil.
//
// Dua jalur diuji: enrollment sukses menulis tepat satu event, dan
// enrollment ke kelas yang sudah penuh tidak menulis apa pun.
func TestEnrollEndpoint_WritesOutboxOnlyOnSuccess(t *testing.T) {
	app, jwtManager := envAPI(t)

	t.Run("sukses menulis satu event", func(t *testing.T) {
		user := seedStudent(t)
		course := seedCourse(t, 3, 40)
		token := tokenFor(t, jwtManager, model.User{ID: user.ID, Username: user.Username, Role: "user"})

		status, body := call(t, app, token, http.MethodPost,
			"/api/v1/courses/"+strconv.Itoa(course.ID)+"/enroll", "")
		if status != http.StatusCreated {
			t.Fatalf("status = %d, seharusnya 201 (body %v)", status, body)
		}
		if n := outboxCount(t, user.ID, model.EventEnrollmentCreated); n != 1 {
			t.Errorf("outbox enrollment.created = %d, seharusnya tepat 1", n)
		}
	})

	t.Run("kelas penuh tidak menulis event", func(t *testing.T) {
		user := seedStudent(t)
		course := seedCourse(t, 3, 1)
		seedEnrollment(t, course.ID, user.ID) // kursinya sudah terisi
		token := tokenFor(t, jwtManager, model.User{ID: user.ID, Username: user.Username, Role: "user"})

		status, body := call(t, app, token, http.MethodPost,
			"/api/v1/courses/"+strconv.Itoa(course.ID)+"/enroll", "")
		if status != http.StatusConflict {
			t.Fatalf("status = %d, seharusnya 409 (body %v)", status, body)
		}
		if body["code"] != "COURSE_FULL" {
			t.Errorf("code = %v, seharusnya COURSE_FULL", body["code"])
		}
		if n := outboxCount(t, user.ID, model.EventEnrollmentCreated); n != 0 {
			t.Errorf("outbox enrollment.created = %d, seharusnya 0", n)
		}
	})

	// Swap ke kelas penuh harus GAGAL TOTAL: mahasiswa tetap di kelas asal,
	// jumlah peserta kedua kelas tidak berubah, dan tidak ada event swap.
	t.Run("swap ke kelas penuh ditolak total", func(t *testing.T) {
		user := seedStudent(t)
		from := seedCourse(t, 3, 40)
		to := seedCourse(t, 3, 1) // kuota 1, sudah penuh
		seedEnrollment(t, from.ID, user.ID)
		other := seedStudent(t)
		seedEnrollment(t, to.ID, other.ID)

		beforeFrom := enrolledCount(t, from.ID)
		beforeTo := enrolledCount(t, to.ID)
		token := tokenFor(t, jwtManager, model.User{ID: user.ID, Username: user.Username, Role: "user"})

		status, body := call(t, app, token, http.MethodPost, "/api/v1/enrollments/swap",
			`{"from_course_id":`+strconv.Itoa(from.ID)+`,"to_course_id":`+strconv.Itoa(to.ID)+`}`)
		if status != http.StatusConflict {
			t.Fatalf("status = %d, seharusnya 409 COURSE_FULL (body %v)", status, body)
		}
		if body["code"] != "COURSE_FULL" {
			t.Errorf("code = %v, seharusnya COURSE_FULL", body["code"])
		}

		var stillEnrolled int
		if err := pool.QueryRow(context.Background(),
			`SELECT COUNT(*) FROM enrollments WHERE course_id = $1 AND user_id = $2`,
			from.ID, user.ID).Scan(&stillEnrolled); err != nil {
			t.Fatalf("cek enrollment: %v", err)
		}
		if stillEnrolled != 1 {
			t.Errorf("enrollment asal hilang setelah swap gagal: count = %d", stillEnrolled)
		}
		if got := enrolledCount(t, from.ID); got != beforeFrom {
			t.Errorf("peserta kelas asal = %d, sebelumnya %d", got, beforeFrom)
		}
		if got := enrolledCount(t, to.ID); got != beforeTo {
			t.Errorf("peserta kelas tujuan = %d, sebelumnya %d", got, beforeTo)
		}
		if n := outboxCount(t, user.ID, model.EventEnrollmentSwapped); n != 0 {
			t.Errorf("outbox enrollment.swapped = %d, seharusnya 0", n)
		}
	})

	// Drop berhasil: 204, enrollment hilang, enrolled_count berkurang satu,
	// dan tepat satu event enrollment.dropped tercatat.
	t.Run("drop melepas kursi dan menulis satu event", func(t *testing.T) {
		user := seedStudent(t)
		course := seedCourse(t, 3, 40)
		seedEnrollment(t, course.ID, user.ID)
		before := enrolledCount(t, course.ID)
		token := tokenFor(t, jwtManager, model.User{ID: user.ID, Username: user.Username, Role: "user"})

		status, _ := call(t, app, token, http.MethodDelete,
			"/api/v1/courses/"+strconv.Itoa(course.ID)+"/enroll", "")
		if status != http.StatusNoContent {
			t.Fatalf("status = %d, seharusnya 204", status)
		}

		var stillEnrolled int
		if err := pool.QueryRow(context.Background(),
			`SELECT COUNT(*) FROM enrollments WHERE course_id = $1 AND user_id = $2`,
			course.ID, user.ID).Scan(&stillEnrolled); err != nil {
			t.Fatalf("cek enrollment: %v", err)
		}
		if stillEnrolled != 0 {
			t.Errorf("enrollment masih ada: count = %d, seharusnya 0", stillEnrolled)
		}
		if got := enrolledCount(t, course.ID); got != before-1 {
			t.Errorf("enrolled_count = %d, sebelumnya %d, seharusnya berkurang satu", got, before)
		}
		if n := outboxCount(t, user.ID, model.EventEnrollmentDropped); n != 1 {
			t.Errorf("outbox enrollment.dropped = %d, seharusnya tepat 1", n)
		}
	})
}

// TestEnrollEndpoint_ConcurrentCreditLimit memastikan batas 24 SKS tetap
// berlaku ketika 10 request enroll datang bersamaan.
//
// Test ini hanya bermakna bila request benar-benar berjalan bersamaan.
// Karena itu pool HARUS punya MaxConns > 1: dengan MaxConns = 1 seluruh
// request berantre pada satu-satunya koneksi, sehingga pengujian konkurensi
// tidak menguji apa pun dan bisa menyembunyikan race condition.
func TestEnrollEndpoint_ConcurrentCreditLimit(t *testing.T) {
	user := seedStudent(t)

	// 10 kelas berbeda, masing-masing 3 SKS. Batas 24 SKS -> tepat 8 boleh masuk.
	courseIDs := make([]int, 0, 10)
	for i := 0; i < 10; i++ {
		courseIDs = append(courseIDs, seedCourse(t, 3, 40).ID)
	}

	app, jwtManager := envAPI(t)
	token := tokenFor(t, jwtManager, model.User{ID: user.ID, Username: user.Username, Role: "user"})

	// Barrier: semua goroutine mulai bersamaan supaya request benar-benar
	// berebut lock, bukan berjalan berurutan.
	var ready sync.WaitGroup
	var goSignal sync.WaitGroup
	ready.Add(len(courseIDs))
	goSignal.Add(1)

	var wg sync.WaitGroup
	results := make([]int, len(courseIDs))
	bodies := make([]map[string]any, len(courseIDs))

	for i, id := range courseIDs {
		wg.Add(1)
		go func(i, id int) {
			defer wg.Done()
			ready.Done()
			goSignal.Wait()

			req := httptest.NewRequest(http.MethodPost,
				"/api/v1/courses/"+strconv.Itoa(id)+"/enroll", nil)
			req.Header.Set("Authorization", "Bearer "+token)
			req.Header.Set("Content-Type", "application/json")

			resp, err := app.Test(req, -1)
			if err != nil {
				t.Errorf("request %d: %v", i, err)
				return
			}
			defer resp.Body.Close()

			var parsed map[string]any
			_ = json.NewDecoder(resp.Body).Decode(&parsed)
			results[i] = resp.StatusCode
			bodies[i] = parsed
		}(i, id)
	}

	ready.Wait()
	goSignal.Done()
	wg.Wait()

	created, conflict, other := 0, 0, 0
	for i, st := range results {
		switch st {
		case http.StatusCreated:
			created++
		case http.StatusConflict:
			conflict++
		default:
			other++
			t.Logf("request %d status %d body %v", i, st, bodies[i])
		}
	}
	t.Logf("maxConns=%d created=%d conflict=%d other=%d",
		pool.Config().MaxConns, created, conflict, other)

	if other > 0 {
		t.Errorf("%d request berakhir dengan status lain (bukan 201/409)", other)
	}
	if created != 8 {
		t.Errorf("jumlah enroll sukses = %d, seharusnya tepat 8 (batas 24 SKS / 3 SKS)", created)
	}

	// Bukti kedua: total SKS yang benar-benar tercatat tidak boleh > 24.
	var total int
	if err := pool.QueryRow(context.Background(),
		`SELECT COALESCE(SUM(c.credits),0) FROM enrollments e
		 JOIN courses c ON c.id = e.course_id WHERE e.user_id = $1`, user.ID).Scan(&total); err != nil {
		t.Fatalf("menghitung total SKS: %v", err)
	}
	if total > 24 {
		t.Errorf("total SKS tercatat = %d, melebihi batas 24", total)
	}
}

// TestClaim_OnlyOnce memeriksa bahwa satu event hanya bisa diklaim satu
// kali, walau beberapa worker mengklaim bersamaan.
//
// Inilah yang membuat worker idempoten: task yang terkirim dua kali
// (at-least-once) tidak boleh menghasilkan dua email.
func TestClaim_OnlyOnce(t *testing.T) {
	user := seedStudent(t)
	repo := repository.NewNotificationRepository(pool)

	n := model.Notification{
		OutboxID: 900001, UserID: user.ID, Channel: "email",
		Subject: "Uji klaim", Body: "sekali saja",
	}

	const workers = 8
	var wg sync.WaitGroup
	var mu sync.Mutex
	claimed := 0

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ok, err := repo.Claim(context.Background(), n)
			if err != nil {
				t.Errorf("claim: %v", err)
				return
			}
			if ok {
				mu.Lock()
				claimed++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	if claimed != 1 {
		t.Errorf("jumlah klaim berhasil = %d, seharusnya tepat 1", claimed)
	}

	var rows int
	if err := pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM notifications WHERE outbox_id = $1 AND user_id = $2`,
		n.OutboxID, n.UserID).Scan(&rows); err != nil {
		t.Fatalf("hitung baris: %v", err)
	}
	if rows != 1 {
		t.Errorf("baris notifications = %d, seharusnya tepat 1", rows)
	}
}
