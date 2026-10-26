package service

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	"api-students/app/repository"
	"api-students/helper"
)

// CourseService menangani operasi kelas dan KRS.
type CourseService struct {
	courses repository.CourseRepository
}

// NewCourseService membuat service dengan repository kelas.
func NewCourseService(courses repository.CourseRepository) *CourseService {
	return &CourseService{courses: courses}
}

// List mengembalikan seluruh kelas.
func (s *CourseService) List(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	courses, err := s.courses.FindAll(ctx)
	if err != nil {
		return helper.Internal(err)
	}
	return helper.Success(c, fiber.StatusOK, "daftar kelas berhasil diambil", courses)
}

// Mine mengembalikan KRS mahasiswa yang sedang login beserta total SKS.
func (s *CourseService) Mine(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	current, ok := helper.CurrentUser(c)
	if !ok {
		return helper.Unauthorized("belum terautentikasi")
	}

	courses, err := s.courses.FindByStudent(ctx, current.UserID)
	if err != nil {
		return helper.Internal(err)
	}

	total := 0
	for _, course := range courses {
		total += course.Credits
	}
	return helper.Success(c, fiber.StatusOK, "KRS berhasil diambil", fiber.Map{
		"courses":       courses,
		"total_credits": total,
		"max_credits":   MaxCreditsPerSemester,
	})
}

// Get mengembalikan satu kelas berdasarkan id.
func (s *CourseService) Get(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	id, valid := helper.ParamID(c)
	if !valid {
		return helper.BadRequest("id harus berupa angka positif")
	}

	course, err := s.courses.FindByID(ctx, id)
	if err != nil {
		return translateCourseError(err)
	}
	return helper.Success(c, fiber.StatusOK, "kelas ditemukan", course)
}

// translateCourseError memetakan error repository dan aturan bisnis menjadi response HTTP. Hanya fungsi ini yang mengetahui kode error repository.
func translateCourseError(err error) error {
	switch {
	case errors.Is(err, repository.ErrNotFound):
		return helper.NotFound("kelas tidak ditemukan")
	default:
		return helper.Internal(err)
	}
}
