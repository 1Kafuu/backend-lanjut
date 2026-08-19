package main

import "fmt"

type Student struct {
	ID       int     `json:"id"`
	Name     string  `json:"name"`
	Grade    float64 `json:"grade"`
	IsActive bool    `json:"is_active"`
}

func (s Student) GetInfo() string {
	return fmt.Sprintf("ID : %v, Nama : %s, Nilai : %.2f, Status Aktif : %t", s.ID, s.Name, s.Grade, s.IsActive)
}

func (s *Student) UpdateGrade(grade float64)  {
	s.Grade = grade
}

func (s *Student) Activate() {s.IsActive = true}

func (s *Student) Deactivate() {s.IsActive = false}

func main () {
	siswa := Student{
		ID: 01,
		Name: "Budi",	
		Grade: 3.9,
	}

	siswa.Activate()
	siswa.UpdateGrade(4)
	fmt.Println(siswa.GetInfo())
	siswa.Deactivate()
	fmt.Println(siswa.GetInfo())
}