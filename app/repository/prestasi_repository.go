package repository

import (
	"api-students/app/model"
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PrestasiRepository interface {
	FindAll(ctx context.Context, q model.PrestasiListQuery) ([]model.Prestasi, int, error)
	FindByID(ctx context.Context, id int) (model.Prestasi, error)
}

type prestasiPostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPrestasiReporsitory(pool *pgxpool.Pool) PrestasiRepository {
	return &prestasiPostgresRepository{pool: pool}
}

func prestasibuildFilter(q model.PrestasiListQuery) (string, []any) {
	where := " WHERE 1=1"
	args := []any{}
	if q.Search != "" {
		where += fmt.Sprintf(" AND nama_prestasi ILIKE $%d", len(args)+1)
		args = append(args, "%"+q.Search+"%")
	}
	return where, args
}

func (r *prestasiPostgresRepository) FindAll(
	ctx context.Context, q model.PrestasiListQuery,
) ([]model.Prestasi, int, error) {
	where, args := prestasibuildFilter(q)

	var total int
	err := r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM prestasi"+where, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("Menghitung jumlah mahasiswa: %w", err)
	}

	mode := "ASC"

	if q.Order == "desc" {
		mode = "DESC"
	}

	sqlText := fmt.Sprintf(
		`SELECT id, student_id, nama_prestasi, juara, created_at 
		FROM prestasi%s
		ORDER BY %s
		LIMIT $%d OFFSET $%d`,
		where, mode, len(args)+1, len(args)+2,
	)

	args = append(args, q.Limit, q.Offset())

	rows, err := r.pool.Query(ctx, sqlText, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("Mengambil daftar [prestasi]: %w", err)
	}
	defer rows.Close()

	hasil := []model.Prestasi{}
	for rows.Next() {
		var u model.Prestasi
		if err := rows.Scan(&u.ID, &u.StudentID, &u.NamaPrestasi, &u.Juara, &u.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("Membaca baris mahasiswa: %w", err)
		}
		hasil = append(hasil, u)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("Membaca hasil query: %w", err)
	}

	return hasil, total, nil
}

func (r *prestasiPostgresRepository) FindByID(ctx context.Context, id int) (model.Prestasi, error) {
	var p model.Prestasi
	err := r.pool.QueryRow(ctx,
		`SELECT id, student_id, nama_prestasi, juara, created_at 
		FROM prestasi WHERE id = $1`, id,
	).Scan(&p.ID, &p.StudentID, &p.NamaPrestasi, &p.Juara, &p.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.Prestasi{}, ErrNotFound
		}
		return model.Prestasi{}, fmt.Errorf("mengambil prestasi: %w", err)
	}

	return p, nil
}