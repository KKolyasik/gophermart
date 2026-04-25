package luhn

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLuhnValidator_Valid(t *testing.T) {
	tests := []struct {
		name   string
		number string
		want   bool
	}{
		{
			name:   "correct 1",
			number: "12345678903",
			want:   true,
		},
		{
			name:   "correct 2",
			number: "2377225624",
			want:   true,
		},
		{
			name:   "incorrect 1",
			number: "321123",
			want:   false,
		},
		{
			name:   "incorrect 2",
			number: "4674312",
			want:   false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewLuhnValidator()
			got := v.Valid(tt.number)
			if tt.want {
				require.True(t, got)
			} else {
				require.False(t, got)
			}
		})
	}
}
