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
	outbox  repository.OutboxRepository
	tx      *repository.TxManager
}

// NewCourseService membuat service dengan repository kelas.
func NewCourseService(
	courses repository.CourseRepository,
	outbox repository.OutboxRepository,
	tx *repository.TxManager,
) *CourseService {
	return &CourseService{courses: courses, outbox: outbox, tx: tx}
}

// List mengembalikan seluruh kelas.
//
// List godoc
//
// @Summary      Daftar seluruh kelas
// @Tags         courses
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  model.WebResponse{data=[]model.Course}
// @Failure      401  {object}  model.ErrorResponse  "Belum terautentikasi"
// @Router       /courses [get]
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
//
// Mine godoc
//
// @Summary      KRS saya
// @Description  Daftar kelas yang diikuti mahasiswa beserta total SKS.
// @Tags         courses
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  model.WebResponse{data=model.MyCourses}
// @Failure      401  {object}  model.ErrorResponse  "Belum terautentikasi"
// @Router       /courses/mine [get]
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
	return helper.Success(c, fiber.StatusOK, "KRS berhasil diambil", model.MyCourses{
		Courses:      courses,
		TotalCredits: total,
		MaxCredits:   MaxCreditsPerSemester,
	})
}

// Get mengembalikan satu kelas berdasarkan id.
//
// Get godoc
//
// @Summary      Detail satu kelas
// @Tags         courses
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  int  true  "ID kelas"
// @Success      200  {object}  model.WebResponse{data=model.Course}
// @Failure      400  {object}  model.ErrorResponse  "ID bukan angka positif"
// @Failure      404  {object}  model.ErrorResponse  "Kelas tidak ditemukan"
// @Router       /courses/{id} [get]
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
//
// Enroll godoc
//
// @Summary      Mendaftar ke sebuah kelas
// @Description  Menambah satu kursi dan menulis satu event enrollment.created ke outbox bila seluruh langkah berhasil.
// @Tags         courses
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  int  true  "ID kelas"
// @Success      201  {object}  model.WebResponse{data=model.Course}
// @Failure      400  {object}  model.ErrorResponse  "Body bukan JSON yang valid"
// @Failure      401  {object}  model.ErrorResponse  "Belum terautentikasi"
// @Failure      404  {object}  model.ErrorResponse  "Kelas tidak ditemukan"
// @Failure      409  {object}  model.ErrorResponse  "COURSE_FULL atau sudah terdaftar"
// @Router       /courses/{id}/enroll [post]
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

		// 4. Catat event-nya ke outbox di DALAM transaction yang sama.
		//    Urutan di dalam transaction tidak mengubah hasil akhir: bila
		//    langkah 5 gagal, catatan ini ikut dibatalkan bersama
		//    penambahan kursi pada langkah 3.
		if err := s.outbox.WithTx(tx).Insert(ctx, model.EventEnrollmentCreated, model.EnrollmentEvent{
			UserID: current.UserID, CourseID: enrolled.ID,
			CourseCode: enrolled.Code, CourseName: enrolled.Name,
		}); err != nil {
			return err
		}

		// 5. Catat pendaftarannya. Bila ternyata sudah terdaftar, error
		//    di sini membatalkan seluruh langkah di atas.
		return repo.InsertEnrollment(ctx, courseID, current.UserID)
	})
	if err != nil {
		return translateCourseError(err)
	}

	return helper.Created(c, "berhasil mendaftar kelas", enrolled,
		"/api/v1/courses/"+strconv.Itoa(courseID))
}

