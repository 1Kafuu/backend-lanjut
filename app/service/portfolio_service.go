package service

import (
	"errors"
	"strings"

	"api-students/app/model"
	"api-students/app/repository"
	"api-students/helper"

	"github.com/gofiber/fiber/v2"
)

type PortfolioService struct {
	portfolios repository.PortfolioRepository
	students   repository.StudentRepository
	perms      *helper.PermissionSet
}

func NewPortfolioService(
	portfolios repository.PortfolioRepository,
	students repository.StudentRepository,
	perms *helper.PermissionSet,
) *PortfolioService {
	return &PortfolioService{
		portfolios: portfolios,
		students:   students,
		perms:      perms,
	}
}

func (s *PortfolioService) Get(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	student, err := s.findAuthorizedStudent(c, "student:read:any")
	if err != nil {
		return err
	}

	portfolio, err := s.portfolios.FindByStudentID(ctx, student.ID)
	if err != nil {
		return translatePortfolioError(err)
	}
	return helper.Success(c, fiber.StatusOK, "portofolio ditemukan", portfolio)
}

func (s *PortfolioService) Upsert(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	student, err := s.findAuthorizedStudent(c, "student:update:any")
	if err != nil {
		return err
	}

	var req model.UpsertPortfolioRequest
	if err := c.BodyParser(&req); err != nil {
		return helper.BadRequest("body harus berupa JSON yang valid")
	}
	normalizePortfolioRequest(&req)
	if errs := helper.ValidateStruct(req); errs != nil {
		return helper.Validation(errs)
	}

	portfolio := model.Portfolio{
		StudentID:    student.ID,
		Headline:     req.Headline,
		Bio:          req.Bio,
		Projects:     make([]model.Project, 0, len(req.Projects)),
		Certificates: make([]model.Certificate, 0, len(req.Certificates)),
	}
	for _, project := range req.Projects {
		portfolio.Projects = append(portfolio.Projects, model.Project{
			Title:        project.Title,
			Technologies: project.Technologies,
			Link:         project.Link,
			Year:         project.Year,
		})
	}
	for _, certificate := range req.Certificates {
		portfolio.Certificates = append(portfolio.Certificates, model.Certificate{
			Name:     certificate.Name,
			Issuer:   certificate.Issuer,
			IssuedAt: certificate.IssuedAt,
		})
	}

	saved, err := s.portfolios.Upsert(ctx, portfolio)
	if err != nil {
		return translatePortfolioError(err)
	}
	return helper.Success(c, fiber.StatusOK, "portofolio berhasil disimpan", saved)
}

func (s *PortfolioService) AddProject(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	student, err := s.findAuthorizedStudent(c, "student:update:any")
	if err != nil {
		return err
	}

	var req model.AddProjectRequest
	if err := c.BodyParser(&req); err != nil {
		return helper.BadRequest("body harus berupa JSON yang valid")
	}
	req.Title = strings.TrimSpace(req.Title)
	req.Link = strings.TrimSpace(req.Link)
	req.Technologies = NormalizeTags(req.Technologies)
	if errs := helper.ValidateStruct(req); errs != nil {
		return helper.Validation(errs)
	}

	portfolio, err := s.portfolios.AddProject(ctx, student.ID, model.Project{
		Title:        req.Title,
		Technologies: req.Technologies,
		Link:         req.Link,
		Year:         req.Year,
	})
	if err != nil {
		return translatePortfolioError(err)
	}
	return helper.Success(c, fiber.StatusCreated, "proyek berhasil ditambahkan", portfolio)
}

func (s *PortfolioService) DeleteProject(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	student, err := s.findAuthorizedStudent(c, "student:update:any")
	if err != nil {
		return err
	}

	portfolio, err := s.portfolios.DeleteProject(
		ctx,
		student.ID,
		c.Params("projectId"),
	)
	if err != nil {
		return translatePortfolioError(err)
	}
	return helper.Success(c, fiber.StatusOK, "proyek berhasil dihapus", portfolio)
}

func (s *PortfolioService) TopTechnologies(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	technologies, err := s.portfolios.TopTechnologies(ctx)
	if err != nil {
		return helper.Internal(err)
	}
	return helper.Success(
		c,
		fiber.StatusOK,
		"teknologi terpopuler berhasil diambil",
		technologies,
	)
}

func (s *PortfolioService) findAuthorizedStudent(
	c *fiber.Ctx,
	anyPermission string,
) (model.Student, error) {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	current, ok := helper.CurrentUser(c)
	if !ok {
		return model.Student{}, helper.Unauthorized("belum terautentikasi")
	}

	studentID, valid := helper.ParamID(c)
	if !valid {
		return model.Student{}, helper.BadRequest("id mahasiswa harus berupa angka positif")
	}

	student, err := s.students.FindByID(ctx, studentID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return model.Student{}, helper.NotFound("mahasiswa tidak ditemukan")
		}
		return model.Student{}, helper.Internal(err)
	}

	if !CanAccessStudent(current, student.OwnerID, s.perms, anyPermission) {
		return model.Student{}, helper.Forbidden(
			"tidak berhak mengakses portofolio mahasiswa lain",
		)
	}

	return student, nil
}

func normalizePortfolioRequest(req *model.UpsertPortfolioRequest) {
	req.Headline = strings.TrimSpace(req.Headline)
	req.Bio = strings.TrimSpace(req.Bio)

	for i := range req.Projects {
		req.Projects[i].Title = strings.TrimSpace(req.Projects[i].Title)
		req.Projects[i].Link = strings.TrimSpace(req.Projects[i].Link)
		req.Projects[i].Technologies = NormalizeTags(
			req.Projects[i].Technologies,
		)
	}

	for i := range req.Certificates {
		req.Certificates[i].Name = strings.TrimSpace(req.Certificates[i].Name)
		req.Certificates[i].Issuer = strings.TrimSpace(req.Certificates[i].Issuer)
	}
}

func translatePortfolioError(err error) error {
	switch {
	case errors.Is(err, repository.ErrInvalidID):
		return helper.BadRequest("id proyek tidak valid")
	case errors.Is(err, repository.ErrNotFound):
		return helper.NotFound("portofolio tidak ditemukan")
	case errors.Is(err, repository.ErrProjectNotFound):
		return helper.NotFound("proyek tidak ditemukan")
	case errors.Is(err, repository.ErrProjectLimit):
		return helper.NewError(
			fiber.StatusConflict,
			"PROJECT_LIMIT",
			"jumlah proyek sudah mencapai 20",
		)
	default:
		return helper.Internal(err)
	}
}
