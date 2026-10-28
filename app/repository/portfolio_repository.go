package repository

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"api-students/app/model"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

var (
	ErrProjectLimit    = errors.New("jumlah proyek sudah mencapai batas")
	ErrProjectNotFound = errors.New("proyek tidak ditemukan")
)

const MaxPortfolioProjects = 20

type PortfolioRepository interface {
	EnsureIndexes(ctx context.Context) error
	FindByStudentID(ctx context.Context, studentID int) (model.Portfolio, error)
	Upsert(ctx context.Context, portfolio model.Portfolio) (model.Portfolio, error)
	AddProject(
		ctx context.Context,
		studentID int,
		project model.Project,
	) (model.Portfolio, error)
	DeleteProject(
		ctx context.Context,
		studentID int,
		projectID string,
	) (model.Portfolio, error)
	TopTechnologies(ctx context.Context) ([]model.TechnologyCount, error)
}

type portfolioDocument struct {
	ID           bson.ObjectID         `bson:"_id,omitempty"`
	StudentID    int                   `bson:"student_id"`
	Headline     string                `bson:"headline"`
	Bio          string                `bson:"bio"`
	Projects     []projectDocument     `bson:"projects"`
	Certificates []certificateDocument `bson:"certificates"`
	CreatedAt    time.Time             `bson:"created_at"`
	UpdatedAt    time.Time             `bson:"updated_at"`
}

type projectDocument struct {
	ID           bson.ObjectID `bson:"_id"`
	Title        string        `bson:"title"`
	Technologies []string      `bson:"technologies"`
	Link         string        `bson:"link"`
	Year         int           `bson:"year"`
}

type certificateDocument struct {
	ID       bson.ObjectID `bson:"_id"`
	Name     string        `bson:"name"`
	Issuer   string        `bson:"issuer"`
	IssuedAt time.Time     `bson:"issued_at"`
}

func (d portfolioDocument) toModel() model.Portfolio {
	projects := make([]model.Project, 0, len(d.Projects))
	for _, project := range d.Projects {
		technologies := project.Technologies
		if technologies == nil {
			technologies = []string{}
		}
		projects = append(projects, model.Project{
			ID:           project.ID.Hex(),
			Title:        project.Title,
			Technologies: technologies,
			Link:         project.Link,
			Year:         project.Year,
		})
	}

	certificates := make([]model.Certificate, 0, len(d.Certificates))
	for _, certificate := range d.Certificates {
		certificates = append(certificates, model.Certificate{
			ID:       certificate.ID.Hex(),
			Name:     certificate.Name,
			Issuer:   certificate.Issuer,
			IssuedAt: certificate.IssuedAt,
		})
	}

	return model.Portfolio{
		StudentID:    d.StudentID,
		Headline:     d.Headline,
		Bio:          d.Bio,
		Projects:     projects,
		Certificates: certificates,
		CreatedAt:    d.CreatedAt,
		UpdatedAt:    d.UpdatedAt,
	}
}

type portfolioMongoRepository struct {
	coll *mongo.Collection
}

func NewPortfolioRepository(db *mongo.Database) PortfolioRepository {
	return &portfolioMongoRepository{
		coll: db.Collection("student_portfolios"),
	}
}

func (r *portfolioMongoRepository) EnsureIndexes(ctx context.Context) error {
	_, err := r.coll.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys: bson.D{{Key: "student_id", Value: 1}},
			Options: options.Index().
				SetUnique(true).
				SetName("student_id_unique"),
		},
		{
			Keys: bson.D{
				{Key: "projects.technologies", Value: 1},
			},
			Options: options.Index().SetName("project_technologies"),
		},
	})
	if err != nil {
		return fmt.Errorf("membuat index student_portfolios: %w", err)
	}
	return nil
}

func (r *portfolioMongoRepository) FindByStudentID(
	ctx context.Context, studentID int,
) (model.Portfolio, error) {
	var doc portfolioDocument
	err := r.coll.
		FindOne(ctx, bson.M{"student_id": studentID}).
		Decode(&doc)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return model.Portfolio{}, ErrNotFound
		}
		return model.Portfolio{}, fmt.Errorf("mengambil portofolio: %w", err)
	}
	return doc.toModel(), nil
}

