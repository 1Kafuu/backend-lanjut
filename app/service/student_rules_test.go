package service

import (
	"api-students/app/model"
	"api-students/helper"
	"testing"
)

func TestApplyPatch(t *testing.T) {
	initial := model.Student{ID: 1, NIM: 111, Name: "sari", Grade: 3.0, IsActive: true}
	inactive := false
	newName := "sari barru"
	newGrade := 3.8
	// patch is_active
	result := ApplyPatch(initial, model.PatchStudentRequest{IsActive: &inactive})
	if result.IsActive != false {
		t.Error("is_active should be false")
	}
	if result.Name != "sari" {
		t.Error("name should not change when not patched")
	}
	// patch name & grade
	result2 := ApplyPatch(initial, model.PatchStudentRequest{Name: &newName, Grade: &newGrade})
	if result2.Name != "sari barru" || result2.Grade != 3.8 {
		t.Errorf("patch failed got %+v", result2)
	}
	// invalid input ditolak di lapis validasi, bukan merge:
	// merge murni tidak menolak apa pun, bahkan string kosong
	empty := ""
	result3 := ApplyPatch(initial, model.PatchStudentRequest{Name: &empty})
	if result3.Name != "" {
		t.Errorf("pure merge must apply as-is, got %q", result3.Name)
	}
	// ...sedangkan validator menolaknya:
	if errs := helper.ValidateStruct(model.PatchStudentRequest{Name: &empty}); errs["name"] == "" {
		t.Error("empty name should fail ValidateStruct")
	}
}

func TestIsEmptyPatch(t *testing.T) {
	if !IsEmptyPatch(model.PatchStudentRequest{}) {
		t.Error("empty struct should be empty patch")
	}
	n := "x"
	if IsEmptyPatch(model.PatchStudentRequest{Name: &n}) {
		t.Error("should not be empty when name set")
	}
}

func TestCountTotalPages(t *testing.T) {
	cases := []struct{ total, limit, want int }{
		{0, 10, 0},
		{1, 10, 1},
		{10, 10, 1},
		{11, 10, 2},
		{137, 20, 7},
		{5, 0, 0},
	}
	for _, tc := range cases {
		if got := CountTotalPages(tc.total, tc.limit); got != tc.want {
			t.Errorf("total=%d limit=%d: want %d got %d", tc.total, tc.limit, tc.want, got)
		}
	}
}
