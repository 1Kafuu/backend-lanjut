package middleware

import (
	"api-students/helper"

	"github.com/gofiber/fiber/v2"
)

func RequirePermission(perms *helper.PermissionSet, permission string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		user, ok := helper.CurrentUser(c)
		if !ok {
			return helper.Fail(c, fiber.StatusUnauthorized, "belum terautentikasi")
		}
		if !perms.Can(user.Role, permission) {
			return helper.Fail(c, fiber.StatusForbidden, "role "+user.Role+" tidak memiliki hak "+permission)
		}
		return c.Next()
	}
}

func RequireRole(roles ...string) fiber.Handler {
	allowed := make(map[string]struct{}, len(roles))
	for _, r := range roles {
		allowed[r] = struct{}{}
	}
	return func(c *fiber.Ctx) error {
		user, ok := helper.CurrentUser(c)
		if !ok {
			return helper.Fail(c, fiber.StatusUnauthorized, "belum terautentikasi")
		}
		if _, ok := allowed[user.Role]; !ok {
			return helper.Fail(c, fiber.StatusForbidden, "role Anda tidak berhak mengakses endpoint ini")
		}
		return c.Next()
	}
}
