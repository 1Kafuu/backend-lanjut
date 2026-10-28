package repository

import (
	"context"
	"errors"
	"fmt"

	"api-students/app/model"

	"github.com/jackc/pgx/v5"
)

type CourseRepository interface {
	// WithTx mengembalikan repository yang memakai transaction tx.
	WithTx(tx DBTX) CourseRepository

	FindAll(ctx context.Context) ([]model.Course, error)
	FindByID(ctx context.Context, id int) (model.Course, error)
	FindByStudent(ctx context.Context, userID int) ([]model.Course, error)
	IsEnrolled(ctx context.Context, courseID, userID int) (bool, error)

	LockStudent(ctx context.Context, userID int) error
	LockCourses(ctx context.Context, ids ...int) error
	SumCredits(ctx context.Context, userID int) (int, error)
	IncrementEnrolled(ctx context.Context, courseID int) (model.Course, error)
	DecrementEnrolled(ctx context.Context, courseID int) (model.Course, error)
	InsertEnrollment(ctx context.Context, courseID, userID int) error
	DeleteEnrollment(ctx context.Context, courseID, userID int) error
	UpdateQuota(ctx context.Context, id, quota, version int) (model.Course, error)
}

const courseColumns = `id, code, name, credits, quota, enrolled_count, version, created_at`

var (
	ErrCourseFull       = errors.New("kuota kelas sudah penuh")
	ErrNotEnrolled      = errors.New("tidak terdaftar di kelas ini")
	ErrVersionConflict  = errors.New("data sudah diubah pihak lain")
	ErrQuotaBelowFilled = errors.New("kuota lebih kecil dari jumlah peserta")
)

type coursePostgresRepository struct{ db DBTX }

// NewCourseRepository menerima DBTX: pool di luar transaction, tx di dalamnya.
func NewCourseRepository(db DBTX) CourseRepository {
	return &coursePostgresRepository{db: db}
}

// WithTx mengembalikan salinan repository yang memakai transaction tx.
// Tipe kembaliannya interface CourseRepository agar pemanggil tetap memakai
// kontrak yang sama, baik di dalam maupun di luar transaction.
func (r *coursePostgresRepository) WithTx(tx DBTX) CourseRepository {
	return &coursePostgresRepository{db: tx}
}

func scanCourse(row pgx.Row) (model.Course, error) {
	var c model.Course
	err := row.Scan(&c.ID, &c.Code, &c.Name, &c.Credits, &c.Quota,
		&c.EnrolledCount, &c.Version, &c.CreatedAt)
	return c, err
}

func (r *coursePostgresRepository) collect(rows pgx.Rows, err error) ([]model.Course, error) {
	if err != nil {
		return nil, fmt.Errorf("mengambil daftar kelas: %w", err)
	}
	defer rows.Close()

	result := []model.Course{}
	for rows.Next() {
		c, err := scanCourse(rows)
		if err != nil {
			return nil, fmt.Errorf("membaca row kelas: %w", err)
		}
		result = append(result, c)
	}
	return result, rows.Err()
}

func (r *coursePostgresRepository) FindAll(ctx context.Context) ([]model.Course, error) {
	return r.collect(r.db.Query(ctx,
		"SELECT "+courseColumns+" FROM courses ORDER BY code"))
}

func (r *coursePostgresRepository) FindByID(ctx context.Context, id int) (model.Course, error) {
	c, err := scanCourse(r.db.QueryRow(ctx,
		"SELECT "+courseColumns+" FROM courses WHERE id = $1", id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.Course{}, ErrNotFound
		}
		return model.Course{}, fmt.Errorf("mengambil kelas: %w", err)
	}
	return c, nil
}

func (r *coursePostgresRepository) FindByStudent(ctx context.Context, userID int) ([]model.Course, error) {
	return r.collect(r.db.Query(ctx,
		`SELECT c.id, c.code, c.name, c.credits, c.quota,
		        c.enrolled_count, c.version, c.created_at
		 FROM courses c
		 JOIN enrollments e ON e.course_id = c.id
		 WHERE e.user_id = $1
		 ORDER BY c.code`, userID))
}

