package helper

import (
	"encoding/base64"
	"errors"
	"strconv"
	"strings"
	"time"

	"api-students/app/model"

	"github.com/gofiber/fiber/v2"
)

var ErrInvalidCursor = errors.New("cursor tidak valid")

func EncodeCursor(createdAt time.Time, id int) string {
	raw := strconv.FormatInt(createdAt.UTC().UnixNano(), 10) + "|" + strconv.Itoa(id)
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func DecodeCursor(encoded string) (model.Cursor, error) {
	if strings.TrimSpace(encoded) == "" {
		return model.Cursor{}, ErrInvalidCursor
	}
	decoded, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return model.Cursor{}, ErrInvalidCursor
	}
	parts := strings.Split(string(decoded), "|")
	if len(parts) != 2 {
		return model.Cursor{}, ErrInvalidCursor
	}
	nanos, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return model.Cursor{}, ErrInvalidCursor
	}
	id, err := strconv.Atoi(parts[1])
	if err != nil || id < 1 {
		return model.Cursor{}, ErrInvalidCursor
	}
	return model.Cursor{CreatedAt: time.Unix(0, nanos).UTC(), ID: id}, nil
}

func ParseCursorQuery(c *fiber.Ctx) (model.CursorQuery, error) {
	limit := c.QueryInt("limit", 10)
	if limit < 1 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}
	q := model.CursorQuery{Limit: limit}
	q.Search = strings.TrimSpace(c.Query("search"))
	if raw := c.Query("is_active"); raw != "" {
		b, err := strconv.ParseBool(raw)
		if err != nil {
			return q, BadRequest("is_active harus berupa boolean")
		}
		q.IsActive = &b
	}
	if cur := c.Query("cursor"); cur != "" {
		after, err := DecodeCursor(cur)
		if err != nil {
			return q, BadRequest("cursor tidak valid")
		}
		q.After = &after
	}
	return q, nil
}
