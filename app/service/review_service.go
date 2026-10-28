package service

import (
	"errors"
	"strings"

	"api-students/app/model"
	"api-students/app/repository"
	"api-students/helper"

	"github.com/gofiber/fiber/v2"
)

// ReviewService spans two databases: courses and enrollment in PostgreSQL, reviews in MongoDB.
type ReviewService struct {
	reviews repository.ReviewRepository
	courses repository.CourseRepository
	perms   *helper.PermissionSet
}

func NewReviewService(
	reviews repository.ReviewRepository,
	courses repository.CourseRepository,
	perms *helper.PermissionSet,
) *ReviewService {
	return &ReviewService{
		reviews: reviews,
		courses: courses,
		perms:   perms,
	}
}

func (s *ReviewService) Create(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	current, ok := helper.CurrentUser(c)
	if !ok {
		return helper.Unauthorized("belum terautentikasi")
	}

	courseID, valid := helper.ParamID(c)
	if !valid {
		return helper.BadRequest("id kelas harus berupa angka positif")
	}

	var req model.CreateReviewRequest
	if err := c.BodyParser(&req); err != nil {
		return helper.BadRequest("body harus berupa JSON yang valid")
	}
	req.Comment = strings.TrimSpace(req.Comment)
	req.Tags = NormalizeTags(req.Tags)
	if errs := helper.ValidateStruct(req); errs != nil {
		return helper.Validation(errs)
	}

	// No foreign key from MongoDB to PostgreSQL, so verify course and enrollment here.
	if _, err := s.courses.FindByID(ctx, courseID); err != nil {
		return translateCourseError(err)
	}
	enrolled, err := s.courses.IsEnrolled(ctx, courseID, current.UserID)
	if err != nil {
		return helper.Internal(err)
	}
	if !enrolled {
		return helper.NewError(
			fiber.StatusForbidden,
			"NOT_ENROLLED",
			"hanya peserta kelas yang boleh menulis ulasan",
		)
	}

	created, err := s.reviews.Create(ctx, model.Review{
		CourseID: courseID,
		UserID:   current.UserID,
		Username: current.Username,
		Rating:   req.Rating,
		Comment:  req.Comment,
		Tags:     req.Tags,
	})
	if err != nil {
		return translateReviewError(err)
	}

	return helper.Created(
		c,
		"ulasan berhasil disimpan",
		created,
		"/api/v1/reviews/"+created.ID,
	)
}

func (s *ReviewService) ListByCourse(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	courseID, valid := helper.ParamID(c)
	if !valid {
		return helper.BadRequest("id kelas harus berupa angka positif")
	}

	query := model.ReviewQuery{
		CourseID: courseID,
		Tag:      strings.ToLower(strings.TrimSpace(c.Query("tag"))),
		Before:   strings.TrimSpace(c.Query("before")),
		Limit:    c.QueryInt("limit", 10),
	}
	if query.Limit < 1 || query.Limit > 50 {
		query.Limit = 10
	}

	// Fetch one extra row to detect whether a next page exists.
	want := query.Limit
	query.Limit = want + 1

	reviews, err := s.reviews.FindByCourse(ctx, query)
	if err != nil {
		return translateReviewError(err)
	}

	hasMore := len(reviews) > want
	if hasMore {
		reviews = reviews[:want]
	}

	nextBefore := ""
	if hasMore && len(reviews) > 0 {
		nextBefore = reviews[len(reviews)-1].ID
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "daftar ulasan berhasil diambil",
		"data":    reviews,
		"meta": fiber.Map{
			"limit":       want,
			"has_more":    hasMore,
			"next_before": nextBefore,
		},
	})
}

func (s *ReviewService) Summary(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	courseID, valid := helper.ParamID(c)
	if !valid {
		return helper.BadRequest("id kelas harus berupa angka positif")
	}
	if _, err := s.courses.FindByID(ctx, courseID); err != nil {
		return translateCourseError(err)
	}

	summary, err := s.reviews.Summary(ctx, courseID)
	if err != nil {
		return helper.Internal(err)
	}
	return helper.Success(
		c,
		fiber.StatusOK,
		"ringkasan ulasan berhasil diambil",
		summary,
	)
}

