package helper

import (
	"github.com/gofiber/fiber/v2"
	"api-students/app/model"
)

// LocalsAuthUser is the Fiber Locals key where RequireAuth stores identity.
// Constant prevents typo between middleware and service.
const LocalsAuthUser = "authUser"

func CurrentUser(c *fiber.Ctx) (model.AuthUser, bool) {
	user, ok := c.Locals(LocalsAuthUser).(model.AuthUser)
	return user, ok
}