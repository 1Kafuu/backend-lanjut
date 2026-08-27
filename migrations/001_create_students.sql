CREATE TABLE IF NOT EXISTS students (
    id SERIAL PRIMARY KEY,
    nim VARCHAR(20) NOT NULL UNIQUE,
    name VARCHAR(100) NOT NULL,
    grade SMALLINT NOT NULL CHECK (grader >= 0 AND grade <= 100),
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_student_name ON students(name);