package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"api-students/app/model"
	"api-students/app/repository"
	"api-students/helper"

	"github.com/gofiber/fiber/v2"
)

const refreshTokenBytes = 32 // 32 bytes → 64 hex chars

type AuthService struct {
	users      repository.UserRepository
	tokens     repository.TokenRepository
	jwt        *helper.JWTManager
	perms      *helper.PermissionSet
	refreshTTL time.Duration
}

func NewAuthService(
	users repository.UserRepository,
	tokens repository.TokenRepository,
	jwtManager *helper.JWTManager,
	perms *helper.PermissionSet,
	refreshTTL time.Duration,
) *AuthService {
	return &AuthService{users: users, tokens: tokens, jwt: jwtManager, perms: perms, refreshTTL: refreshTTL}
}

// Register godoc
//
// @Summary      Mendaftarkan akun baru
// @Description  Role akun baru selalu "user". Password minimal 8 karakter dan memuat huruf serta angka.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body  body      model.RegisterRequest  true  "Data akun baru"
// @Success      201   {object}  model.WebResponse{data=model.User}
// @Failure      400   {object}  model.ErrorResponse  "Body bukan JSON yang valid"
// @Failure      409   {object}  model.ErrorResponse  "Username sudah dipakai"
// @Failure      422   {object}  model.ErrorResponse  "Validasi gagal; detail per field ada di fields"
// @Router       /auth/register [post]
func (s *AuthService) Register(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()
	var req model.RegisterRequest
	if err := c.BodyParser(&req); err != nil {
		return helper.BadRequest("body harus berupa JSON yang valid")
	}
	req.Username = strings.TrimSpace(req.Username)
	req.Email = strings.TrimSpace(req.Email)
	if errs := helper.ValidateStruct(req); len(errs) > 0 {
		return helper.Validation(errs)
	}
	hashed, err := helper.HashPassword(req.Password)
	if err != nil {
		return helper.Internal(fmt.Errorf("gagal memproses password: %w", err))
	}
	created, err := s.users.Create(ctx, model.User{
		Username: req.Username,
		Email:    req.Email,
		Password: hashed,
		Role:     "user", // server-determined, never from request (mass assignment guard)
		IsActive: true,
	})
	if err != nil {
		if errors.Is(err, repository.ErrDuplicate) {
			return helper.Conflict("username sudah dipakai")
		}
		return helper.Internal(err)
	}
	return helper.Created(c, "pendaftaran berhasil", created, "/api/v1/users/"+strconv.Itoa(created.ID))
}

// Login godoc
//
// @Summary      Login dan mendapatkan pasangan token
// @Description  Dibatasi 5 kali gagal per menit per IP.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body  body      model.LoginRequest  true  "Username dan password"
// @Success      200   {object}  model.WebResponse{data=model.TokenPair}
// @Failure      401   {object}  model.ErrorResponse  "Username atau password salah"
// @Failure      429   {object}  model.ErrorResponse  "Terlalu banyak percobaan login"
// @Router       /auth/login [post]
func (s *AuthService) Login(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()
	var req model.LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return helper.BadRequest("body harus berupa JSON yang valid")
	}
	if errs := ValidateLogin(req); len(errs) > 0 {
		return helper.Validation(errs)
	}
	user, err := s.users.FindByUsername(ctx, strings.TrimSpace(req.Username))
	if err != nil {
		helper.VerifyDummyPassword(req.Password) // timing-attack mitigation (user enumeration guard)
		return helper.Unauthorized("username atau password salah")
	}
	if !helper.VerifyPassword(user.Password, req.Password) {
		return helper.Unauthorized("username atau password salah")
	}
	if !user.IsActive {
		return helper.Forbidden("akun dinonaktifkan")
	}
	pair, err := s.issueTokenPair(ctx, user)
	if err != nil {
		return helper.Internal(fmt.Errorf("gagal membuat token: %w", err))
	}
	return helper.Success(c, fiber.StatusOK, "login berhasil", pair)
}

