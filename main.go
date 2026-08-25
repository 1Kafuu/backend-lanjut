package main

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/requestid"
)

var metodeBerbody = map[string]bool {
	fiber.MethodPost: true,
	fiber.MethodPut: true,
	fiber.MethodPatch: true,
}

// requireJSON menolak request berisi body yang content-type-nya bukan JSON.
// Status 415 - Unsupported Media Type
func requireJSON(c *fiber.Ctx) error {
	if metodeBerbody[c.Method()] {
		ct  := c.Get("Content-Type")
		if !strings.HasPrefix(ct, fiber.MIMEApplicationJSON) {
			return fail(c, fiber.StatusUnsupportedMediaType, "Content-Type harus application/json")
		}
	}

	return c.Next()
}

func main() {
	app := fiber.New(fiber.Config{
		AppName: "API Students - Praktikum Backend Lanjut Minggu2",
		ErrorHandler: func (c *fiber.Ctx, err error) error {
			status := fiber.StatusInternalServerError
			pesan := "terjadi kesalahan server"
			if e, ok := err.(*fiber.Error); ok {
				status = e.Code
				pesan = e.Message
			}
			return fail(c, status, pesan)
		},
	})

	// Middleware Global
	app.Use(requestid.New())
	app.Use(logger.New(logger.Config{
		Format: "[${time}] $locals:requestid ${method} ${path} ${status} ${latency} \n", 
	}))
	app.Use(cors.New())

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Hello, World!")
	})

	api := app.Group("/api/v1")

	api.Get("/health", func (c *fiber.Ctx) error  {
		return ok(c, "server sudah berjalan", fiber.Map{"timestamp": time.Now()})
	}) 

	// requireJSON
	s := api.Group("/students", requireJSON)
	s.Get("/", listStudents)
	s.Get("/:id", getStudents)
	s.Post("/", createStudent)
	s.Put("/:id", replaceStudent)
	s.Patch("/:id", patchStudent)
	s.Delete("/:id", deleteStudent)

	// Endpoint tidak dikenal
	app.Use(func(c *fiber.Ctx) error {
		return fail(c, fiber.StatusNotFound, "endpoint tidak ditemukan")
	})

	fmt.Println("Server berjalan pada http://localhost:3000")
	log.Fatal(app.Listen(":3000"))
}