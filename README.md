# API Students — Praktikum Backend Lanjut Minggu 3

REST API manajemen data mahasiswa dengan **Go + Fiber v2 + PostgreSQL + Repository Pattern**. Pada Minggu 3 ini seluruh logika *filtering, searching, sorting, pagination,* dan *total count* dipindahkan ke **SQL** (parameterized query, bukan lagi di Go).

Base URL: `http://localhost:3000` — Prefix: `/api/v1`

---

## Tech Stack

| Komponen | Teknologi |
|---|---|
| Bahasa | Go 1.26.5 |
| Framework | Fiber v2 |
| Database | PostgreSQL 18 + `pgxpool` |
| Env | `godotenv` |

---

## Skema Tabel `students`

Didefinisikan di `migrations/001_create_students.sql`:

```sql
CREATE TABLE IF NOT EXISTS students (
    id         SERIAL PRIMARY KEY,
    nim        VARCHAR(20) NOT NULL UNIQUE,
    name       VARCHAR(100) NOT NULL,
    grade      DOUBLE PRECISION NOT NULL CHECK (grade >= 0 AND grade <= 4),
    is_active  BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_student_name ON students(name);
```

| Kolom | Tipe | Keterangan |
|---|---|---|
| `id` | `SERIAL` | Primary key, auto-increment |
| `nim` | `VARCHAR(20) UNIQUE` | Nomor induk, **UNIQUE** dijaga oleh DB agar tidak duplikat (mencegah race condition yang tidak bisa dicegah hanya dengan validasi Go). Error duplikat `23505` → `409 Conflict` |
| `name` | `VARCHAR(100)` | Nama — pencarian via `ILIKE '%keyword%'` |
| `grade` | `DOUBLE PRECISION` | IPK 0.0–4.0, dibatasi `CHECK` di DB |
| `is_active` | `BOOLEAN` | Status aktif |
| `created_at` | `TIMESTAMPTZ` | Diisi otomatis `NOW()` |

**Indeks:** `idx_student_name ON students(name)` mempercepat `WHERE name ILIKE` dan `ORDER BY name`. Indeks `students_nim_key` (dari `UNIQUE nim`) mempercepat pengecekan duplikat.

---

## Struktur Project

```
.
├── migrations/001_create_students.sql   # skema + indeks
├── database/postgres.go                 # connection pool + Ping
├── config/env.go                       # loader .env
├── app/model/student.go                 # Student, DTO, ListQuery, Meta
├── app/repository/student_repository.go # interface + implementasi Postgres
├── main.go, handler.go, helper.go
├── .env.example                         # template env (nilai kosong)
├── .env                                 # kredensial lokal (di-ignore Git)
└── go.mod
```

Repository (`app/repository`) tidak bergantung pada Fiber — hanya `pgxpool` — sehingga mudah diganti framework.

---

## Environment Variable

Kredensial disimpan di `.env` (tidak di-commit). `.gitignore` sudah berisi `.env`.

**`.env.example`** (yang ada di Git — nilai kosong):
```
APP_PORT=
DB_HOST=
DB_PORT=
DB_USER=
DB_PASSWORD=
DB_NAME=
DB_SSLMODE=
DB_MAX_CONNS=
```

**`.env`** (buat sendiri dari template, contoh dummy):
```
APP_PORT=3000
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=isi_password_postgres_kamu_disini
DB_NAME=praktikum_backend
DB_SSLMODE=disable
DB_MAX_CONNS=10
```

| Variabel | Contoh | Deskripsi |
|---|---|---|
| `APP_PORT` | `3000` | Port Fiber |
| `DB_HOST` / `DB_PORT` | `localhost` / `5432` | Alamat PostgreSQL |
| `DB_USER` / `DB_PASSWORD` | `postgres` / `isi_password...` | Kredensial DB — ganti dengan password instalasi lokal |
| `DB_NAME` | `praktikum_backend` | Nama database |
| `DB_SSLMODE` | `disable` | `disable` untuk lokal |
| `DB_MAX_CONNS` | `10` | Maks koneksi pool |

---

## Persiapan Database Dari Nol

1. **Pastikan PostgreSQL berjalan** dan `psql` tersedia (`psql --version`).
2. **Buat database:**
   ```bash
   psql -U postgres -h localhost -c "CREATE DATABASE praktikum_backend;"
   ```
