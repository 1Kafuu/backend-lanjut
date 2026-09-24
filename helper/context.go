package helper

import (
	"api-students/app/model"

	"github.com/gofiber/fiber/v2"
)

// LocalsAuthUser is the Fiber Locals key where RequireAuth stores identity.
// Constant prevents typo between middleware and service.
const LocalsAuthUser = "authUser"

func CurrentUser(c *fiber.Ctx) (model.AuthUser, bool) {
	user, ok := c.Locals(LocalsAuthUser).(model.AuthUser)
	return user, ok
}
