package service

import (
	"errors"
	"strconv"

	"github.com/gofiber/fiber/v2"

	"api-students/app/model"
	"api-students/app/repository"
	"api-students/helper"
)

// CourseService menangani operasi kelas dan KRS.
type CourseService struct {
	courses repository.CourseRepository
	tx      *repository.TxManager
}

// NewCourseService membuat service dengan repository kelas.
func NewCourseService(courses repository.CourseRepository, tx *repository.TxManager) *CourseService {
	return &CourseService{courses: courses, tx: tx}
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

var errCreditLimit = errors.New("batas SKS terlampaui")

// Enroll mendaftarkan mahasiswa ke satu kelas.
//
// Urutan langkah: kunci KRS mahasiswa, periksa batas SKS, ambil kursi
// secara atomik, lalu catat pendaftaran. Semua lewat repo yang terikat
// pada transaction supaya kegagalan di langkah mana pun membatalkan
// seluruhnya.
func (s *CourseService) Enroll(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	current, ok := helper.CurrentUser(c)
	if !ok {
		return helper.Unauthorized("belum terautentikasi")
	}

	courseID, valid := helper.ParamID(c)
	if !valid {
		return helper.BadRequest("id harus berupa angka positif")
	}

	var enrolled model.Course

	err := s.tx.WithTx(ctx, func(tx repository.DBTX) error {
		repo := s.courses.WithTx(tx)

		// 1. Kunci KRS milik mahasiswa ini.
		if err := repo.LockStudent(ctx, current.UserID); err != nil {
			return err
		}

		// 2. Periksa batas SKS.
		course, err := repo.FindByID(ctx, courseID)
		if err != nil {
			return err
		}
		taken, err := repo.SumCredits(ctx, current.UserID)
		if err != nil {
			return err
		}
		if !WithinCreditLimit(taken, course.Credits) {
			return errCreditLimit
		}

		// 3. Ambil satu kursi secara atomik.
		enrolled, err = repo.IncrementEnrolled(ctx, courseID)
		if err != nil {
			return err
		}

		// 4. Catat pendaftaran. Bila sudah terdaftar, error di sini
		//    membatalkan juga penambahan kursi pada langkah 3.
		return repo.InsertEnrollment(ctx, courseID, current.UserID)
	})
	if err != nil {
		return translateCourseError(err)
	}

	return helper.Created(c, "berhasil mendaftar kelas", enrolled,
		"/api/v1/courses/"+strconv.Itoa(courseID))
}

// Drop membatalkan pendaftaran mahasiswa pada satu kelas.
func (s *CourseService) Drop(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	current, ok := helper.CurrentUser(c)
	if !ok {
		return helper.Unauthorized("belum terautentikasi")
	}

	courseID, valid := helper.ParamID(c)
	if !valid {
		return helper.BadRequest("id harus berupa angka positif")
	}

	err := s.tx.WithTx(ctx, func(tx repository.DBTX) error {
		repo := s.courses.WithTx(tx)

		if err := repo.LockStudent(ctx, current.UserID); err != nil {
			return err
		}
		if err := repo.DeleteEnrollment(ctx, courseID, current.UserID); err != nil {
			return err
		}
		_, err := repo.DecrementEnrolled(ctx, courseID)
		return err
	})
	if err != nil {
		return translateCourseError(err)
	}

	return helper.NoContent(c)
}

// translateCourseError memetakan error repository dan aturan bisnis menjadi response HTTP. Hanya fungsi ini yang mengetahui kode error repository.
func translateCourseError(err error) error {
	switch {
	case errors.Is(err, repository.ErrNotFound):
		return helper.NotFound("kelas tidak ditemukan")
	case errors.Is(err, repository.ErrCourseFull):
		return helper.NewError(fiber.StatusConflict, "COURSE_FULL", "kuota kelas sudah penuh")
	case errors.Is(err, repository.ErrDuplicate):
		return helper.NewError(fiber.StatusConflict, "ALREADY_ENROLLED", "Anda sudah terdaftar di kelas ini")
	case errors.Is(err, repository.ErrNotEnrolled):
		return helper.NewError(fiber.StatusConflict, "NOT_ENROLLED", "Anda tidak terdaftar di kelas tersebut")
	case errors.Is(err, errCreditLimit):
		return helper.NewError(fiber.StatusConflict, "CREDIT_LIMIT_EXCEEDED",
			"batas "+strconv.Itoa(MaxCreditsPerSemester)+" SKS terlampaui")
	default:
		return helper.Internal(err)
	}
}
