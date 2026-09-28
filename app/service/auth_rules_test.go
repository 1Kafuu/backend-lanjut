package service

import (
	"api-students/app/model"
	"testing"
)

func TestValidateLogin_Required(t *testing.T) {
	errs := ValidateLogin(model.LoginRequest{Username: "", Password: ""})
	if errs["username"] == "" || errs["password"] == "" {
		t.Errorf("both fields required, got %v", errs)
	}
	// whitespace username should be treated as empty
	errs = ValidateLogin(model.LoginRequest{Username: "   ", Password: "x"})
	if errs["username"] == "" {
		t.Error("whitespace username should be required")
	}
}
