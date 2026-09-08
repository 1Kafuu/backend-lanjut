package service

import (
	"strings"
	"api-students/app/model"
)

func ValidateCreate(req model.CreateStudentRequest) map[string]string {
	errs := map[string]string{}
	if strings.TrimSpace(req.Name) == "" {
		errs["name"] = "wajib diisi"
	}
	if req.NIM == 0 {
		errs["nim"] = "NIM wajib diisi"
	}
	if req.Grade < 0 || req.Grade > 4 {
		errs["grade"] = "grade harus antara 0.0 - 4.0"
	}
	return errs
}

func ValidateReplace(req model.ReplaceStudentRequest) map[string]string {
	errs := map[string]string{}
	if strings.TrimSpace(req.Name) == "" {
		errs["name"] = "wajib diisi pada PUT"
	}
	if req.NIM == 0 {
		errs["nim"] = "NIM wajib diisi pada PUT"
	}
	if req.Grade == nil {
		errs["grade"] = "wajib diisi pada PUT"
	} else if *req.Grade < 0 || *req.Grade > 4 {
		errs["grade"] = "grade harus antara 0.0 - 4.0"
	}
	if req.IsActive == nil {
		errs["is_active"] = "wajib diisi pada PUT"
	}
	return errs
}

func ApplyPatch(current model.Student, req model.PatchStudentRequest) (model.Student, map[string]string) {
	errs := map[string]string{}
	if req.Name != nil {
		if strings.TrimSpace(*req.Name) == "" {
			errs["name"] = "tidak boleh kosong"
		} else {
			current.Name = strings.TrimSpace(*req.Name)
		}
	}
	if req.NIM != nil {
		if *req.NIM == 0 {
			errs["nim"] = "tidak valid"
		} else {
			current.NIM = *req.NIM
		}
	}
	if req.Grade != nil {
		if *req.Grade < 0 || *req.Grade > 4 {
			errs["grade"] = "grade harus 0.0 - 4.0"
		} else {
			current.Grade = *req.Grade
		}
	}
	if req.IsActive != nil {
		current.IsActive = *req.IsActive
	}
	return current, errs
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