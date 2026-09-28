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
	updated, errs := ApplyPatch(saatIni, req)
	if len(errs) > 0 {
		return helper.Validation(errs)
	}
	hasil, err := s.repo.Update(ctx, updated)
	if err != nil {
		return translateError(err, "mahasiswa")
	}
	return helper.Success(c, fiber.StatusOK, "mahasiswa berhasil diperbarui sebagian", hasil)
}

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
