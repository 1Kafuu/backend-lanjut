package main

import (
	"sort"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
)

var students []Student
var nextID = 1

func findStudentIndex(id int) int {
	for i := range students {
		if students[i].ID == id {
			return i
		}
	}
	return -1
}

// cocokPencarian apakah kata kunci muncul pada nama
func cocokPencarian(s Student, kata string) bool {
	kata = strings.ToLower(kata)
	return strings.Contains(strings.ToLower(s.Name), kata)
}

func paramID(c *fiber.Ctx) (int, bool) {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil || id < 1 {
		return 0, false
	}
	return id, true
}

func listStudents(c *fiber.Ctx) error {
	q := parseListQuery(c)

	// Saring
	hasil := []Student{}
	for _, u := range students {
		if q.IsActive != nil && u.IsActive != *q.IsActive {
			continue
		}
		if q.Search != "" && !cocokPencarian(u, q.Search) {
			continue
		}
		hasil = append(hasil, u)
	}

	// Urutkan
	sort.SliceStable(hasil, func(i, j int) bool {
		var lebihKecil bool
		switch q.Sort {
		case "nim":
			lebihKecil = hasil[i].NIM < hasil[j].NIM
		case "name":
			lebihKecil = hasil[i].Name < hasil[j].Name
		case "grade":
			lebihKecil = hasil[i].Grade < hasil[j].Grade
		default:
			lebihKecil = hasil[i].ID < hasil[j].ID
		}
		if q.Order == "desc" {
			return !lebihKecil
		}
		return lebihKecil
	})

	// Potong sesuai halaman
	total := len(hasil)
	totalPages := (total + q.Limit - 1) / q.Limit
	mulai := (q.Page - 1) * q.Limit
	if mulai > total {
		mulai = total
	}
	akhir := mulai + q.Limit
	if akhir > total {
		akhir = total
	}

	return okList(c, "daftar mahasiswa berhasil diambil", hasil[mulai:akhir], &Meta{
		Page: q.Page, Limit: q.Limit, Total: total, TotalPages: totalPages,
	})
}

func getStudents(c *fiber.Ctx) error {
	id, valid := paramID(c)
	if !valid {
		return fail(c, fiber.StatusBadRequest, "id harus berupa angka positif")
	}

	i := findStudentIndex(id)
	if i == -1 {
		return fail(c, fiber.StatusNotFound, "mahasiswa tidak ditemukan")
	}

	return ok(c, "mahasiswa ditemukan", students[i])
}

// POST createNewStudent
func createStudent(c *fiber.Ctx) error {
	var req CreateStudentRequest
	if err := c.BodyParser(&req); err != nil {
		return fail(c, fiber.StatusBadRequest, "body harus berupa JSON yang valid")
	}

	errs := map[string]string{}
	req.Name = strings.TrimSpace(req.Name)

	if req.Name == "" {
		errs["name"] = "wajib diisi"
	}
	if req.NIM == 0 {
		errs["nim"] = "NIM wajib diisi"
	}
	if req.Grade < 0 || req.Grade > 4 {
		errs["grade"] = "grade harus antara 0.0 - 4.0"
	}
	if len(errs) > 0 {
		return failValidation(c, errs)
	}
	for _, s := range students {
		if s.NIM == req.NIM {
			return failConflict(c, "NIM sudah terdaftar", map[string]string{
				"nim": "NIM sudah digunakan untuk mahasiswa lain",
			})
		}
	}

	baru := Student{
		ID:       nextID,
		NIM:      req.NIM,
		Name:     req.Name,
		Grade:    req.Grade,
		IsActive: true,
	}
	students = append(students, baru)
	nextID++

	return created(c, "mahasiswa berhasil dibuat", baru,
		"/api/v1/students/"+strconv.Itoa(baru.ID))
}

// PUT updateAllStudentField
func replaceStudent(c *fiber.Ctx) error {
	id, valid := paramID(c)
	if !valid {
		return fail(c, fiber.StatusBadRequest, "id harus berupa angka positif")
	}

	i := findStudentIndex(id)
	if i == -1 {
		return fail(c, fiber.StatusNotFound, "mahasiswa tidak ditemukan")
	}

	var req ReplaceStudentRequest
	if err := c.BodyParser(&req); err != nil {
		return fail(c, fiber.StatusBadRequest, "body harus berupa JSON yang valid")
	}

	errs := map[string]string{}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		errs["name"] = "wajib diisi pada PUT"
	}
	if req.NIM == 0 {
		errs["nim"] = "NIM wajib diisi pada PUT"
	}
	if req.Grade < 0 || req.Grade > 4 {
		errs["grade"] = "grade harus antara 0.0 - 4.0"
	}
	if len(errs) > 0 {
		return failValidation(c, errs)
	}
	for _, s := range students {
		if s.NIM == req.NIM && s.ID != id {
			return failConflict(c, "NIM sudah terdaftar", map[string]string{
				"nim": "NIM sudah digunakan untuk mahasiswa lain",
			})
		}
	}

	students[i].Name = req.Name
	students[i].NIM = req.NIM
	students[i].Grade = req.Grade
	students[i].IsActive = req.IsActive

	return ok(c, "mahasiswa berhasil diganti seluruhnya", students[i])
}

// PATCH updatePartialStudentField
func patchStudent(c *fiber.Ctx) error {
	id, valid := paramID(c)
	if !valid {
		return fail(c, fiber.StatusBadRequest, "id harus berupa angka positif")
	}

	i := findStudentIndex(id)
	if i == -1 {
		return fail(c, fiber.StatusNotFound, "mahasiswa tidak ditemukan")
	}

	var req PatchStudentRequest
	if err := c.BodyParser(&req); err != nil {
		return fail(c, fiber.StatusBadRequest, "body harus berupa JSON yang valid")
	}

	errs := map[string]string{}
	if req.Name != nil {
		*req.Name = strings.TrimSpace(*req.Name)
		if *req.Name == "" {
			errs["name"] = "nama tidak boleh kosong"
		} else {
			students[i].Name = *req.Name
		}
	}
	if req.NIM != nil {
		if *req.NIM == 0 {
			errs["nim"] = "NIM tidak valid"
		} else {
			for _, s := range students {
				if s.NIM == *req.NIM && s.ID != id {
					return failConflict(c, "NIM sudah terdaftar", map[string]string{
						"nim": "NIM sudah digunakan untuk mahasiswa lain",
					})
				}
			}
			if len(errs) == 0 {
				students[i].NIM = *req.NIM
			}
		}
	}
	if req.Grade != nil {
		if *req.Grade < 0 || *req.Grade > 4 {
			errs["grade"] = "grade harus 0.0 - 4.0"
		} else {
			students[i].Grade = *req.Grade
		}
	}
	if req.IsActive != nil {
		students[i].IsActive = *req.IsActive
	}
	if len(errs) > 0 {
		return failValidation(c, errs)
	}

	return ok(c, "mahasiswa berhasil diperbarui", students[i])
}

// DELETE deleteStudent
func deleteStudent(c *fiber.Ctx) error {
	id, valid := paramID(c)
	if !valid {
		return fail(c, fiber.StatusBadRequest, "id harus berupa angka positif")
	}

	i := findStudentIndex(id)
	if i == -1 {
		return fail(c, fiber.StatusNotFound, "mahasiswa tidak ditemukan")
	}

	students = append(students[:i], students[i+1:]...)

	return noContent(c) //204: berhasil dan memang tidak ada yang perlu dikirim
}
