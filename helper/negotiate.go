package helper

import (
	"encoding/csv"
	"strconv"
	"strings"

	"api-students/app/model"

	"github.com/gofiber/fiber/v2"
)

const (
	FormatJSON = fiber.MIMEApplicationJSON
	FormatCSV  = "text/csv"
)

func Negotiate(c *fiber.Ctx, offered ...string) (string, error) {
	accept := strings.TrimSpace(c.Get(fiber.HeaderAccept))
	if accept == "" || accept == "*/*" {
		return offered[0], nil
	}
	chosen := c.Accepts(offered...)
	if chosen == "" {
		return "", NotAcceptable("format yang diminta tidak tersedia, pilih salah satu dari: " + strings.Join(offered, ", "))
	}
	return chosen, nil
}

func WriteUsersCSV(c *fiber.Ctx, users []model.User) error {
	c.Set(fiber.HeaderContentType, FormatCSV+"; charset=utf-8")
	c.Set(fiber.HeaderContentDisposition, `attachment; filename="users.csv"`)
	var buf strings.Builder
	w := csv.NewWriter(&buf)
	header := []string{"id", "username", "email", "role", "is_active", "created_at"}
	if err := w.Write(header); err != nil {
		return Internal(err)
	}
	for _, u := range users {
		row := []string{
			strconv.Itoa(u.ID), u.Username, u.Email, u.Role,
			strconv.FormatBool(u.IsActive),
			u.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		}
		if err := w.Write(row); err != nil {
			return Internal(err)
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return Internal(err)
	}
	return c.SendString(buf.String())
}
