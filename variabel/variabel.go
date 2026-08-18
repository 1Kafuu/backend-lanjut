package main

import "fmt"

func main() {
	var nama string = "Ucup Slamet"
	var umur int = 67
	var ipk float64 = 3.67
	var sehat bool = false

	hobi := []string{"Mancing", "Judol", "Depo"}
	fmt.Printf("Mahasiswa bernama %s dengan umur %d, mempunyai ipk sebesar %f kondisinya %t sehat dan hobinya adalah %v\n", nama, umur, ipk, sehat, hobi)

	// Deklarasi map
	mahasiswa := make(map[string]int)
	mahasiswa["Ucup"] = 67 // Menambah data ke map
	mahasiswa["Slamet"] = 81
	mahasiswa["Budi"] = 90

	fmt.Println("Nilai mahasiswa bernama Budi adalah : ", mahasiswa["Budi"])

	mahasiswa["Budi"] = 100 // Perbarui data di map
	fmt.Println("Nilai mahasiswa bernama Budi setelah diperbarui adalah : ", mahasiswa["Budi"])

	nilai, ada := mahasiswa["Budi"] // Mengambil data dari map
	if ada {
		fmt.Println("Nilai mahasiswa bernama Budi adalah : ", nilai)
	} else {
		fmt.Println("Mahasiswa bernama Budi tidak ditemukan")
	}

	delete(mahasiswa, "Budi") // Menghapus data dari map
	nilai, ada = mahasiswa["Budi"]
	if ada {
		fmt.Println("Nilai mahasiswa bernama Budi adalah : ", nilai)
	} else {
		fmt.Println("Mahasiswa bernama Budi tidak ditemukan")
	}

	for key, value := range mahasiswa {
		fmt.Println("Nama mahasiswa : ", key, " Nilai : ", value)
	}

	// Pointer
	
}