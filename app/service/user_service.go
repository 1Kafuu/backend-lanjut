package service

import (
	"api-students/app/model"
	"api-students/app/repository"
	"api-students/helper"

	"github.com/gofiber/fiber/v2"
)

type UserService struct {
	repo repository.UserRepository
	perms *helper.PermissionSet
}

func NewUserService(repo repository.UserRepository, perms *helper.PermissionSet) *UserService {
    return &UserService{repo: repo, perms: perms}
}

func (s *UserService) Get(c *fiber.Ctx) error {
    current, ok := helper.CurrentUser(c)
    if !ok {
        return helper.Fail(c, fiber.StatusUnauthorized, "belum terautentikasi")
    }
    id, valid := helper.ParamID(c)
    if !valid {
        return helper.Fail(c, fiber.StatusBadRequest, "id harus berupa angka positif")
    }
    if !CanAccessUser(current, id, s.perms, "user:read:any") {
        return helper.Fail(c, fiber.StatusForbidden, "tidak berhak melihat user lain")
    }
    u, err := s.repo.FindByID(c.UserContext(), id)
    if err != nil {
        return helper.Fail(c, fiber.StatusNotFound, "user tidak ditemukan")
    }
    u.Password = ""
    return helper.Success(c, fiber.StatusOK, "user ditemukan", u)
}

func (s *UserService) AssignRole(c *fiber.Ctx) error {
    current, ok := helper.CurrentUser(c)
    if !ok {
        return helper.Fail(c, fiber.StatusUnauthorized, "belum terautentikasi")
    }

    id, valid := helper.ParamID(c)
    if !valid {
        return helper.Fail(c, fiber.StatusBadRequest, "id harus berupa angka positif")
    }
    if !s.perms.Can(current.Role, "role:assign") {
        return helper.Fail(c, fiber.StatusForbidden, "role "+current.Role+" tidak memiliki hak")
    }
    var req model.AssignRoleRequest
    if err := c.BodyParser(&req); err != nil {
        return helper.Fail(c, fiber.StatusBadRequest, "body harus berupa JSON yang valid")
    }
    if errs := ValidateAssignRole(current, id, req, s.perms); len(errs) > 0 {
        return helper.FailValidation(c, errs)
    }
    if _, err := s.repo.FindByID(c.UserContext(), id); err != nil {
        if err == repository.ErrNotFound {
            return helper.Fail(c, fiber.StatusNotFound, "user tidak ditemukan")
        }
        return helper.Fail(c,fiber.StatusInternalServerError, "gagal mengambil user")
    }
    updated, err := s.repo.UpdateRole(c.UserContext(), id, req.Role)
    if err != nil {
        return helper.Fail(c, fiber.StatusInternalServerError, "gagal mengubah role user")
    }
    updated.Password = ""
    return helper.Success(c, fiber.StatusOK, "role berhasil diubah", updated)
}
