# API Students — Praktikum Backend Lanjut Minggu 2

REST API sederhana untuk manajemen data mahasiswa (CRUD + pencarian, filter, sort & pagination) dibangun dengan **Go** + **Fiber v2**. Penyimpanan menggunakan slice in-memory (tanpa database).

- **Base URL:** `http://localhost:3000`
- **Prefix API:** `/api/v1`
- **Content-Type:** `application/json` (wajib untuk `POST`, `PUT`, `PATCH`)
- **Format Respons Umum:**

```json
{
  "success": true,
  "message": "pesan deskriptif",
  "data": {},
  "meta": { "page": 1, "limit": 10, "total": 25, "total_pages": 3 },
  "error": { "field": "pesan error" }
}
```

---

## Cara Menjalankan

```bash
go mod tidy
go run .
# Server berjalan pada http://localhost:3000
```

## Tech Stack

| Komponen | Teknologi |
|---|---|
| Bahasa | Go 1.26.5 |
| Framework | Fiber v2 |
| Middleware | `requestid`, `logger`, `cors` |

## Struktur Proyek

```
api-students/
├── main.go      # Setup Fiber, middleware, routing, error handler
├── handler.go   # Handler CRUD students + logic pencarian/sort/pagination
├── model.go     # Struct Student, Request DTO, WebResponse, Meta, ListQuery
├── helper.go    # Helper respons (ok, created, fail, ...) & parseListQuery
├── go.mod
└── README.md
```

---

## Kontrak API

### Ringkasan Endpoint

| # | Metode | Endpoint | Deskripsi |
|---|---|---|---|
| 1 | GET | `/` | Cek root server |
| 2 | GET | `/api/v1/health` | Health check + timestamp |
| 3 | GET | `/api/v1/students/` | Daftar mahasiswa (filter, search, sort, pagination) |
| 4 | GET | `/api/v1/students/:id` | Detail mahasiswa by ID |
| 5 | POST | `/api/v1/students/` | Buat mahasiswa baru |
| 6 | PUT | `/api/v1/students/:id` | Ganti seluruh data mahasiswa |
| 7 | PATCH | `/api/v1/students/:id` | Ubah sebagian data mahasiswa |
| 8 | DELETE | `/api/v1/students/:id` | Hapus mahasiswa |

### Tabel Kontrak Lengkap

| Metode | Endpoint | Parameter | Contoh Body Permintaan | Status yang Mungkin Dikembalikan | Contoh Respons |
|---|---|---|---|---|---|
| **GET** | `/` | *Tidak ada* | *Tidak ada body* | `200 OK` | `Hello, World!` (text/plain) |
| **GET** | `/api/v1/health` | *Tidak ada* | *Tidak ada body* | `200 OK` | `{"success": true, "message": "server sudah berjalan", "data": {"timestamp": "2026-05-13T07:00:00Z"}}` |
| **GET** | `/api/v1/students/` | Query Params (opsional): `page` int default `1`, `limit` int default `10` (1–100), `search` string cari substring pada `name`, `sort` = `id`/`nim`/`name`/`grade` default `id`, `order` = `asc`/`desc` default `asc`, `is_active` bool `true`/`false` | *Tidak ada body* | `200 OK` | `{"success": true, "message": "daftar mahasiswa berhasil diambil", "data": [{"id": 1, "nim": 21001, "username": "Budi", "grade": 3.5, "is_active": true}], "meta": {"page": 1, "limit": 10, "total": 1, "total_pages": 1}}` |
| **GET** | `/api/v1/students/:id` | Path Param: `id` int harus angka positif (>0) | *Tidak ada body* | `200 OK` ditemukan, `400 Bad Request` id bukan angka/≤0, `404 Not Found` ID tidak ada | 200: `{"success": true, "message": "mahasiswa ditemukan", "data": {"id": 1, "nim": 21001, "username": "Budi", "grade": 3.75, "is_active": true}}` — 400: `{"success": false, "message": "id harus berupa angka positif"}` — 404: `{"success": false, "message": "mahasiswa tidak ditemukan"}` |
| **POST** | `/api/v1/students/` | Header wajib: `Content-Type: application/json` — Body JSON wajib: `nim` int !=0, `name` string wajib/tidak kosong, `grade` float 0.0–4.0 | `{"nim": 21001, "name": "Budi Santoso", "grade": 3.75}` | `201 Created` berhasil + header `Location`, `400 Bad Request` body bukan JSON valid, `409 Conflict` NIM duplikat, `415 Unsupported Media Type` Content-Type salah, `422 Unprocessable Entity` validasi gagal | 201: `{"success": true, "message": "mahasiswa berhasil dibuat", "data": {"id": 1, "nim": 21001, "username": "Budi Santoso", "grade": 3.75, "is_active": true}}` — 409: `{"success": false, "message": "NIM sudah terdaftar", "error": {"nim": "NIM sudah digunakan untuk mahasiswa lain"}}` — 422: `{"success": false, "message": "validasi gagal", "error": {"name": "wajib diisi", "nim": "NIM wajib diisi", "grade": "grade harus antara 0.0 - 4.0"}}` — 415: `{"success": false, "message": "Content-Type harus application/json"}` |
| **PUT** | `/api/v1/students/:id` | Path Param: `id` int positif — Header: `Content-Type: application/json` — Body JSON semua field wajib: `name` string, `nim` int !=0, `grade` float 0.0–4.0, `is_active` bool | `{"name": "Budi Update", "nim": 21001, "grade": 3.9, "is_active": false}` | `200 OK` berhasil diganti, `400 Bad Request` ID tidak valid/body bukan JSON, `404 Not Found` ID tidak ada, `409 Conflict` NIM duplikat milik mahasiswa lain, `415 Unsupported Media Type`, `422 Unprocessable Entity` validasi gagal | 200: `{"success": true, "message": "mahasiswa berhasil diganti seluruhnya", "data": {"id": 1, "nim": 21001, "username": "Budi Update", "grade": 3.9, "is_active": false}}` — 409: `{"success": false, "message": "NIM sudah terdaftar", "error": {"nim": "NIM sudah digunakan untuk mahasiswa lain"}}` — 422: `{"success": false, "message": "validasi gagal", "error": {"name": "wajib diisi pada PUT"}}` |
| **PATCH** | `/api/v1/students/:id` | Path Param: `id` int positif — Header: `Content-Type: application/json` — Body JSON parsial: `name` string opsional, `nim` int opsional, `grade` float 0.0–4.0 opsional, `is_active` bool opsional | `{"grade": 3.85, "is_active": false}` atau `{"name": "Budi Patch"}` | `200 OK` berhasil diperbarui, `400 Bad Request`, `404 Not Found`, `409 Conflict` NIM duplikat milik mahasiswa lain, `415 Unsupported Media Type`, `422 Unprocessable Entity` field tidak valid | 200: `{"success": true, "message": "mahasiswa berhasil diperbarui", "data": {"id": 1, "nim": 21001, "username": "Budi Patch", "grade": 3.85, "is_active": false}}` — 409: `{"success": false, "message": "NIM sudah terdaftar", "error": {"nim": "NIM sudah digunakan untuk mahasiswa lain"}}` — 422: `{"success": false, "message": "validasi gagal", "error": {"name": "nama tidak boleh kosong"}}` |
| **DELETE** | `/api/v1/students/:id` | Path Param: `id` int positif | *Tidak ada body* | `204 No Content` berhasil dihapus (tanpa body), `400 Bad Request` ID tidak valid, `404 Not Found` ID tidak ada | 204: *(body kosong)* — 404: `{"success": false, "message": "mahasiswa tidak ditemukan"}` |
| **ANY** | `/*` (endpoint tidak terdaftar) | *Tidak ada* | *Tidak ada* | `404 Not Found` | `{"success": false, "message": "endpoint tidak ditemukan"}` |
| **ANY** | Error internal (panic / ErrorHandler) | *Tidak ada* | *Tidak ada* | `500 Internal Server Error` | `{"success": false, "message": "terjadi kesalahan server"}` |

