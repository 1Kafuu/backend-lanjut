//go:build integration

package integration_test

import (
	"context"
	"errors"
	"testing"

	"api-students/app/repository"
)

// TestIncrementEnrolled_StopsAtQuota memastikan penambahan kursi berhenti
// pada kuota, bukan hanya menaikkan angka.
func TestIncrementEnrolled_StopsAtQuota(t *testing.T) {
	course := seedCourse(t, 3, 2)
	repo := repository.NewCourseRepository(pool)
	ctx := context.Background()

	first, err := repo.IncrementEnrolled(ctx, course.ID)
	if err != nil {
		t.Fatalf("pengisian kursi pertama: %v", err)
	}
	if first.EnrolledCount != 1 {
		t.Errorf("enrolled_count = %d, seharusnya 1", first.EnrolledCount)
	}

	second, err := repo.IncrementEnrolled(ctx, course.ID)
	if err != nil {
		t.Fatalf("pengisian kursi kedua: %v", err)
	}
	if second.EnrolledCount != 2 {
		t.Errorf("enrolled_count = %d, seharusnya 2", second.EnrolledCount)
	}

	// Kursi ketiga harus ditolak, dan angka TIDAK boleh naik.
	_, err = repo.IncrementEnrolled(ctx, course.ID)
	if err == nil {
		t.Fatal("pengisian kursi ketiga seharusnya gagal: kuota habis")
	}
	if !errors.Is(err, repository.ErrCourseFull) {
		t.Errorf("error = %v, seharusnya repository.ErrCourseFull", err)
	}

	var count int
	if err := pool.QueryRow(ctx,
		`SELECT enrolled_count FROM courses WHERE id = $1`, course.ID).Scan(&count); err != nil {
		t.Fatalf("cek enrolled_count: %v", err)
	}
	if count != 2 {
		t.Errorf("enrolled_count di DB = %d, seharusnya tetap 2", count)
	}
}

// TestInsertEnrollment_Duplicate memastikan pendaftaran ganda ditolak
// dengan error sentinel, bukan error database mentah.
func TestInsertEnrollment_Duplicate(t *testing.T) {
	user := seedStudent(t)
	course := seedCourse(t, 3, 40)
	repo := repository.NewCourseRepository(pool)
	ctx := context.Background()

	if err := repo.InsertEnrollment(ctx, course.ID, user.ID); err != nil {
		t.Fatalf("pendaftaran pertama: %v", err)
	}

	err := repo.InsertEnrollment(ctx, course.ID, user.ID)
	if err == nil {
		t.Fatal("pendaftaran kedua seharusnya gagal")
	}
	if !errors.Is(err, repository.ErrDuplicate) {
		t.Errorf("error = %v, seharusnya repository.ErrDuplicate", err)
	}

	var rows int
	if err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM enrollments WHERE course_id = $1 AND user_id = $2`,
		course.ID, user.ID).Scan(&rows); err != nil {
		t.Fatalf("hitung baris: %v", err)
	}
	if rows != 1 {
		t.Errorf("baris enrollments = %d, seharusnya 1", rows)
	}
}

// TestWithTx_RollsBackEveryStep memastikan transaction dibatalkan utuh
// saat salah satu langkah gagal. Tidak boleh ada sisa yang tertinggal.
func TestWithTx_RollsBackEveryStep(t *testing.T) {
	user := seedStudent(t)
	course := seedCourse(t, 3, 40)
	tx := repository.NewTxManager(pool)
	ctx := context.Background()

	boom := errors.New("langkah ketiga gagal")

	err := tx.WithTx(ctx, func(dbtx repository.DBTX) error {
		repo := repository.NewCourseRepository(dbtx)
		if err := repo.InsertEnrollment(ctx, course.ID, user.ID); err != nil {
			return err
		}
		if _, err := repo.IncrementEnrolled(ctx, course.ID); err != nil {
			return err
		}
		return boom
	})
	if err == nil {
		t.Fatal("transaction seharusnya gagal")
	}

	// Rollback: enrollment tidak boleh ada.
	var rows int
	if err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM enrollments WHERE course_id = $1 AND user_id = $2`,
		course.ID, user.ID).Scan(&rows); err != nil {
		t.Fatalf("hitung enrollment: %v", err)
	}
	if rows != 0 {
		t.Errorf("baris enrollments = %d, seharusnya 0 (rollback)", rows)
	}

	// Rollback: enrolled_count juga tidak boleh naik.
	var count int
	if err := pool.QueryRow(ctx,
		`SELECT enrolled_count FROM courses WHERE id = $1`, course.ID).Scan(&count); err != nil {
		t.Fatalf("cek enrolled_count: %v", err)
	}
	if count != 0 {
		t.Errorf("enrolled_count = %d, seharusnya 0 (rollback)", count)
	}
}
