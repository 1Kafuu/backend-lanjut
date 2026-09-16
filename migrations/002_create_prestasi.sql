-- Active: 1779062697284@@127.0.0.1@5432@praktikum_backend
CREATE TABLE IF NOT EXISTS prestasi (
    id SERIAL PRIMARY KEY,
    student_id INT NOT NULL REFERENCES students(id) ON DELETE CASCADE,
    nama_prestasi VARCHAR(150) NOT NULL,
    juara VARCHAR(50) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_prestasi_student_id ON prestasi(student_id);