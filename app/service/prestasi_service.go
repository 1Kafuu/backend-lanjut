package service

import (
	"api-students/app/model"
	"api-students/app/repository"
	"api-students/helper"

	"github.com/gofiber/fiber/v2"
)

type PrestasiService struct {
	prestasiRepo repository.PrestasiRepository
	studentRepo  repository.StudentRepository
}

func NewPrestasiService(prestasiRepo repository.PrestasiRepository, studentRepo repository.StudentRepository) *PrestasiService {
	return &PrestasiService{prestasiRepo: prestasiRepo, studentRepo: studentRepo}
}

func (s *PrestasiService) List(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()
	q := helper.PrestasiParseListQuery(c)
	prestasi, total, err := s.prestasiRepo.FindAll(ctx, q)
	if err != nil {
		return helper.Fail(c, fiber.StatusInternalServerError, "gagal mengambil data prestasi")
	}
	return helper.SuccessList(c, "daftar prestasi berhasil diambil", prestasi, &model.Meta{
		Page: q.Page, Limit: q.Limit, Total: total, TotalPages: CountTotalPages(total, q.Limit),
	})
}

func (p *PrestasiService) Get(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()
	id, valid := helper.ParamID(c)
	if !valid {
		return helper.Fail(c, fiber.StatusBadRequest, "id harus berupa angka positif")
	}
	prestasi, err := p.prestasiRepo.FindByID(ctx, id)
	if err != nil {
		return translateError(c, err, "gagal mengambil data prestasi")
	}
	return helper.Success(c, fiber.StatusOK, "prestasi ditemukan", prestasi)
}