> **Catatan penting:**
> - Field `Name` pada struct Go memiliki tag JSON `username`, sehingga respons menggunakan kunci `username` bukan `name`. Request tetap memakai `name`.
> - `sort` yang diizinkan di `helper.go` adalah `id`, `nim`, `name`, `grade`; nilai lain akan fallback ke `id`.
> - Semua endpoint `POST`/`PUT`/`PATCH` melewati middleware `requireJSON` → mengembalikan `415` jika `Content-Type` bukan `application/json`.
> - `409 Conflict` dikembalikan oleh `POST`/`PUT`/`PATCH /api/v1/students` jika `nim` duplikat (sudah digunakan mahasiswa lain) — helper `failConflict` di `helper.go` dengan body `{"success": false, "message": "NIM sudah terdaftar", "error": {"nim": "NIM sudah digunakan untuk mahasiswa lain"}}`.

### Detail Contoh Respons (Pretty JSON)

**GET /api/v1/students?search=budi&is_active=true&page=1&limit=10**
```json
{
  "success": true,
  "message": "daftar mahasiswa berhasil diambil",
  "data": [
    { "id": 1, "nim": 21001, "username": "Budi", "grade": 3.5, "is_active": true }
  ],
  "meta": { "page": 1, "limit": 10, "total": 1, "total_pages": 1 }
}
```

**POST /api/v1/students/ — 422 Validation Error**
```json
{
  "success": false,
  "message": "validasi gagal",
  "error": {
    "name": "wajib diisi",
    "nim": "NIM wajib diisi",
    "grade": "grade harus antara 0.0 - 4.0"
  }
}
```

**POST / PUT / PATCH /api/v1/students — 409 Conflict (NIM duplikat)**
```json
{
  "success": false,
  "message": "NIM sudah terdaftar",
  "error": {
    "nim": "NIM sudah digunakan untuk mahasiswa lain"
  }
}
```

---

## Model Data

### Student

| Field | Tipe | JSON Key | Keterangan |
|---|---|---|---|
| ID | int | `id` | Auto-increment |
| NIM | int | `nim` | Nomor induk mahasiswa |
| Name | string | `username` | Nama mahasiswa (tag JSON `username`) |
| Grade | float64 | `grade` | IPK `0.0 – 4.0` |
| IsActive | bool | `is_active` | Status aktif (default `true` saat create) |

### Contoh cURL

```bash
# Health
curl http://localhost:3000/api/v1/health

# List dengan filter & pagination
curl "http://localhost:3000/api/v1/students?search=budi&is_active=true&page=1&limit=5&sort=nim&order=asc"

# Create
curl -X POST http://localhost:3000/api/v1/students/ \
  -H "Content-Type: application/json" \
  -d '{"nim":21001,"name":"Budi","grade":3.75}'

# Get by ID
curl http://localhost:3000/api/v1/students/1

# PUT (replace)
curl -X PUT http://localhost:3000/api/v1/students/1 \
  -H "Content-Type: application/json" \
  -d '{"name":"Budi Update","nim":21001,"grade":3.9,"is_active":false}'

# PATCH (partial)
curl -X PATCH http://localhost:3000/api/v1/students/1 \
  -H "Content-Type: application/json" \
  -d '{"grade":3.85}'

# DELETE
curl -X DELETE http://localhost:3000/api/v1/students/1
```
