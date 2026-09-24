package service

import (
	"api-students/app/model"
	"testing"
)

func TestValidateRegister_Valid(t *testing.T) {
	req := model.RegisterRequest{Username: "sari_123", Email: "sari@example.com", Password: "rahasia123"}
	if errs := ValidateRegister(req); len(errs) != 0 {
		t.Fatalf("expected no errors, got %v", errs)
	}
}

func TestValidateRegister_InvalidUsername(t *testing.T) {
	cases := []struct {
		username string
		wantKey  string
	}{
		{"", "username"},
		{"ab", "username"},        // <3
		{"sari@hack", "username"}, // illegal char @
	}
	for _, tc := range cases {
		errs := ValidateRegister(model.RegisterRequest{Username: tc.username, Email: "a@b.com", Password: "rahasia123"})
		if errs[tc.wantKey] == "" {
			t.Errorf("username %q should error on %s, got %v", tc.username, tc.wantKey, errs)
		}
	}
}

func TestValidateRegister_InvalidEmail(t *testing.T) {
	errs := ValidateRegister(model.RegisterRequest{Username: "sari", Email: "not-an-email", Password: "rahasia123"})
	if errs["email"] == "" {
		t.Error("expected email error")
	}
}

func TestValidateRegister_WeakPassword(t *testing.T) {
	cases := []string{"short1", "password1", "12345678", "qwerty123", "hanyaHuruf", "123456789"}
	for _, pw := range cases {
		errs := ValidateRegister(model.RegisterRequest{Username: "sari", Email: "sari@example.com", Password: pw})
		if errs["password"] == "" {
			t.Errorf("password %q should be rejected", pw)
		}
	}
}

func TestValidateRegister_StrongPasswordPasses(t *testing.T) {
	// must have letter+digit, >=8, not in weak list
	errs := ValidateRegister(model.RegisterRequest{Username: "budi", Email: "budi@example.com", Password: "Kuat1234"})
	if errs["password"] != "" {
		t.Errorf("strong password should pass, got %v", errs)
	}
}

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

func TestCheckPasswordStrength_Direct(t *testing.T) {
	if msg := checkPasswordStrength("abc12345"); msg != "" {
		t.Errorf("abc12345 should be strong, got %q", msg)
	}
	if msg := checkPasswordStrength("short1"); msg == "" {
		t.Error("short1 should fail min length")
	}
	if msg := checkPasswordStrength("password1"); msg == "" {
		t.Error("password1 is in weak list, should fail")
	}
	if msg := checkPasswordStrength("ABCDEFGH"); msg == "" {
		t.Error("no digit should fail")
	}
}
