package service

import (
	"api-students/app/model"
	"strings"
)

func ApplyPatch(current model.Student, req model.PatchStudentRequest) model.Student {
	if req.Name != nil {
		current.Name = strings.TrimSpace(*req.Name)
	}
	if req.NIM != nil {
		current.NIM = *req.NIM
	}
	if req.Grade != nil {
		current.Grade = *req.Grade
	}
	if req.IsActive != nil {
		current.IsActive = *req.IsActive
	}
	return current
}

func IsEmptyPatch(req model.PatchStudentRequest) bool {
	return req.Name == nil && req.NIM == nil && req.Grade == nil && req.IsActive == nil
}

func CountTotalPages(total, limit int) int {
	if limit <= 0 {
		return 0
	}
	return (total + limit - 1) / limit
}
