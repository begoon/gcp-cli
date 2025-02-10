package ext

import (
	"testing"
)

func TestHumanizeSize(t *testing.T) {
	cases := []struct {
		size int
		want string
	}{
		{0, "0 bytes"},
		{1300, "1.27 KB"},
		{1300 * 1024, "1.27 MB"},
		{1300 * 1024 * 1024, "1.27 GB"},
		{1300 * 1024 * 1024 * 1024, "1300.00 GB"},
	}
	for _, tt := range cases {
		got := HumanizeSize(tt.size)
		if got != tt.want {
			t.Errorf("HumanizeSize(%d) = %q; want %q", tt.size, got, tt.want)
		}
	}
}