3. **Siapkan env:**
   ```bash
   cp .env.example .env
   # lalu isi DB_PASSWORD dll di .env
   ```
4. **Jalankan migrasi:**
   ```bash
   psql -U postgres -h localhost -d praktikum_backend -f migrations/001_create_students.sql
   ```

5. **Verifikasi tabel:**
   ```bash
   psql -U postgres -h localhost -d praktikum_backend -c "\d students"
   # harus: id integer, nim varchar(20) UNIQUE, name varchar(100), grade double precision CHECK 0-4, is_active boolean, created_at timestamptz, idx_student_name
   ```

---

## Menjalankan Aplikasi

```bash
go mod tidy
go run .
```

Server berjalan di `http://localhost:3000`. `database/postgres.go` membangun `pgxpool` (`MaxConns 10`, `MinConns 2`, `MaxConnLifetime 1h`) dan melakukan `Ping` saat startup — jika gagal, aplikasi tidak akan start.

Cek kesehatan (sekaligus cek koneksi DB):
```bash
curl http://localhost:3000/api/v1/health
# DB hidup: {"success":true,"message":"server dan database berjalan"}
# DB mati : {"success":false,"message":"database tidak dapat dihubungi"} 503

curl http://localhost:3000/
# sama — keduanya Ping DB, sesuai checklist
```

Matikan DB untuk membuktikan `503` (butuh screenshot laporan): stop service PostgreSQL → `curl /health` harus `503`, start lagi → `200`.

---

## API Endpoints

| Metode | Endpoint | Keterangan |
|---|---|---|
| GET | `/` | Root + cek DB |
| GET | `/api/v1/health` | Health + cek DB |
| GET | `/api/v1/students/?page=&limit=&search=&sort=&order=&is_active=&min_grade=&max_grade=` | List (semua query di SQL) |
| GET | `/api/v1/students/:id` | Detail |
| POST | `/api/v1/students/` | Buat |
| PUT | `/api/v1/students/:id` | Ganti semua field |
| PATCH | `/api/v1/students/:id` | Ganti sebagian |
| DELETE | `/api/v1/students/:id` | Hapus |

**Query `GET /students`:**
- `search` → `WHERE name ILIKE '%keyword%'` (parameterized, partial match)
- `is_active`, `min_grade`, `max_grade` → `WHERE is_active = $1 AND grade >= $2`
- `sort` whitelist `id, nim, name, grade, created_at` + `order asc/desc` → `ORDER BY` (fallback `id` jika tidak di whitelist — anti injection)
- `page`, `limit` → `LIMIT $1 OFFSET $2` (`offset = (page-1)*limit`)
- `total` → `SELECT COUNT(*) ... WHERE` sama dengan query data → `meta.total`

**Status:**
`200 OK`, `201 Created`, `204 No Content`, `400 Bad Request` (ID bukan angka / Content-Type salah), `422` validasi, `404` ID tidak ada, `409` NIM duplikat, `503` DB mati, `500` error lain.

### Contoh `curl`

```bash
curl http://localhost:3000/api/v1/health

curl "http://localhost:3000/api/v1/students?search=budi&is_active=true&page=1&limit=5&sort=nim&order=asc"

curl -X POST http://localhost:3000/api/v1/students/ \
  -H "Content-Type: application/json" \
  -d '{"nim":21001,"name":"Budi Santoso","grade":3.75}'

curl http://localhost:3000/api/v1/students/1

curl -X PUT http://localhost:3000/api/v1/students/1 \
  -H "Content-Type: application/json" \
  -d '{"name":"Budi Update","nim":21001,"grade":3.9,"is_active":false}'

curl -X PATCH http://localhost:3000/api/v1/students/1 \
  -H "Content-Type: application/json" \
  -d '{"grade":3.85}'

curl -X DELETE http://localhost:3000/api/v1/students/1
```

## Troubleshooting

- `psql: command not found` → tambah `C:\Program Files\PostgreSQL\18\bin` ke PATH.
- `password authentication failed` → `DB_PASSWORD` di `.env` salah.
- `relation "students" does not exist` → belum `psql -f migrations/001_create_students.sql`.
- `bind: address already in use :3000` → `netstat -ano | findstr :3000` → `taskkill /F /PID <PID>`.
- `git status` muncul `.env` → pastikan `.gitignore` berisi `.env`.

