package jupiter

import "testing"

func TestParseAmountUint_Valid(t *testing.T) {
	tests := []struct {
		in  string
		out uint64
	}{
		{"0", 0},
		{"1", 1},
		{"1234567890", 1234567890},
	}

	for _, tt := range tests {
		got, err := ParseAmountUint(tt.in)
		if err != nil {
			t.Fatalf("ParseAmountUint(%q) unexpected error: %v", tt.in, err)
		}
		if got != tt.out {
			t.Fatalf("ParseAmountUint(%q) = %d, want %d", tt.in, got, tt.out)
		}
	}
}

func TestParseAmountUint_Invalid(t *testing.T) {
	invalid := []string{"", "abc", "-1", "1.23"}
	for _, in := range invalid {
		if _, err := ParseAmountUint(in); err == nil {
			t.Fatalf("ParseAmountUint(%q) expected error, got nil", in)
		}
	}
}

