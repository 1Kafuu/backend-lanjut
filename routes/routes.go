package routes

import (
	"context"
	"time"

	"api-students/app/service"
	"api-students/helper"
	"api-students/middleware"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type Dependencies struct {
	Pool             *pgxpool.Pool
	Mongo            *mongo.Client
	JWT              *helper.JWTManager
	Permissions      *helper.PermissionSet
	StudentService   *service.StudentService
	UserService      *service.UserService
	PrestasiService  *service.PrestasiService
	AuthService      *service.AuthService
	CourseService    *service.CourseService
	ReviewService    *service.ReviewService
	PortfolioService *service.PortfolioService
}

func Register(app *fiber.App, deps Dependencies) {
	api := app.Group("/api/v1")

	// public
	api.Get("/health", healthCheck(deps.Pool, deps.Mongo))

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

	students.Get("/:id/portfolio", deps.PortfolioService.Get)
	students.Put("/:id/portfolio", deps.PortfolioService.Upsert)
	students.Post(
		"/:id/portfolio/projects",
		deps.PortfolioService.AddProject,
	)
	students.Delete(
		"/:id/portfolio/projects/:projectId",
		deps.PortfolioService.DeleteProject,
	)

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

	courses := api.Group("/courses",
		middleware.RequireJSON,
		middleware.RequireAuth(deps.JWT))

	courses.Get("/:id/reviews/summary", deps.ReviewService.Summary)
	courses.Get("/:id/reviews", deps.ReviewService.ListByCourse)
	courses.Post("/:id/reviews", deps.ReviewService.Create)

	courses.Get("/", deps.CourseService.List)
	courses.Get("/mine", deps.CourseService.Mine)
	courses.Get("/:id", deps.CourseService.Get)
	courses.Patch("/:id", middleware.RequirePermission(deps.Permissions, "course:manage"), deps.CourseService.UpdateQuota)
	courses.Post("/:id/enroll", deps.CourseService.Enroll)
	courses.Delete("/:id/enroll", deps.CourseService.Drop)

	reviews := api.Group("/reviews",
		middleware.RequireJSON,
		middleware.RequireAuth(deps.JWT))
	reviews.Get("/:id", deps.ReviewService.Get)
	reviews.Patch("/:id", deps.ReviewService.Update)
	reviews.Delete("/:id", deps.ReviewService.Delete)
	reviews.Post("/:id/replies", deps.ReviewService.Reply)
	reviews.Post("/:id/helpful", deps.ReviewService.MarkHelpful)

	portfolios := api.Group("/portfolios",
		middleware.RequireJSON,
		middleware.RequireAuth(deps.JWT))
	portfolios.Get(
		"/stats/technologies",
		deps.PortfolioService.TopTechnologies,
	)

	// pindah kelas
	enrollments := api.Group("/enrollments",
		middleware.RequireJSON,
		middleware.RequireAuth(deps.JWT))

	enrollments.Post("/swap", deps.CourseService.Swap)
}

func healthCheck(pool *pgxpool.Pool, mongoClient *mongo.Client) fiber.Handler {
	return func(c *fiber.Ctx) error {
		ctx, cancel := context.WithTimeout(c.UserContext(), 2*time.Second)
		defer cancel()

		if err := pool.Ping(ctx); err != nil {
			return helper.NewError(
				fiber.StatusServiceUnavailable,
				helper.CodeServiceUnavailable,
				"PostgreSQL tidak dapat dihubungi",
			)
		}
		if err := mongoClient.Ping(ctx, nil); err != nil {
			return helper.NewError(
				fiber.StatusServiceUnavailable,
				helper.CodeServiceUnavailable,
				"MongoDB tidak dapat dihubungi",
			)
		}

		return helper.Success(
			c,
			fiber.StatusOK,
			"server, PostgreSQL, dan MongoDB berjalan",
			nil,
		)
	}
}
