package model

import "time"

type Student struct {
	ID        int       `json:"id"`
	NIM       int       `json:"nim"`
	Name      string    `json:"name"`
	Grade     float64   `json:"grade"`
	IsActive  bool      `json:"is_active"`
	OwnerID   *int      `json:"owner_id"`
	CreatedAt time.Time `json:"created_at"`
}

// POST - CreateStudent
type CreateStudentRequest struct {
	NIM   int     `json:"nim" validate:"required"`
	Name  string  `json:"name" validate:"required"`
	Grade float64 `json:"grade" validate:"gte=0,lte=4"`
}

// PUT- UpdateAllField
type ReplaceStudentRequest struct {
	Name     string   `json:"name" validate:"required,max=120"`
	NIM      int      `json:"nim" validate:"required"`
	Grade    *float64 `json:"grade" validate:"required,gte=0,lte=4"`
	IsActive *bool    `json:"is_active" validate:"required"`
}

// PATCH - UpdatePartialField
type PatchStudentRequest struct {
	Name     *string  `json:"name,omitempty" validate:"omitnil,min=1,max=120"`
	NIM      *int     `json:"nim,omitempty" validate:"omitnil,gt=0"`
	Grade    *float64 `json:"grade,omitempty" validate:"omitnil,gte=0,lte=4"`
	IsActive *bool    `json:"is_active,omitempty"`
}

type ListQuery struct {
	Page     int
	Limit    int
	Search   string
	Sort     string
	Order    string
	IsActive *bool
	MinGrade *float64
	MaxGrade *float64
}

func (q ListQuery) Offset() int {
	return (q.Page - 1) * q.Limit
}
