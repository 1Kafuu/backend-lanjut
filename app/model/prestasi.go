package model

import "time"

type Prestasi struct {
	ID           int       `json:"id"`
	StudentID    int       `json:"student_id"`
	NamaPrestasi string    `json:"nama_prestasi"`
	Juara        string    `json:"juara"`
	CreatedAt    time.Time `json:"created_at"`
}

type PrestasiListQuery struct {
	Page   int
	Limit  int
	Search string
	Order  string
}

func (q PrestasiListQuery) Offset() int {
	return (q.Page - 1) * q.Limit
}
