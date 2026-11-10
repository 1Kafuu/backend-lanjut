package service

import "testing"

func TestWithinCreditLimit(t *testing.T) {
	tests := []struct {
		name   string
		taken  int
		adding int
		want   bool
	}{
		{"kursi masih kosong", 0, 3, true},
		{"tepat di batas", 21, 3, true},
		{"satu SKS di atas batas", 21, 4, false},
		{"jauh di atas batas", 24, 3, false},
		{"kelas tanpa SKS", 24, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := WithinCreditLimit(tt.taken, tt.adding); got != tt.want {
				t.Errorf("WithinCreditLimit(%d, %d) = %v, seharusnya %v", tt.taken, tt.adding, got, tt.want)
			}
		})
	}
}

func TestCreditsAfterSwap(t *testing.T) {
	tests := []struct {
		name    string
		taken   int
		leaving int
		joining int
		want    int
	}{
		{"kelas lebih besar", 12, 3, 4, 13},
		{"kelas lebih kecil", 12, 4, 3, 11},
		{"kelas sama", 12, 3, 3, 12},
		{"hanya satu kelas", 3, 3, 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CreditsAfterSwap(tt.taken, tt.leaving, tt.joining); got != tt.want {
				t.Errorf("CreditsAfterSwap(%d, %d, %d) = %d,harusnya %d",
					tt.taken, tt.leaving, tt.joining, got, tt.want)
			}
		})
	}
}
