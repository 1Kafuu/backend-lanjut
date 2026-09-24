package routes

import (
	"context"
	"time"

	"api-students/app/service"
	"api-students/helper"
	"api-students/middleware"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Dependencies struct {
	Pool            *pgxpool.Pool
	JWT             *helper.JWTManager
	Permissions     *helper.PermissionSet
	StudentService  *service.StudentService
	UserService     *service.UserService
	PrestasiService *service.PrestasiService
	AuthService     *service.AuthService
}

func Register(app *fiber.App, deps Dependencies) {
	api := app.Group("/api/v1")

	// public
	api.Get("/health", healthCheck(deps.Pool))

	// auth
	auth := api.Group("/auth", middleware.RequireJSON)
	auth.Post("/register", deps.AuthService.Register)
	auth.Post("/login", middleware.LoginRateLimiter(), deps.AuthService.Login)
	auth.Post("/refresh", deps.AuthService.Refresh)
	auth.Post("/logout", deps.AuthService.Logout)
	auth.Get("/me", middleware.RequireAuth(deps.JWT), deps.AuthService.Me)

	students := api.Group("/students", middleware.RequireJSON, middleware.RequireAuth(deps.JWT))
	students.Get("/", middleware.RequirePermission(deps.Permissions, "student:list"), deps.StudentService.List)
	students.Post("/", middleware.RequirePermission(deps.Permissions, "student:create"), deps.StudentService.Create)
	students.Delete("/:id", middleware.RequirePermission(deps.Permissions, "student:delete"), deps.StudentService.Delete)

	students.Get("/:id", deps.StudentService.Get)
	students.Put("/:id", deps.StudentService.Replace)
	students.Patch("/:id", deps.StudentService.Patch)

	prestasi := api.Group("/prestasi", middleware.RequireJSON, middleware.RequireAuth(deps.JWT))
	prestasi.Get("/", deps.PrestasiService.List)
	prestasi.Get("/:id", deps.PrestasiService.Get)

	users := api.Group("/users", middleware.RequireJSON, middleware.RequireAuth(deps.JWT))
	users.Get("/", middleware.RequirePermission(deps.Permissions, "user:list"), deps.UserService.List)
	users.Get("/:id", deps.UserService.Get)
	users.Put("/:id", deps.UserService.Replace)
	users.Patch("/:id", deps.UserService.Patch)
	users.Delete("/:id", deps.UserService.Delete)
	users.Patch("/:id/role", middleware.RequirePermission(deps.Permissions, "role:assign"), deps.UserService.AssignRole)
}

func healthCheck(pool *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		ctx, cancel := context.WithTimeout(c.UserContext(), 2*time.Second)
		defer cancel()
		if err := pool.Ping(ctx); err != nil {
			return helper.Fail(c, fiber.StatusServiceUnavailable, "database tidak dapat dihubungi")
		}
		return helper.Success(c, fiber.StatusOK, "server dan database berjalan", nil)
	}
}
