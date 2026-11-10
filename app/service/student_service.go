package service

import (
	"errors"
	"strconv"
	"strings"

	"api-students/app/model"
	"api-students/app/repository"
	"api-students/helper"

	"github.com/gofiber/fiber/v2"
)

type StudentService struct {
	repo  repository.StudentRepository
	perms *helper.PermissionSet
}

func NewStudentService(repo repository.StudentRepository, perms *helper.PermissionSet) *StudentService {
	return &StudentService{repo: repo, perms: perms}
}

// List godoc
//
// @Summary      Daftar mahasiswa
// @Description  Mendukung paginasi, pencarian, pengurutan, dan penyaringan.
// @Tags         students
// @Produce      json
// @Security     BearerAuth
// @Param        page      query  int     false  "Nomor halaman"      default(1)
// @Param        limit     query  int     false  "Jumlah data per halaman"  default(10)
// @Param        search    query  string  false  "Cari nama atau NIM"
// @Param        sort      query  string  false  "Kolom pengurutan"  default(id)
// @Param        order     query  string  false  "Arah pengurutan"  Enums(asc, desc)
// @Param        is_active query  bool    false  "Saring status aktif"
// @Param        min_grade query  number  false  "Nilai minimum"
// @Param        max_grade query  number  false  "Nilai maksimum"
// @Success      200       {object}  model.WebResponse{data=[]model.Student}
// @Failure      401       {object}  model.ErrorResponse  "Belum terautentikasi"
// @Failure      403       {object}  model.ErrorResponse  "Tidak memiliki hak student:list"
// @Router       /students [get]
func (s *StudentService) List(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()
	if _, ok := helper.CurrentUser(c); !ok {
		return helper.Unauthorized("belum terautentikasi")
	}
	q := helper.ParseListQuery(c)
	students, total, err := s.repo.FindAll(ctx, q)
	if err != nil {
		return helper.Internal(err)
	}
	return helper.SuccessList(c, "daftar mahasiswa berhasil diambil", students, &model.Meta{
		Page: q.Page, Limit: q.Limit, Total: total, TotalPages: CountTotalPages(total, q.Limit),
	})
}

// Get godoc
//
// @Summary      Detail satu mahasiswa
// @Tags         students
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  int  true  "ID mahasiswa"
// @Success      200  {object}  model.WebResponse{data=model.Student}
// @Failure      400  {object}  model.ErrorResponse  "ID bukan angka positif"
// @Failure      403  {object}  model.ErrorResponse  "Tidak berhak mengakses data mahasiswa lain"
// @Failure      404  {object}  model.ErrorResponse  "Mahasiswa tidak ditemukan"
// @Router       /students/{id} [get]
func (s *StudentService) Get(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()
	current, ok := helper.CurrentUser(c)
	if !ok {
		return helper.Unauthorized("belum terautentikasi")
	}
	id, valid := helper.ParamID(c)
	if !valid {
		return helper.BadRequest("id harus berupa angka positif")
	}
	student, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return translateError(err, "mahasiswa")
	}
	if !CanAccessStudent(current, student.OwnerID, s.perms, "student:read:any") {
		return helper.Forbidden("tidak berhak mengakses data mahasiswa lain")
	}
	return helper.Success(c, fiber.StatusOK, "mahasiswa ditemukan", student)
}

// Create godoc
//
// @Summary      Membuat mahasiswa baru
// @Tags         students
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body  body  model.CreateStudentRequest  true  "Data mahasiswa"
// @Success      201   {object}  model.WebResponse{data=model.Student}
// @Failure      400   {object}  model.ErrorResponse  "Body bukan JSON yang valid"
// @Failure      403   {object}  model.ErrorResponse  "Tidak memiliki hak student:create"
// @Failure      409   {object}  model.ErrorResponse  "NIM sudah dipakai"
// @Failure      422   {object}  model.ErrorResponse  "Validasi gagal"
// @Router       /students [post]
func (s *StudentService) Create(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()
	current, ok := helper.CurrentUser(c)
	if !ok {
		return helper.Unauthorized("belum terautentikasi")
	}
	var req model.CreateStudentRequest
	if err := c.BodyParser(&req); err != nil {
		return helper.BadRequest("body harus berupa JSON yang valid")
	}
	req.Name = strings.TrimSpace(req.Name)
	if errs := helper.ValidateStruct(req); len(errs) > 0 {
		return helper.Validation(errs)
	}
	ownerID := current.UserID
	baru, err := s.repo.Create(ctx, model.Student{
		NIM: req.NIM, Name: req.Name, Grade: req.Grade, IsActive: true, OwnerID: &ownerID,
	})
	if err != nil {
		return translateError(err, "mahasiswa")
	}
	return helper.Created(c, "mahasiswa berhasil dibuat", baru, "/api/v1/students/"+strconv.Itoa(baru.ID))
}

