package common

import (
	"testing"
)

func TestStorageSizeString(t *testing.T) {
	tests := []struct {
		size StorageSize
		str  string
	}{
		{2381273, "2.27 MiB"},
		{2192, "2.14 KiB"},
		{12, "12.00 B"},
	}

	for _, test := range tests {
		if test.size.String() != test.str {
			t.Errorf("%f: got %q, want %q", float64(test.size), test.size.String(), test.str)
		}
	}
}
