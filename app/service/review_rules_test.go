package service

import (
	"reflect"
	"testing"
)

func TestNormalizeTags(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want []string
	}{
		{"duplikat setelah normalisasi", []string{"Seru", "seru"}, []string{"seru"}},
		{"trim spasi", []string{"  Seru  "}, []string{"seru"}},
		{"kosong diabaikan", []string{"", "   "}, []string{}},
		{"nil", nil, []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeTags(tt.in)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NormalizeTags(%v) = %v, seharusnya %v", tt.in, got, tt.want)
			}
		})
	}
}