// Drop membatalkan pendaftaran mahasiswa pada satu kelas.
//
// Drop godoc
//
// @Summary      Keluar dari sebuah kelas
// @Description  Mengurangi satu kursi dan menulis satu event enrollment.dropped ke outbox.
// @Tags         courses
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  int  true  "ID kelas"
// @Success      204  "Kursi dilepas"
// @Failure      401  {object}  model.ErrorResponse  "Belum terautentikasi"
// @Failure      404  {object}  model.ErrorResponse  "Tidak terdaftar pada kelas ini"
// @Router       /courses/{id}/enroll [delete]
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
		course, err := repo.DecrementEnrolled(ctx, courseID)
		if err != nil {
			return err
		}
		return s.outbox.WithTx(tx).Insert(ctx, model.EventEnrollmentDropped, model.EnrollmentEvent{
			UserID: current.UserID, CourseID: courseID,
			CourseCode: course.Code, CourseName: course.Name,
		})
	})
	if err != nil {
		return translateCourseError(err)
	}

	return helper.NoContent(c)
}

// Swap memindahkan mahasiswa dari satu kelas ke kelas lain sebagai satu
// tindakan: bila kelas tujuan penuh, mahasiswa tetap di kelas asalnya.
//
// LockStudent diambil lebih dulu, baru LockCourses yang selalu mengunci
// dalam urutan id menaik, sehingga dua mahasiswa yang bertukar arah tidak
// saling menunggu.
//
// Swap godoc
//
// @Summary      Pindah kelas
// @Description  Keluar dari satu kelas dan masuk ke kelas lain sebagai satu tindakan. Bila kelas tujuan penuh, tidak ada yang berubah.
// @Tags         courses
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body      model.SwapRequest  true  "Kelas asal dan kelas tujuan"
// @Success      200   {object}  model.WebResponse{data=model.Course}
// @Failure      400   {object}  model.ErrorResponse  "Body bukan JSON yang valid"
// @Failure      401   {object}  model.ErrorResponse  "Belum terautentikasi"
// @Failure      409   {object}  model.ErrorResponse  "COURSE_FULL pada kelas tujuan"
// @Router       /enrollments/swap [post]
func (s *CourseService) Swap(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	current, ok := helper.CurrentUser(c)
	if !ok {
		return helper.Unauthorized("belum terautentikasi")
	}

	var req model.SwapRequest
	if err := c.BodyParser(&req); err != nil {
		return helper.BadRequest("body harus berupa JSON yang valid")
	}
	if errs := helper.ValidateStruct(req); errs != nil {
		return helper.Validation(errs)
	}
	if req.FromCourseID == req.ToCourseID {
		return helper.Validation(map[string]string{
			"to_course_id": "tidak boleh sama dengan from_course_id",
		})
	}

	var joined model.Course
	err := s.tx.WithTx(ctx, func(tx repository.DBTX) error {
		repo := s.courses.WithTx(tx)

		if err := repo.LockStudent(ctx, current.UserID); err != nil {
			return err
		}

		if err := repo.LockCourses(ctx, req.FromCourseID, req.ToCourseID); err != nil {
			return err
		}

		from, err := repo.FindByID(ctx, req.FromCourseID)
		if err != nil {
			return err
		}
		to, err := repo.FindByID(ctx, req.ToCourseID)
		if err != nil {
			return err
		}
		taken, err := repo.SumCredits(ctx, current.UserID)
		if err != nil {
			return err
		}
		if CreditsAfterSwap(taken, from.Credits, to.Credits) > MaxCreditsPerSemester {
			return errCreditLimit
		}

		if err := repo.DeleteEnrollment(ctx, req.FromCourseID, current.UserID); err != nil {
			return err
		}
		// repo, bukan s.courses: semua baca dan tulis harus memakai koneksi
		// transaction yang sama supaya atomik dan tidak mengunci diri sendiri.
		if _, err := repo.DecrementEnrolled(ctx, req.FromCourseID); err != nil {
			return err
		}

		joined, err = repo.IncrementEnrolled(ctx, req.ToCourseID)
		if err != nil {
			return err
		}
		if err := repo.InsertEnrollment(ctx, req.ToCourseID, current.UserID); err != nil {
			return err
		}

		// Satu event "pindah", bukan dua event "keluar" dan "masuk".
		// Mahasiswa melakukan satu tindakan, maka ia menerima satu notifikasi.
		return s.outbox.WithTx(tx).Insert(ctx, model.EventEnrollmentSwapped, model.EnrollmentEvent{
			UserID: current.UserID, CourseID: to.ID,
			CourseCode: to.Code, CourseName: to.Name,
			FromCourseCode: from.Code, FromCourseName: from.Name,
		})
	})
	if err != nil {
		return translateCourseError(err)
	}

	return helper.Success(c, fiber.StatusOK, "berhasil pindah kelas", joined)
}

