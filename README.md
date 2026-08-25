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
| **GET** | `/api/v1/health` | *Tidak ada* | *Tidak ada body* | `200 OK` | ```json<br>{<br>  "success": true,<br>  "message": "server sudah berjalan",<br>  "data": { "timestamp": "2026-05-13T07:00:00Z" }<br>}``` |
| **GET** | `/api/v1/students/` | **Query Params (opsional):**<br>`page` int (default `1`, min `1`)<br>`limit` int (default `10`, 1–100)<br>`search` string — cari substring pada `name` (case-insensitive)<br>`sort` string — `id` \| `nim` \| `email` \| `created_at` (default `id`, selain itu fallback ke `id`)<br>`order` string — `asc` \| `desc` (default `asc`)<br>`is_active` bool — `true`/`false` | *Tidak ada body* | `200 OK` | ```json<br>// GET /api/v1/students?search=budi&is_active=true&page=1&limit=10&sort=nim&order=asc<br>{<br>  "success": true,<br>  "message": "daftar mahasiswa berhasil diambil",<br>  "data": [<br>    { "id": 1, "nim": 21001, "username": "Budi", "grade": 3.5, "is_active": true }<br>  ],<br>  "meta": { "page": 1, "limit": 10, "total": 1, "total_pages": 1 }<br>}``` |
| **GET** | `/api/v1/students/:id` | **Path Param:**<br>`id` int — harus angka positif (`>0`) | *Tidak ada body* | `200 OK` — ditemukan<br>`400 Bad Request` — `id` bukan angka / ≤ 0<br>`404 Not Found` — ID tidak ada | **200:**<br>```json<br>{<br>  "success": true,<br>  "message": "mahasiswa ditemukan",<br>  "data": { "id": 1, "nim": 21001, "username": "Budi", "grade": 3.75, "is_active": true }<br>}```<br>**400:**<br>```json<br>{ "success": false, "message": "id harus berupa angka positif" }```<br>**404:**<br>```json<br>{ "success": false, "message": "mahasiswa tidak ditemukan" }``` |
| **POST** | `/api/v1/students/` | **Header wajib:**<br>`Content-Type: application/json`<br>**Body (JSON, wajib):**<br>`nim` int — wajib, `!=0`<br>`name` string — wajib, tidak boleh kosong (di-trim)<br>`grade` float — wajib, `0.0–4.0` | ```json<br>{<br>  "nim": 21001,<br>  "name": "Budi Santoso",<br>  "grade": 3.75<br>}``` | `201 Created` — berhasil (header `Location: /api/v1/students/:id`)<br>`400 Bad Request` — body bukan JSON valid<br>`415 Unsupported Media Type` — `Content-Type` bukan `application/json`<br>`422 Unprocessable Entity` — validasi gagal | **201:**<br>```json<br>{<br>  "success": true,<br>  "message": "mahasiswa berhasil dibuat",<br>  "data": { "id": 1, "nim": 21001, "username": "Budi Santoso", "grade": 3.75, "is_active": true }<br>}```<br>**422:**<br>```json<br>{<br>  "success": false,<br>  "message": "validasi gagal",<br>  "error": {<br>    "name": "wajib diisi",<br>    "nim": "NIM wajib diisi",<br>    "grade": "grade harus antara 0.0 - 4.0"<br>  }<br>}```<br>**415:**<br>```json<br>{ "success": false, "message": "Content-Type harus application/json" }``` |
| **PUT** | `/api/v1/students/:id` | **Path Param:**<br>`id` int (positif)<br>**Header:**<br>`Content-Type: application/json`<br>**Body (JSON, semua field wajib):**<br>`name` string — wajib<br>`nim` int — wajib, `!=0`<br>`grade` float — `0.0–4.0`<br>`is_active` bool | ```json<br>{<br>  "name": "Budi Update",<br>  "nim": 21001,<br>  "grade": 3.9,<br>  "is_active": false<br>}``` | `200 OK` — berhasil diganti<br>`400 Bad Request` — ID tidak valid / body bukan JSON<br>`404 Not Found` — ID tidak ada<br>`415 Unsupported Media Type` — Content-Type salah<br>`422 Unprocessable Entity` — validasi gagal | **200:**<br>```json<br>{<br>  "success": true,<br>  "message": "mahasiswa berhasil diganti seluruhnya",<br>  "data": { "id": 1, "nim": 21001, "username": "Budi Update", "grade": 3.9, "is_active": false }<br>}```<br>**422:**<br>```json<br>{<br>  "success": false,<br>  "message": "validasi gagal",<br>  "error": { "name": "wajib diisi pada PUT", "grade": "grade harus antara 0.0 - 4.0" }<br>}``` |
| **PATCH** | `/api/v1/students/:id` | **Path Param:**<br>`id` int (positif)<br>**Header:**<br>`Content-Type: application/json`<br>**Body (JSON, parsial — hanya field yang ingin diubah):**<br>`name` *string (opsional)<br>`nim` *int (opsional)<br>`grade` *float (opsional, `0.0–4.0`)<br>`is_active` *bool (opsional) | ```json<br>{<br>  "grade": 3.85,<br>  "is_active": false<br>}```<br>atau<br>```json<br>{ "name": "Budi Patch" }``` | `200 OK` — berhasil diperbarui<br>`400 Bad Request` — ID tidak valid / body bukan JSON<br>`404 Not Found` — ID tidak ada<br>`415 Unsupported Media Type`<br>`422 Unprocessable Entity` — field yang dikirim tidak valid | **200:**<br>```json<br>{<br>  "success": true,<br>  "message": "mahasiswa berhasil diperbarui",<br>  "data": { "id": 1, "nim": 21001, "username": "Budi Patch", "grade": 3.85, "is_active": false }<br>}```<br>**422 (contoh name kosong):**<br>```json<br>{<br>  "success": false,<br>  "message": "validasi gagal",<br>  "error": { "name": "nama tidak boleh kosong" }<br>}``` |
| **DELETE** | `/api/v1/students/:id` | **Path Param:**<br>`id` int (positif) | *Tidak ada body* | `204 No Content` — berhasil dihapus (tanpa body)<br>`400 Bad Request` — ID tidak valid<br>`404 Not Found` — ID tidak ada | **204:** *(body kosong)*<br>**404:**<br>```json<br>{ "success": false, "message": "mahasiswa tidak ditemukan" }``` |
| **ANY** | `/*` (endpoint tidak terdaftar) | *Tidak ada* | *Tidak ada* | `404 Not Found` | ```json<br>{ "success": false, "message": "endpoint tidak ditemukan" }``` |
| **ANY** | Error internal (panic / ErrorHandler) | *Tidak ada* | *Tidak ada* | `500 Internal Server Error` | ```json<br>{ "success": false, "message": "terjadi kesalahan server" }``` |

> **Catatan penting:**
> - Field `Name` pada struct Go memiliki tag JSON `username`, sehingga respons menggunakan kunci `username` bukan `name`. Request tetap memakai `name`.
> - `sort` yang diizinkan di `helper.go` adalah `id`, `nim`, `email`, `created_at`; nilai lain akan fallback ke `id`. Sorting aktual pada `handler.go` mendukung `nim`, `name`, `grade`, `id` — perhatikan inkonsistensi ini.
> - Semua endpoint `POST`/`PUT`/`PATCH` melewati middleware `requireJSON` → mengembalikan `415` jika `Content-Type` bukan `application/json`.

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
