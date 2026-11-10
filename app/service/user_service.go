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

// List godoc
//
// @Summary      Daftar user dengan paginasi cursor
// @Description  Mendukung ?search=, ?is_active=, ?limit=, dan ?after=.
// @Tags         users
// @Produce      json
// @Security     BearerAuth
// @Param        search    query  string  false  "Cari username atau email"
// @Param        is_active query  bool    false  "Saring status aktif"
// @Param        limit     query  int     false  "Jumlah data per halaman"
// @Param        after     query  string  false  "Cursor halaman sebelumnya"
// @Success      200       {object}  model.UserCursorPage
// @Failure      401       {object}  model.ErrorResponse  "Belum terautentikasi"
// @Failure      403       {object}  model.ErrorResponse  "Tidak memiliki hak user:list"
// @Router       /users [get]
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
	// Content negotiation BEFORE query (Langkah 8)
	format, err := helper.Negotiate(c, helper.FormatJSON, helper.FormatCSV)
	if err != nil {
		return err
	}
	q, err := helper.ParseCursorQuery(c)
	if err != nil {
		return err
	}
	rows, err := s.repo.FindAfterCursor(ctx, q)
	if err != nil {
		return helper.Internal(err)
	}
	hasMore := len(rows) > q.Limit
	if hasMore {
		rows = rows[:q.Limit]
	}
	meta := &model.CursorMeta{Limit: q.Limit, HasMore: hasMore}
	if hasMore && len(rows) > 0 {
		last := rows[len(rows)-1]
		meta.NextCursor = helper.EncodeCursor(last.CreatedAt, last.ID)
	}
	if format == helper.FormatCSV {
		return helper.WriteUsersCSV(c, rows)
	}
	return c.Status(fiber.StatusOK).JSON(model.UserCursorPage{
		Success: true,
		Message: "daftar user berhasil diambil",
		Data:    rows,
		Meta:    *meta,
	})
}

// Get godoc
//
// @Summary      Detail satu user
// @Tags         users
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  int  true  "ID user"
// @Success      200  {object}  model.WebResponse{data=model.User}
// @Failure      400  {object}  model.ErrorResponse  "ID bukan angka positif"
// @Failure      403  {object}  model.ErrorResponse  "Tidak berhak mengakses data user lain"
// @Failure      404  {object}  model.ErrorResponse  "User tidak ditemukan"
// @Router       /users/{id} [get]
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

// Replace godoc
//
// @Summary      Mengganti data user
// @Description  Semua field wajib diisi; field yang tidak dikirim menjadi kosong.
// @Tags         users
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id    path  int                       true  "ID user"
// @Param        body  body  model.ReplaceUserRequest  true  "Data user baru"
// @Success      200   {object}  model.WebResponse{data=model.User}
// @Failure      400   {object}  model.ErrorResponse  "Body bukan JSON yang valid"
// @Failure      403   {object}  model.ErrorResponse  "Tidak berhak mengubah user lain"
// @Failure      422   {object}  model.ErrorResponse  "Validasi gagal"
// @Router       /users/{id} [put]
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
	var req model.ReplaceUserRequest
	if err := c.BodyParser(&req); err != nil {
		return helper.BadRequest("body harus berupa JSON yang valid")
	}
	req.Username = strings.TrimSpace(req.Username)
	req.Email = strings.TrimSpace(req.Email)
	if errs := helper.ValidateStruct(req); errs != nil {
		return helper.Validation(errs)
	}
	updated, err := s.repo.Update(ctx, id, req.Username, req.Email)
	if err != nil {
		return translateError(err, "user")
	}
	updated.Password = ""
	return helper.Success(c, fiber.StatusOK, "user berhasil diperbarui", updated)
}

// Patch godoc
//
// @Summary      Mengubah sebagian data user
// @Description  Hanya field yang dikirim yang diubah.
// @Tags         users
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id    path  int                    true  "ID user"
// @Param        body  body  model.PatchUserRequest  true  "Field yang diubah"
// @Success      200   {object}  model.WebResponse{data=model.User}
// @Failure      400   {object}  model.ErrorResponse  "Body bukan JSON yang valid"
// @Failure      403   {object}  model.ErrorResponse  "Tidak berhak mengubah user lain"
// @Failure      422   {object}  model.ErrorResponse  "Validasi gagal"
// @Router       /users/{id} [patch]
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
	var req model.PatchUserRequest
	if err := c.BodyParser(&req); err != nil {
		return helper.BadRequest("body harus berupa JSON yang valid")
	}
	if IsEmptyUserPatch(req) {
		return helper.BadRequest("tidak ada field yang diubah")
	}
	existing, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return translateError(err, "user")
	}
	// Validasi dulu (tag omitnil), baru merge murni
	if errs := helper.ValidateStruct(req); errs != nil {
		return helper.Validation(errs)
	}
	merged := ApplyUserPatch(existing, req)
	updated, err := s.repo.Update(ctx, id, merged.Username, merged.Email)
	if err != nil {
		return translateError(err, "user")
	}
	updated.Password = ""
	return helper.Success(c, fiber.StatusOK, "user berhasil diperbarui", updated)
}

// Delete godoc
//
// @Summary      Menghapus user
// @Description  Soft delete: akun dinonaktifkan, bukan dihapus permanen.
// @Tags         users
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  int  true  "ID user"
// @Success      204  "User dinonaktifkan"
// @Failure      401  {object}  model.ErrorResponse  "Belum terautentikasi"
// @Failure      403  {object}  model.ErrorResponse  "Tidak berhak menghapus user lain"
// @Failure      404  {object}  model.ErrorResponse  "User tidak ditemukan"
// @Router       /users/{id} [delete]
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

// AssignRole godoc
//
// @Summary      Mengganti role user
// @Tags         users
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id    path  int                      true  "ID user"
// @Param        body  body  model.AssignRoleRequest  true  "Role baru"
// @Success      200   {object}  model.WebResponse{data=model.User}
// @Failure      400   {object}  model.ErrorResponse  "Body bukan JSON yang valid"
// @Failure      403   {object}  model.ErrorResponse  "Tidak memiliki hak role:assign"
// @Failure      404   {object}  model.ErrorResponse  "User tidak ditemukan"
// @Failure      422   {object}  model.ErrorResponse  "Role tidak dikenal"
// @Router       /users/{id}/role [patch]
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