// Replace godoc
//
// @Summary      Mengganti data mahasiswa
// @Description  Semua field wajib diisi; field yang tidak dikirim menjadi kosong.
// @Tags         students
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id    path  int                        true  "ID mahasiswa"
// @Param        body  body  model.ReplaceStudentRequest  true  "Data mahasiswa baru"
// @Success      200   {object}  model.WebResponse{data=model.Student}
// @Failure      400   {object}  model.ErrorResponse  "Body bukan JSON yang valid"
// @Failure      403   {object}  model.ErrorResponse  "Tidak berhak mengubah data mahasiswa lain"
// @Failure      404   {object}  model.ErrorResponse  "Mahasiswa tidak ditemukan"
// @Failure      422   {object}  model.ErrorResponse  "Validasi gagal"
// @Router       /students/{id} [put]
func (s *StudentService) Replace(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()
	current, ok := helper.CurrentUser(c)
	if !ok {
		return helper.Unauthorized("belum terautentikasi")
	}
	id, valid := helper.ParamID(c)
	if !valid {
		return helper.BadRequest("id harus berupa angka positif")
	}
	var req model.ReplaceStudentRequest
	if err := c.BodyParser(&req); err != nil {
		return helper.BadRequest("body harus berupa JSON yang valid")
	}
	if errs := helper.ValidateStruct(req); len(errs) > 0 {
		return helper.Validation(errs)
	}
	saatIni, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return translateError(err, "mahasiswa")
	}
	if !CanAccessStudent(current, saatIni.OwnerID, s.perms, "student:update:any") {
		return helper.Forbidden("tidak berhak mengubah data mahasiswa lain")
	}
	hasil, err := s.repo.Update(ctx, model.Student{
		ID: id, Name: strings.TrimSpace(req.Name), NIM: req.NIM, Grade: *req.Grade, IsActive: *req.IsActive,
	})
	if err != nil {
		return translateError(err, "mahasiswa")
	}
	return helper.Success(c, fiber.StatusOK, "mahasiswa berhasil diganti seluruhnya", hasil)
}

// Patch godoc
//
// @Summary      Mengubah sebagian data mahasiswa
// @Description  Hanya field yang dikirim yang diubah.
// @Tags         students
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id    path  int                       true  "ID mahasiswa"
// @Param        body  body  model.PatchStudentRequest  true  "Field yang diubah"
// @Success      200   {object}  model.WebResponse{data=model.Student}
// @Failure      400   {object}  model.ErrorResponse  "Body bukan JSON yang valid atau tidak ada field yang diubah"
// @Failure      403   {object}  model.ErrorResponse  "Tidak berhak mengubah data mahasiswa lain"
// @Failure      404   {object}  model.ErrorResponse  "Mahasiswa tidak ditemukan"
// @Failure      422   {object}  model.ErrorResponse  "Validasi gagal"
// @Router       /students/{id} [patch]
func (s *StudentService) Patch(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()
	current, ok := helper.CurrentUser(c)
	if !ok {
		return helper.Unauthorized("belum terautentikasi")
	}
	id, valid := helper.ParamID(c)
	if !valid {
		return helper.BadRequest("id harus berupa angka positif")
	}
	var req model.PatchStudentRequest
	if err := c.BodyParser(&req); err != nil {
		return helper.BadRequest("body harus berupa JSON yang valid")
	}
	if IsEmptyPatch(req) {
		return helper.BadRequest("tidak ada field yang diubah")
	}
	saatIni, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return translateError(err, "mahasiswa")
	}
	if !CanAccessStudent(current, saatIni.OwnerID, s.perms, "student:update:any") {
		return helper.Forbidden("tidak berhak mengubah data mahasiswa lain")
	}
	if errs := helper.ValidateStruct(req); errs != nil {
		return helper.Validation(errs)
	}
	hasil, err := s.repo.Update(ctx, ApplyPatch(saatIni, req))
	if err != nil {
		return translateError(err, "mahasiswa")
	}
	return helper.Success(c, fiber.StatusOK, "mahasiswa berhasil diperbarui sebagian", hasil)
}

// Delete godoc
//
// @Summary      Menghapus mahasiswa
// @Tags         students
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  int  true  "ID mahasiswa"
// @Success      204  "Mahasiswa dihapus"
// @Failure      400  {object}  model.ErrorResponse  "ID bukan angka positif"
// @Failure      401  {object}  model.ErrorResponse  "Belum terautentikasi"
// @Failure      403  {object}  model.ErrorResponse  "Tidak memiliki hak student:delete"
// @Failure      404  {object}  model.ErrorResponse  "Mahasiswa tidak ditemukan"
// @Router       /students/{id} [delete]
func (s *StudentService) Delete(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()
	current, ok := helper.CurrentUser(c)
	if !ok {
		return helper.Unauthorized("belum terautentikasi")
	}
	if !s.perms.Can(current.Role, "student:delete") {
		return helper.Forbidden("role " + current.Role + " tidak memiliki hak")
	}
	id, valid := helper.ParamID(c)
	if !valid {
		return helper.BadRequest("id harus berupa angka positif")
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return translateError(err, "mahasiswa")
	}
	return helper.NoContent(c)
}

func translateError(err error, entity string) error {
	switch {
	case errors.Is(err, repository.ErrNotFound):
		return helper.NotFound(entity + " tidak ditemukan")
	case errors.Is(err, repository.ErrDuplicate):
		return helper.Conflict("nim sudah dipakai")
	default:
		return helper.Internal(err)
	}
}
