package main

import (
	"fmt"
)

type User struct {
	ID int `json:"id"`
	Username string `json:"username"`
	IsActive bool `json:"is_active"`
}

func (u *User) Activate() {u.IsActive = true}
func (u User) Info() string {return fmt.Sprintf("%s aktif : %v", u.Username, u.IsActive)}

func tukarNilai(a, b *int) {
	*a, *b = *b, *a
}

func main() {
	// Variabel
	var nama string = "John Kaisen"
	umur := 67
	var kosong string
	fmt.Printf("%s berumur %d zero value : %q\n", nama, umur, kosong)

	// Slice
	nilai := []int{10, 20, 30}
	nilai = append(nilai, 40)
	fmt.Println(nilai, nilai[1:3], len(nilai), cap(nilai))

	// Map
	skor := make(map[string]int)
	skor["John Kaisen"] = 100
	if n, ada := skor["Asep Kopling"]; ada {
		fmt.Println("Skor Asep Kopling", n)
	} else {
		fmt.Println("Asep Kopling belum punya nilai")
	}

	// Channel
	ch := make(chan string)
	go func() {
		ch <- "Halo John Kaisen"
	} ()
	fmt.Println(<-ch)

	// Pointer
	a, b := 10, 20
	tukarNilai(&a, &b)
	fmt.Println("setelah ditukar: ", a, b)

	// Struct
	u := User{ID: 1, Username: "John Kaisen"}
	u.Activate()
	fmt.Println(u.Info())
}