// UpdateQuota godoc
//
// @Summary      Mengubah kuota kelas (admin)
// @Description  Optimistic locking: kirim version yang terakhir dilihat. Membutuhkan permission course:manage.
// @Tags         courses
// @Accept       json
// @Produce      json
// @Param        id    path      int                       true  "ID kelas"
// @Param        body  body      model.UpdateQuotaRequest  true  "Kuota baru dan version"
// @Success      200   {object}  model.WebResponse{data=model.Course}
// @Failure      400   {object}  model.ErrorResponse  "Body bukan JSON yang valid"
// @Failure      403   {object}  model.ErrorResponse  "Tidak memiliki hak course:manage"
// @Failure      404   {object}  model.ErrorResponse  "Kelas tidak ditemukan"
// @Failure      409   {object}  model.ErrorResponse  "VERSION_CONFLICT atau QUOTA_BELOW_ENROLLED"
// @Failure      422   {object}  model.ErrorResponse  "Validasi gagal"
// @Security     BearerAuth
// @Router       /courses/{id} [patch]
//
// UpdateQuota mengubah kuota kelas lalu memberi tahu seluruh peserta.
//
// Dua pernyataan sekarang: UPDATE courses dan INSERT outbox. Keduanya
// harus berhasil bersama, jadi keduanya berada dalam SATU transaction.
// Bila INSERT outbox gagal, kuota juga dibatalkan, sehingga tidak pernah
// ada kelas yang kuotanya berubah tanpa memberitahu peserta.
func (s *CourseService) UpdateQuota(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	id, valid := helper.ParamID(c)
	if !valid {
		return helper.BadRequest("id harus berupa angka positif")
	}

	var req model.UpdateQuotaRequest
	if err := c.BodyParser(&req); err != nil {
		return helper.BadRequest("body harus berupa JSON yang valid")
	}
	if errs := helper.ValidateStruct(req); errs != nil {
		return helper.Validation(errs)
	}

	var updated model.Course
	err := s.tx.WithTx(ctx, func(tx repository.DBTX) error {
		repo := s.courses.WithTx(tx)

		course, err := repo.UpdateQuota(ctx, id, req.Quota, req.Version)
		if err != nil {
			return err
		}

		// Peserta diambil DALAM transaction yang sama, supaya daftar
		// penerima sama dengan keadaan kelas saat kuota berubah.
		participants, err := repo.ListParticipantIDs(ctx, id)
		if err != nil {
			return err
		}

		if err := s.outbox.WithTx(tx).Insert(ctx, model.EventCourseQuotaChanged, model.EnrollmentEvent{
			CourseID: course.ID, CourseCode: course.Code, CourseName: course.Name,
			NewQuota: course.Quota, UserIDs: participants,
		}); err != nil {
			return err
		}

		updated = course
		return nil
	})
	if err != nil {
		return translateCourseError(err)
	}

	return helper.Success(c, fiber.StatusOK, "kuota berhasil diubah", updated)
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
	case errors.Is(err, repository.ErrVersionConflict):
		return helper.NewError(fiber.StatusConflict, "VERSION_CONFLICT",
			"data kelas sudah diubah pihak lain, muat ulang lalu coba lagi")
	case errors.Is(err, repository.ErrQuotaBelowFilled):
		return helper.NewError(fiber.StatusConflict, "QUOTA_BELOW_ENROLLED",
			"kuota tidak boleh lebih kecil dari jumlah peserta saat ini")
	default:
		return helper.Internal(err)
	}
}
