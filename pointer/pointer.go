package main

import "fmt"

func swapPointer(a, b *int) {
	*a, *b = *b, *a
}

func swapValue(a, b int) {
	a, b = b, a
}

func updateSliceValue(slice []string, newItem string) {
	slice = append(slice, newItem)
}

func updateSlicePointer(slice *[]string, newItem string) {
	*slice = append(*slice, newItem)
}

func main() {
	angka1 := 67
	angka2 := 81
	fmt.Println("Angka Awal", angka1, angka2)

	swapValue(angka1, angka2)
	fmt.Println("Setelah Swap by Value ", angka1, angka2)

	swapPointer(&angka1, &angka2)
	fmt.Println("Setelah Swap by Pointer ", angka1, angka2)

	buah := []string{"Jeruk"}
	fmt.Println("Slice Awal", buah)

	updateSliceValue(buah, "Apel")
	fmt.Println("Setelah Update Slice by Value", buah)

	updateSlicePointer(&buah, "Mangga")
	fmt.Println("Setelah Update Slice by Pointer", buah)

}
