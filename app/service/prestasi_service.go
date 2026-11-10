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

// List godoc
//
// @Summary      Daftar prestasi mahasiswa
// @Description  Mendukung paginasi, pencarian, dan pengurutan.
// @Tags         prestasi
// @Produce      json
// @Security     BearerAuth
// @Param        page    query  int     false  "Nomor halaman"      default(1)
// @Param        limit   query  int     false  "Jumlah data per halaman"  default(10)
// @Param        search  query  string  false  "Cari nama prestasi"
// @Param        order   query  string  false  "Arah pengurutan"  Enums(asc, desc)
// @Success      200     {object}  model.WebResponse{data=[]model.Prestasi}
// @Failure      401     {object}  model.ErrorResponse  "Belum terautentikasi"
// @Router       /prestasi [get]
func (s *PrestasiService) List(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()
	q := helper.PrestasiParseListQuery(c)
	prestasi, total, err := s.prestasiRepo.FindAll(ctx, q)
	if err != nil {
		return helper.Internal(err)
	}
	return helper.SuccessList(c, "daftar prestasi berhasil diambil", prestasi, &model.Meta{
		Page: q.Page, Limit: q.Limit, Total: total, TotalPages: CountTotalPages(total, q.Limit),
	})
}

// Get godoc
//
// @Summary      Detail satu prestasi
// @Tags         prestasi
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  int  true  "ID prestasi"
// @Success      200  {object}  model.WebResponse{data=model.Prestasi}
// @Failure      400  {object}  model.ErrorResponse  "ID bukan angka positif"
// @Failure      404  {object}  model.ErrorResponse  "Prestasi tidak ditemukan"
// @Router       /prestasi/{id} [get]
func (p *PrestasiService) Get(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()
	id, valid := helper.ParamID(c)
	if !valid {
		return helper.BadRequest("id harus berupa angka positif")
	}
	prestasi, err := p.prestasiRepo.FindByID(ctx, id)
	if err != nil {
		return translateError(err, "prestasi")
	}
	return helper.Success(c, fiber.StatusOK, "prestasi ditemukan", prestasi)
}