// Refresh godoc
//
// @Summary      Memperbarui access token
// @Description  Refresh token dirotasi: token lama langsung dicabut saat yang baru terbit.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body  body      model.RefreshRequest  true  "Refresh token lama"
// @Success      200   {object}  model.WebResponse{data=model.TokenPair}
// @Failure      401   {object}  model.ErrorResponse  "Refresh token tidak valid atau kedaluwarsa"
// @Router       /auth/refresh [post]
func (s *AuthService) Refresh(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()
	var req model.RefreshRequest
	if err := c.BodyParser(&req); err != nil {
		return helper.BadRequest("body harus berupa JSON yang valid")
	}
	if strings.TrimSpace(req.RefreshToken) == "" {
		return helper.BadRequest("refresh_token wajib diisi")
	}
	hash := helper.SHA256Hex(req.RefreshToken)
	stored, err := s.tokens.FindActive(ctx, hash)
	if err != nil {
		return helper.Unauthorized("refresh token tidak valid atau sudah kedaluwarsa")
	}
	user, err := s.users.FindByID(ctx, stored.UserID)
	if err != nil || !user.IsActive {
		return helper.Unauthorized("akun tidak dapat dipakai")
	}
	// Rotation: revoke old immediately
	if err := s.tokens.Revoke(ctx, hash); err != nil {
		return helper.Internal(fmt.Errorf("gagal memperbarui token: %w", err))
	}
	pair, err := s.issueTokenPair(ctx, user)
	if err != nil {
		return helper.Internal(fmt.Errorf("gagal membuat token: %w", err))
	}
	return helper.Success(c, fiber.StatusOK, "token berhasil diperbarui", pair)
}

// Logout godoc
//
// @Summary      Keluar dan mencabut refresh token
// @Description  Idempoten: mencabut refresh token yang tidak ada tetap dianggap berhasil.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body  body      model.RefreshRequest  false  "Refresh token yang dicabut"
// @Success      200   {object}  model.WebResponse
// @Failure      400   {object}  model.ErrorResponse  "Body bukan JSON yang valid"
// @Router       /auth/logout [post]
func (s *AuthService) Logout(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()
	var req model.RefreshRequest
	if err := c.BodyParser(&req); err != nil {
		return helper.BadRequest("body harus berupa JSON yang valid")
	}
	if strings.TrimSpace(req.RefreshToken) != "" {
		_ = s.tokens.Revoke(ctx, helper.SHA256Hex(req.RefreshToken)) // idempotent — never error to client
	}
	return helper.Success(c, fiber.StatusOK, "logout berhasil", nil)
}

// Me godoc
//
// @Summary      Profil saya
// @Description  Data akun dan daftar permission sesuai role.
// @Tags         auth
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  model.WebResponse{data=model.MyProfile}
// @Failure      401  {object}  model.ErrorResponse  "Belum terautentikasi"
// @Router       /auth/me [get]
func (s *AuthService) Me(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()
	authUser, ok := helper.CurrentUser(c)
	if !ok {
		return helper.Unauthorized("belum terautentikasi")
	}
	user, err := s.users.FindByID(ctx, authUser.UserID)
	if err != nil {
		return helper.Unauthorized("user tidak ditemukan")
	}
	user.Password = ""
	return helper.Success(c, fiber.StatusOK, "profil berhasil diambil", model.MyProfile{
		User:        user,
		Permissions: s.perms.PermissionsOf(authUser.Role),
	})
}

func (s *AuthService) issueTokenPair(ctx context.Context, user model.User) (model.TokenPair, error) {
	accessToken, err := s.jwt.GenerateAccess(user)
	if err != nil {
		return model.TokenPair{}, err
	}
	refreshToken, err := helper.RandomToken(refreshTokenBytes)
	if err != nil {
		return model.TokenPair{}, err
	}
	err = s.tokens.Save(ctx, model.RefreshToken{
		UserID:    user.ID,
		TokenHash: helper.SHA256Hex(refreshToken),
		ExpiresAt: time.Now().Add(s.refreshTTL),
	})
	if err != nil {
		return model.TokenPair{}, err
	}
	return model.TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    int(s.jwt.AccessTTL().Seconds()),
	}, nil
}
