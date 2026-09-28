package helper

import (
	"fmt"

	"github.com/gofiber/fiber/v2"
)

// Stable machine-readable codes (contract, never change Message).
const (
	CodeValidation         = "VALIDATION_ERROR"
	CodeBadRequest         = "BAD_REQUEST"
	CodeUnauthorized       = "UNAUTHORIZED"
	CodeForbidden          = "FORBIDDEN"
	CodeNotFound           = "NOT_FOUND"
	CodeConflict           = "CONFLICT"
	CodeUnsupportedMedia   = "UNSUPPORTED_MEDIA_TYPE"
	CodeNotAcceptable      = "NOT_ACCEPTABLE"
	CodeTooManyRequests    = "TOO_MANY_REQUESTS"
	CodeServiceUnavailable = "SERVICE_UNAVAILABLE"
	CodeInternal           = "INTERNAL_ERROR"
)

// AppError is the only error type in the app. Does NOT touch fiber.Ctx.
type AppError struct {
	Status  int               // HTTP status
	Code    string            // stable code
	Message string            // human message
	Fields  map[string]string // per-field validation details
	cause   error             // original error, log-only, unexported
}

func (e *AppError) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}
func (e *AppError) Unwrap() error { return e.cause }

func BadRequest(m string) *AppError {
	return &AppError{Status: fiber.StatusBadRequest, Code: CodeBadRequest, Message: m}
}
func Unauthorized(m string) *AppError {
	return &AppError{Status: fiber.StatusUnauthorized, Code: CodeUnauthorized, Message: m}
}
func Forbidden(m string) *AppError {
	return &AppError{Status: fiber.StatusForbidden, Code: CodeForbidden, Message: m}
}
func NotFound(m string) *AppError {
	return &AppError{Status: fiber.StatusNotFound, Code: CodeNotFound, Message: m}
}
func Conflict(m string) *AppError {
	return &AppError{Status: fiber.StatusConflict, Code: CodeConflict, Message: m}
}
func Validation(fields map[string]string) *AppError {
	return &AppError{Status: fiber.StatusUnprocessableEntity, Code: CodeValidation, Message: "validasi gagal", Fields: fields}
}
func NotAcceptable(m string) *AppError {
	return &AppError{Status: fiber.StatusNotAcceptable, Code: CodeNotAcceptable, Message: m}
}
func UnsupportedMediaType(m string) *AppError {
	return &AppError{Status: fiber.StatusUnsupportedMediaType, Code: CodeUnsupportedMedia, Message: m}
}
func TooManyRequests(m string) *AppError {
	return &AppError{Status: fiber.StatusTooManyRequests, Code: CodeTooManyRequests, Message: m}
}
func ServiceUnavailable(cause error) *AppError {
	return &AppError{Status: fiber.StatusServiceUnavailable, Code: CodeServiceUnavailable, Message: "layanan sementara tidak tersedia", cause: cause}
}
func Internal(cause error) *AppError {
	return &AppError{Status: fiber.StatusInternalServerError, Code: CodeInternal, Message: "terjadi kesalahan pada server", cause: cause}
}
