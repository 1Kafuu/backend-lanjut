package service

import (
	"strings"

	"api-students/app/model"
	"api-students/app/repository"
	"api-students/helper"

	"github.com/gofiber/fiber/v2"
)

type UserService struct {
	repo  repository.UserRepository
	perms *helper.PermissionSet
}

func NewUserService(repo repository.UserRepository, perms *helper.PermissionSet) *UserService {
	return &UserService{repo: repo, perms: perms}
}

func patchFieldErrors(username, email string) map[string]string {
	errs := map[string]string{}
	if username == "" {
		errs["username"] = "tidak boleh kosong"
	}
	if email == "" {
		errs["email"] = "tidak boleh kosong"
	}
	if len(errs) == 0 {
		return nil
	}
	return errs
}

func (s *UserService) List(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()
	current, ok := helper.CurrentUser(c)
	if !ok {
		return helper.Unauthorized("belum terautentikasi")
	}
	if !s.perms.Can(current.Role, "user:list") {
		return helper.Forbidden("role " + current.Role + " tidak memiliki hak user:list")
	}
	q := helper.ParseListQuery(c)
	users, total, err := s.repo.FindAll(ctx, q)
	if err != nil {
		return helper.Internal(err)
	}
	return helper.SuccessList(c, "daftar user berhasil diambil", users, &model.Meta{
		Page: q.Page, Limit: q.Limit, Total: total, TotalPages: CountTotalPages(total, q.Limit),
	})
}

func (s *UserService) Get(c *fiber.Ctx) error {
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
	if !CanAccessUser(current, id, s.perms, "user:read:any") {
		return helper.Forbidden("tidak berhak mengakses data user lain")
	}
	user, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return translateError(err, "user")
	}
	return helper.Success(c, fiber.StatusOK, "user ditemukan", user)
}

func (s *UserService) Replace(c *fiber.Ctx) error {
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
	if !CanAccessUser(current, id, s.perms, "user:update:any") {
		return helper.Forbidden("tidak berhak mengubah data user lain")
	}
	var req struct {
		Username string `json:"username"`
		Email    string `json:"email"`
	}
	if err := c.BodyParser(&req); err != nil {
		return helper.BadRequest("body harus berupa JSON yang valid")
	}
	req.Username = strings.TrimSpace(req.Username)
	req.Email = strings.TrimSpace(req.Email)
	if req.Username == "" || req.Email == "" {
		return helper.BadRequest("username dan email wajib diisi")
	}
	updated, err := s.repo.Update(ctx, id, req.Username, req.Email)
	if err != nil {
		return translateError(err, "user")
	}
	updated.Password = ""
	return helper.Success(c, fiber.StatusOK, "user berhasil diperbarui", updated)
}

func (s *UserService) Patch(c *fiber.Ctx) error {
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
	if !CanAccessUser(current, id, s.perms, "user:update:any") {
		return helper.Forbidden("tidak berhak mengubah data user lain")
	}
	var req map[string]string
	if err := c.BodyParser(&req); err != nil {
		return helper.BadRequest("body harus berupa JSON yang valid")
	}
	// ambil existing untuk patch
	existing, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return translateError(err, "user")
	}
	username := existing.Username
	email := existing.Email
	if v, ok := req["username"]; ok {
		username = strings.TrimSpace(v)
	}
	if v, ok := req["email"]; ok {
		email = strings.TrimSpace(v)
	}
	if errs := patchFieldErrors(username, email); errs != nil {
		return helper.Validation(errs)
	}
	updated, err := s.repo.Update(ctx, id, username, email)
	if err != nil {
		return translateError(err, "user")
	}
	updated.Password = ""
	return helper.Success(c, fiber.StatusOK, "user berhasil diperbarui", updated)
}

func (s *UserService) Delete(c *fiber.Ctx) error {
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
	if current.UserID == id {
		return helper.Forbidden("tidak boleh menghapus akun sendiri")
	}
	if !s.perms.Can(current.Role, "user:delete") {
		return helper.Forbidden("role " + current.Role + " tidak memiliki hak user:delete")
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return translateError(err, "user")
	}
	return helper.NoContent(c)
}

func (s *UserService) AssignRole(c *fiber.Ctx) error {
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
	if !s.perms.Can(current.Role, "role:assign") {
		return helper.Forbidden("role " + current.Role + " tidak memiliki hak")
	}
	var req model.AssignRoleRequest
	if err := c.BodyParser(&req); err != nil {
		return helper.BadRequest("body harus berupa JSON yang valid")
	}
	req.Role = strings.TrimSpace(req.Role)
	if errs := ValidateAssignRole(current, id, req, s.perms); len(errs) > 0 {
		return helper.Validation(errs)
	}
	if _, err := s.repo.FindByID(ctx, id); err != nil {
		return translateError(err, "user")
	}
	updated, err := s.repo.UpdateRole(ctx, id, req.Role)
	if err != nil {
		return translateError(err, "user")
	}
	updated.Password = ""
	return helper.Success(c, fiber.StatusOK, "role berhasil diubah", updated)
}
