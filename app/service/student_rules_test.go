package service

import (
	"api-students/app/model"
	"testing"
)

func TestValidateCreate(t *testing.T) {
	// valid case
	if errs := ValidateCreate(model.CreateStudentRequest{NIM: 123, Name: "Sari", Grade: 3.5}); len(errs) != 0 {
		t.Fatalf("expected no errs, got %v", errs)
	}
	// invalid all
	errs := ValidateCreate(model.CreateStudentRequest{NIM: 0, Name: "", Grade: 5})
	if errs["name"] == "" || errs["nim"] == "" || errs["grade"] == "" {
		t.Errorf("expected all fields error, got %v", errs)
	}
	// grade boundary
	errs = ValidateCreate(model.CreateStudentRequest{NIM: 1, Name: "A", Grade: -0.1})
	if errs["grade"] == "" {
		t.Error("expected grade error for -0.1")
	}
}

func TestValidateReplace(t *testing.T) {
	grade := 3.0
	active := true
	// valid
	req := model.ReplaceStudentRequest{Name: "Budi", NIM: 456, Grade: &grade, IsActive: &active}
	if errs := ValidateReplace(req); len(errs) != 0 {
		t.Fatalf("expected no errs, got %v", errs)
	}
	// missing grade
	req2 := model.ReplaceStudentRequest{Name: "Budi", NIM: 456, IsActive: &active, Grade: nil}
	if errs := ValidateReplace(req2); errs["grade"] == "" {
		t.Error("expected grade required error")
	}
}

func TestApplyPatch(t *testing.T) {
	initial := model.Student{ID: 1, NIM: 111, Name: "sari", Grade: 3.0, IsActive: true}
	inactive := false
	newName := "sari barru"
	newGrade := 3.8
	// patch is_active
	result, errs := ApplyPatch(initial, model.PatchStudentRequest{IsActive: &inactive})
	if len(errs) != 0 {
		t.Fatalf("unexpected errs %v", errs)
	}
	if result.IsActive != false {
		t.Error("is_active should be false")
	}
	if result.Name != "sari" {
		t.Error("name should not change when not patched")
	}
	// patch name & grade
	result2, errs := ApplyPatch(initial, model.PatchStudentRequest{Name: &newName, Grade: &newGrade})
	if len(errs) != 0 {
		t.Fatalf("unexpected errs %v", errs)
	}
	if result2.Name != "sari barru" || result2.Grade != 3.8 {
		t.Errorf("patch failed got %+v", result2)
	}
	// invalid patch
	empty := ""
	_, errs = ApplyPatch(initial, model.PatchStudentRequest{Name: &empty})
	if errs["name"] == "" {
		t.Error("expected name empty error")
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
