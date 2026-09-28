package service

import (
	"api-students/app/model"
	"strings"
)

// ValidateLogin only checks presence, not strength (Modul §6: old passwords may predate new rules).
func ValidateLogin(req model.LoginRequest) map[string]string {
	errs := map[string]string{}
	if strings.TrimSpace(req.Username) == "" {
		errs["username"] = "wajib diisi"
	}
	if req.Password == "" {
		errs["password"] = "wajib diisi"
	}
	return errs
}
