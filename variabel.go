package main

import "fmt"

func main() {
	var nama string = "Ucup Slamet"
	var umur int = 67
	var ipk float64 = 3.67
	var sehat bool = false

	hobi := []string{"Mancing", "Judol", "Depo"}
	fmt.Printf("Mahasiswa bernama %s dengan umur %d, mempunyai ipk sebesar %f kondisinya %t sehat dan hobinya adalah %v", nama, umur, ipk, sehat, hobi)
}