package service

// MaxCreditsPerSemester adalah batas SKS yang boleh diambil dalam satu KRS.
const MaxCreditsPerSemester = 24

// WithinCreditLimit memeriksa batas SKS tanpa database atau Fiber.
// Fungsi ini hanya valid bila taken masih berlaku saat disimpan; jaminan
// tersebut berasal dari LockStudent, bukan dari fungsi ini.
func WithinCreditLimit(taken, adding int) bool {
	return taken+adding <= MaxCreditsPerSemester
}

// CreditsAfterSwap menghitung SKS setelah pindah kelas.
func CreditsAfterSwap(taken, leaving, joining int) int {
	return taken - leaving + joining
}