// IsEnrolled dipakai layanan lain, misalnya ulasan kelas di MongoDB,
// yang perlu tahu apakah seorang user terdaftar di satu kelas.
func (r *coursePostgresRepository) IsEnrolled(
	ctx context.Context, courseID, userID int,
) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM enrollments
			WHERE course_id = $1 AND user_id = $2
		)`,
		courseID, userID,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("memeriksa pendaftaran: %w", err)
	}
	return exists, nil
}

func (r *coursePostgresRepository) InsertEnrollment(ctx context.Context, courseID, userID int) error {
	_, err := r.db.Exec(ctx,
		"INSERT INTO enrollments (course_id, user_id) VALUES ($1, $2)", courseID, userID)
	if err != nil {
		if pgErrorCode(err) == codeUniqueViolation {
			return ErrDuplicate
		}
		return fmt.Errorf("mencatat pendaftaran: %w", err)
	}
	return nil
}

// LockStudent mengunci row milik satu mahasiswa sampai transaction selesai.
// FOR UPDATE membuat transaction lain milik mahasiswa yang sama menunggu di
// sini, sehingga pemeriksaan SKS dan penyimpanan terjadi tanpa sela.
func (r *coursePostgresRepository) LockStudent(ctx context.Context, userID int) error {
	var id int
	err := r.db.QueryRow(ctx,
		"SELECT id FROM users WHERE id = $1 FOR UPDATE", userID).Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("mengunci data mahasiswa: %w", err)
	}
	return nil
}

// SumCredits menghitung total SKS yang sedang diambil mahasiswa.
func (r *coursePostgresRepository) SumCredits(ctx context.Context, userID int) (int, error) {
	var total int
	err := r.db.QueryRow(ctx,
		`SELECT COALESCE(SUM(c.credits), 0)
		 FROM enrollments e
		 JOIN courses c ON c.id = e.course_id
		 WHERE e.user_id = $1`, userID).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("menghitung SKS: %w", err)
	}
	return total, nil
}

// IncrementEnrolled mengambil satu kursi secara atomik. Keputusan "masih ada
// kursi?" dan "tambah satu kursi" terjadi dalam satu pernyataan SQL. Syaratnya
// <, bukan <=, supaya kursi terakhir ditolak rapi sebagai nol baris.
func (r *coursePostgresRepository) IncrementEnrolled(ctx context.Context, courseID int) (model.Course, error) {
	c, err := scanCourse(r.db.QueryRow(ctx,
		`UPDATE courses SET enrolled_count = enrolled_count + 1
		 WHERE id = $1 AND enrolled_count < quota
		 RETURNING `+courseColumns, courseID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.Course{}, ErrCourseFull
		}
		return model.Course{}, fmt.Errorf("mengambil kursi: %w", err)
	}
	return c, nil
}

// DecrementEnrolled melepas satu kursi.
func (r *coursePostgresRepository) DecrementEnrolled(ctx context.Context, courseID int) (model.Course, error) {
	c, err := scanCourse(r.db.QueryRow(ctx,
		`UPDATE courses SET enrolled_count = enrolled_count - 1
		 WHERE id = $1
		 RETURNING `+courseColumns, courseID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.Course{}, ErrNotFound
		}
		return model.Course{}, fmt.Errorf("melepas kursi: %w", err)
	}
	return c, nil
}

// DeleteEnrollment menghapus pendaftaran sambil memeriksa apakah memang ada
// yang terhapus. DELETE tanpa baris bukan error menurut PostgreSQL, jadi
// RowsAffected harus diperiksa agar pemanggil tidak mengurangi jumlah peserta
// untuk pendaftaran yang tidak pernah ada.
func (r *coursePostgresRepository) DeleteEnrollment(ctx context.Context, courseID, userID int) error {
	tag, err := r.db.Exec(ctx,
		"DELETE FROM enrollments WHERE course_id = $1 AND user_id = $2", courseID, userID)
	if err != nil {
		return fmt.Errorf("menghapus pendaftaran: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotEnrolled
	}
	return nil
}

// LockCourses mengunci beberapa kelas dalam urutan id menaik, apa pun urutan
// argumennya. Urutan yang seragam ini yang mencegah deadlock.
func (r *coursePostgresRepository) LockCourses(ctx context.Context, ids ...int) error {
	rows, err := r.db.Query(ctx,
		"SELECT id FROM courses WHERE id = ANY($1) ORDER BY id FOR UPDATE", ids)
	if err != nil {
		return fmt.Errorf("mengunci kelas: %w", err)
	}
	defer rows.Close()

	found := 0
	for rows.Next() {
		found++
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("mengunci kelas: %w", err)
	}
	if found != len(ids) {
		return ErrNotFound
	}
	return nil
}

// UpdateQuota menerapkan optimistic locking: UPDATE hanya berhasil bila
// version di database masih sama dengan version yang dikirim client. Setiap
// perubahan yang berhasil menaikkan version tepat satu. Syarat kuota tidak
// lebih kecil dari jumlah peserta juga ditulis di WHERE.
func (r *coursePostgresRepository) UpdateQuota(ctx context.Context, id, quota, version int) (model.Course, error) {
	c, err := scanCourse(r.db.QueryRow(ctx,
		`UPDATE courses SET quota = $1, version = version + 1
		 WHERE id = $2 AND version = $3 AND $1 >= enrolled_count
		 RETURNING `+courseColumns, quota, id, version))
	if err == nil {
		return c, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return model.Course{}, fmt.Errorf("mengubah kuota: %w", err)
	}

	// Tidak ada baris yang cocok: version bergeser, atau kuota terlalu kecil.
	current, findErr := r.FindByID(ctx, id)
	if findErr != nil {
		return model.Course{}, findErr
	}
	if current.Version != version {
		return model.Course{}, ErrVersionConflict
	}
	return model.Course{}, ErrQuotaBelowFilled
}