func (s *ReviewService) Get(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	review, err := s.reviews.FindByID(ctx, c.Params("id"))
	if err != nil {
		return translateReviewError(err)
	}
	return helper.Success(c, fiber.StatusOK, "ulasan ditemukan", review)
}

func (s *ReviewService) Update(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	current, ok := helper.CurrentUser(c)
	if !ok {
		return helper.Unauthorized("belum terautentikasi")
	}

	var req model.UpdateReviewRequest
	if err := c.BodyParser(&req); err != nil {
		return helper.BadRequest("body harus berupa JSON yang valid")
	}
	if req.Rating == nil && req.Comment == nil && req.Tags == nil {
		return helper.BadRequest("tidak ada field yang diubah")
	}
	if req.Comment != nil {
		trimmed := strings.TrimSpace(*req.Comment)
		req.Comment = &trimmed
	}
	if req.Tags != nil {
		normalized := NormalizeTags(*req.Tags)
		req.Tags = &normalized
	}
	if errs := helper.ValidateStruct(req); errs != nil {
		return helper.Validation(errs)
	}

	updated, err := s.reviews.UpdateOwn(ctx, c.Params("id"), current.UserID, req)
	if err != nil {
		return translateReviewError(err)
	}
	return helper.Success(c, fiber.StatusOK, "ulasan berhasil diubah", updated)
}

func (s *ReviewService) Delete(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	current, ok := helper.CurrentUser(c)
	if !ok {
		return helper.Unauthorized("belum terautentikasi")
	}

	anyOwner := s.perms.Can(current.Role, "review:moderate")
	if err := s.reviews.Delete(ctx, c.Params("id"), current.UserID, anyOwner); err != nil {
		return translateReviewError(err)
	}
	return helper.NoContent(c)
}

func (s *ReviewService) Reply(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	current, ok := helper.CurrentUser(c)
	if !ok {
		return helper.Unauthorized("belum terautentikasi")
	}

	var req model.ReplyRequest
	if err := c.BodyParser(&req); err != nil {
		return helper.BadRequest("body harus berupa JSON yang valid")
	}
	req.Body = strings.TrimSpace(req.Body)
	if errs := helper.ValidateStruct(req); errs != nil {
		return helper.Validation(errs)
	}

	updated, err := s.reviews.AddReply(ctx, c.Params("id"), model.Reply{
		UserID:   current.UserID,
		Username: current.Username,
		Body:     req.Body,
	})
	if err != nil {
		return translateReviewError(err)
	}
	return helper.Success(c, fiber.StatusCreated, "balasan berhasil ditambahkan", updated)
}

func (s *ReviewService) MarkHelpful(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	current, ok := helper.CurrentUser(c)
	if !ok {
		return helper.Unauthorized("belum terautentikasi")
	}

	updated, err := s.reviews.MarkHelpful(ctx, c.Params("id"), current.UserID)
	if err != nil {
		return translateReviewError(err)
	}
	return helper.Success(c, fiber.StatusOK, "terima kasih atas penilaiannya", updated)
}

func translateReviewError(err error) error {
	switch {
	case errors.Is(err, repository.ErrInvalidID):
		return helper.BadRequest("id ulasan tidak valid")
	case errors.Is(err, repository.ErrNotFound):
		return helper.NotFound("ulasan tidak ditemukan")
	case errors.Is(err, repository.ErrDuplicate):
		return helper.NewError(
			fiber.StatusConflict,
			"ALREADY_REVIEWED",
			"Anda sudah menulis ulasan untuk kelas ini",
		)
	case errors.Is(err, repository.ErrNotOwner):
		return helper.Forbidden("hanya pemilik ulasan yang boleh melakukan ini")
	case errors.Is(err, repository.ErrOwnReview):
		return helper.NewError(
			fiber.StatusConflict,
			"OWN_REVIEW",
			"tidak dapat menilai ulasan sendiri",
		)
	case errors.Is(err, repository.ErrAlreadyVoted):
		return helper.NewError(
			fiber.StatusConflict,
			"ALREADY_VOTED",
			"Anda sudah menilai ulasan ini",
		)
	case errors.Is(err, repository.ErrReplyLimit):
		return helper.NewError(
			fiber.StatusConflict,
			"REPLY_LIMIT",
			"jumlah balasan sudah mencapai batas",
		)
	default:
		return helper.Internal(err)
	}
}
