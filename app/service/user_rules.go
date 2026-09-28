package service

import (
	"api-students/app/model"
	"strings"
)

func ApplyUserPatch(current model.User, req model.PatchUserRequest) model.User {
	if req.Username != nil {
		current.Username = strings.TrimSpace(*req.Username)
	}
	if req.Email != nil {
		current.Email = strings.TrimSpace(*req.Email)
	}
	if req.IsActive != nil {
		current.IsActive = *req.IsActive
	}
	return current
}

func IsEmptyUserPatch(req model.PatchUserRequest) bool {
	return req.Username == nil && req.Email == nil && req.IsActive == nil
}
