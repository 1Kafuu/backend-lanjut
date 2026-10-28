package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"api-students/app/model"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

var (
	ErrInvalidID    = errors.New("format id tidak valid")
	ErrNotOwner     = errors.New("bukan pemilik data")
	ErrOwnReview    = errors.New("tidak boleh menilai ulasan sendiri")
	ErrAlreadyVoted = errors.New("sudah pernah menilai ulasan ini")
	ErrReplyLimit   = errors.New("jumlah balasan sudah mencapai batas")
)

const MaxRepliesPerReview = 50

type ReviewRepository interface {
	EnsureIndexes(ctx context.Context) error
	Create(ctx context.Context, review model.Review) (model.Review, error)
	FindByID(ctx context.Context, id string) (model.Review, error)
	FindByCourse(ctx context.Context, query model.ReviewQuery) ([]model.Review, error)
}

type reviewDocument struct {
	ID           bson.ObjectID   `bson:"_id,omitempty"`
	CourseID     int             `bson:"course_id"`
	UserID       int             `bson:"user_id"`
	Username     string          `bson:"username"`
	Rating       int             `bson:"rating"`
	Comment      string          `bson:"comment"`
	Tags         []string        `bson:"tags"`
	Replies      []replyDocument `bson:"replies"`
	HelpfulCount int             `bson:"helpful_count"`
	HelpfulBy    []int           `bson:"helpful_by"`
	CreatedAt    time.Time       `bson:"created_at"`
	UpdatedAt    time.Time       `bson:"updated_at"`
}

type replyDocument struct {
	ID        bson.ObjectID `bson:"_id"`
	UserID    int           `bson:"user_id"`
	Username  string        `bson:"username"`
	Body      string        `bson:"body"`
	CreatedAt time.Time     `bson:"created_at"`
}

func (d reviewDocument) toModel() model.Review {
	replies := make([]model.Reply, 0, len(d.Replies))
	for _, reply := range d.Replies {
		replies = append(replies, model.Reply{
			ID:        reply.ID.Hex(),
			UserID:    reply.UserID,
			Username:  reply.Username,
			Body:      reply.Body,
			CreatedAt: reply.CreatedAt,
		})
	}

	tags := d.Tags
	if tags == nil {
		tags = []string{}
	}

	return model.Review{
		ID:           d.ID.Hex(),
		CourseID:     d.CourseID,
		UserID:       d.UserID,
		Username:     d.Username,
		Rating:       d.Rating,
		Comment:      d.Comment,
		Tags:         tags,
		Replies:      replies,
		HelpfulCount: d.HelpfulCount,
		CreatedAt:    d.CreatedAt,
		UpdatedAt:    d.UpdatedAt,
	}
}

type reviewMongoRepository struct {
	coll *mongo.Collection
}

func NewReviewRepository(db *mongo.Database) ReviewRepository {
	return &reviewMongoRepository{
		coll: db.Collection("course_reviews"),
	}
}

func now() time.Time {
	return time.Now().UTC().Truncate(time.Millisecond)
}

func parseObjectID(id string) (bson.ObjectID, error) {
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return bson.ObjectID{}, ErrInvalidID
	}
	return oid, nil
}

func (r *reviewMongoRepository) EnsureIndexes(ctx context.Context) error {
	_, err := r.coll.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "course_id", Value: 1},
				{Key: "user_id", Value: 1},
			},
			Options: options.Index().
				SetUnique(true).
				SetName("course_user_unique"),
		},
		{
			Keys: bson.D{
				{Key: "course_id", Value: 1},
				{Key: "_id", Value: -1},
			},
			Options: options.Index().SetName("course_newest"),
		},
		{
			Keys: bson.D{
				{Key: "course_id", Value: 1},
				{Key: "tags", Value: 1},
			},
			Options: options.Index().SetName("course_tags"),
		},
	})
	if err != nil {
		return fmt.Errorf("membuat index course_reviews: %w", err)
	}
	return nil
}

func (r *reviewMongoRepository) Create(
	ctx context.Context, review model.Review,
) (model.Review, error) {
	timestamp := now()

	tags := review.Tags
	if tags == nil {
		tags = []string{}
	}

	doc := reviewDocument{
		ID:           bson.NewObjectID(),
		CourseID:     review.CourseID,
		UserID:       review.UserID,
		Username:     review.Username,
		Rating:       review.Rating,
		Comment:      review.Comment,
		Tags:         tags,
		Replies:      []replyDocument{},
		HelpfulCount: 0,
		HelpfulBy:    []int{},
		CreatedAt:    timestamp,
		UpdatedAt:    timestamp,
	}

	if _, err := r.coll.InsertOne(ctx, doc); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return model.Review{}, ErrDuplicate
		}
		return model.Review{}, fmt.Errorf("menyimpan ulasan: %w", err)
	}

	return doc.toModel(), nil
}

func (r *reviewMongoRepository) FindByID(
	ctx context.Context, id string,
) (model.Review, error) {
	oid, err := parseObjectID(id)
	if err != nil {
		return model.Review{}, err
	}

	var doc reviewDocument
	if err := r.coll.FindOne(ctx, bson.M{"_id": oid}).Decode(&doc); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return model.Review{}, ErrNotFound
		}
		return model.Review{}, fmt.Errorf("mengambil ulasan: %w", err)
	}

	return doc.toModel(), nil
}

func (r *reviewMongoRepository) FindByCourse(
	ctx context.Context, query model.ReviewQuery,
) ([]model.Review, error) {
	filter := bson.M{"course_id": query.CourseID}

	// query.Tag is a plain string, so it can never become an operator like {"$ne": null}.
	if query.Tag != "" {
		filter["tags"] = query.Tag
	}
	if query.Before != "" {
		before, err := parseObjectID(query.Before)
		if err != nil {
			return nil, err
		}
		filter["_id"] = bson.M{"$lt": before}
	}

	opts := options.Find().
		SetSort(bson.D{{Key: "_id", Value: -1}}).
		SetLimit(int64(query.Limit)).
		SetProjection(bson.M{"helpful_by": 0})

	cursor, err := r.coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("mengambil daftar ulasan: %w", err)
	}
	defer cursor.Close(ctx)

	var docs []reviewDocument
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("membaca daftar ulasan: %w", err)
	}

	result := make([]model.Review, 0, len(docs))
	for _, doc := range docs {
		result = append(result, doc.toModel())
	}
	return result, nil
}