func (r *portfolioMongoRepository) Upsert(
	ctx context.Context, portfolio model.Portfolio,
) (model.Portfolio, error) {
	timestamp := now()

	projects := make([]projectDocument, 0, len(portfolio.Projects))
	for _, project := range portfolio.Projects {
		technologies := project.Technologies
		if technologies == nil {
			technologies = []string{}
		}
		projects = append(projects, projectDocument{
			ID:           bson.NewObjectID(),
			Title:        project.Title,
			Technologies: technologies,
			Link:         project.Link,
			Year:         project.Year,
		})
	}

	certificates := make([]certificateDocument, 0, len(portfolio.Certificates))
	for _, certificate := range portfolio.Certificates {
		certificates = append(certificates, certificateDocument{
			ID:       bson.NewObjectID(),
			Name:     certificate.Name,
			Issuer:   certificate.Issuer,
			IssuedAt: certificate.IssuedAt,
		})
	}

	filter := bson.M{"student_id": portfolio.StudentID}
	update := bson.M{
		"$set": bson.M{
			"headline":     portfolio.Headline,
			"bio":          portfolio.Bio,
			"projects":     projects,
			"certificates": certificates,
			"updated_at":   timestamp,
		},
		"$setOnInsert": bson.M{
			"student_id": portfolio.StudentID,
			"created_at": timestamp,
		},
	}

	if _, err := r.coll.UpdateOne(
		ctx,
		filter,
		update,
		options.UpdateOne().SetUpsert(true),
	); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return model.Portfolio{}, ErrDuplicate
		}
		return model.Portfolio{}, fmt.Errorf("menyimpan portofolio: %w", err)
	}

	return r.FindByStudentID(ctx, portfolio.StudentID)
}

func (r *portfolioMongoRepository) AddProject(
	ctx context.Context, studentID int, project model.Project,
) (model.Portfolio, error) {
	technologies := project.Technologies
	if technologies == nil {
		technologies = []string{}
	}

	doc := projectDocument{
		ID:           bson.NewObjectID(),
		Title:        project.Title,
		Technologies: technologies,
		Link:         project.Link,
		Year:         project.Year,
	}

	limitField := "projects." + strconv.Itoa(MaxPortfolioProjects-1)
	var updated portfolioDocument
	err := r.coll.FindOneAndUpdate(
		ctx,
		bson.M{
			"student_id": studentID,
			limitField:   bson.M{"$exists": false},
		},
		bson.M{
			"$push": bson.M{"projects": doc},
			"$set":  bson.M{"updated_at": now()},
		},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	).Decode(&updated)
	if err == nil {
		return updated.toModel(), nil
	}
	if !errors.Is(err, mongo.ErrNoDocuments) {
		return model.Portfolio{}, fmt.Errorf("menambah proyek: %w", err)
	}

	count, err := r.coll.CountDocuments(ctx, bson.M{"student_id": studentID})
	if err != nil {
		return model.Portfolio{}, fmt.Errorf("memeriksa portofolio: %w", err)
	}
	if count == 0 {
		return model.Portfolio{}, ErrNotFound
	}
	return model.Portfolio{}, ErrProjectLimit
}

func (r *portfolioMongoRepository) DeleteProject(
	ctx context.Context, studentID int, projectID string,
) (model.Portfolio, error) {
	oid, err := parseObjectID(projectID)
	if err != nil {
		return model.Portfolio{}, err
	}

	result, err := r.coll.UpdateOne(
		ctx,
		bson.M{
			"student_id":   studentID,
			"projects._id": oid,
		},
		bson.M{
			"$pull": bson.M{
				"projects": bson.M{"_id": oid},
			},
			"$set": bson.M{"updated_at": now()},
		},
	)
	if err != nil {
		return model.Portfolio{}, fmt.Errorf("menghapus proyek: %w", err)
	}
	if result.MatchedCount == 0 {
		count, countErr := r.coll.CountDocuments(
			ctx,
			bson.M{"student_id": studentID},
		)
		if countErr != nil {
			return model.Portfolio{}, fmt.Errorf("memeriksa portofolio: %w", countErr)
		}
		if count == 0 {
			return model.Portfolio{}, ErrNotFound
		}
		return model.Portfolio{}, ErrProjectNotFound
	}

	return r.FindByStudentID(ctx, studentID)
}

func (r *portfolioMongoRepository) TopTechnologies(
	ctx context.Context,
) ([]model.TechnologyCount, error) {
	pipeline := mongo.Pipeline{
		{{Key: "$unwind", Value: "$projects"}},
		{{Key: "$unwind", Value: "$projects.technologies"}},
		{{Key: "$group", Value: bson.M{
			"_id":   "$projects.technologies",
			"total": bson.M{"$sum": 1},
		}}},
		{{Key: "$sort", Value: bson.D{
			{Key: "total", Value: -1},
			{Key: "_id", Value: 1},
		}}},
		{{Key: "$limit", Value: 10}},
	}

	cursor, err := r.coll.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("menghitung teknologi: %w", err)
	}
	defer cursor.Close(ctx)

	var rows []struct {
		Technology string `bson:"_id"`
		Total      int    `bson:"total"`
	}
	if err := cursor.All(ctx, &rows); err != nil {
		return nil, fmt.Errorf("membaca teknologi: %w", err)
	}

	result := make([]model.TechnologyCount, 0, len(rows))
	for _, row := range rows {
		result = append(result, model.TechnologyCount{
			Technology: row.Technology,
			Total:      row.Total,
		})
	}
	return result, nil
}
