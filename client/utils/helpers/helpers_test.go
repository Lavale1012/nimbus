package helpers

import "testing"

// --- IsEmailValid ---

func TestIsEmailValid(t *testing.T) {
	valid := []string{
		"user@example.com",
		"user.name+tag@sub.domain.org",
		"a@b.co",
		"test123@test.io",
	}
	for _, email := range valid {
		if !IsEmailValid(email) {
			t.Errorf("expected %q to be valid", email)
		}
	}

	invalid := []string{
		"",
		"notanemail",
		"missing@",
		"@nodomain.com",
		"spaces in@email.com",
		"double@@domain.com",
	}
	for _, email := range invalid {
		if IsEmailValid(email) {
			t.Errorf("expected %q to be invalid", email)
		}
	}
}

// --- FormatSize ---

func TestFormatSize(t *testing.T) {
	tests := []struct {
		bytes int64
		want  string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1023, "1023 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1024 * 1024, "1.0 MB"},
		{1024 * 1024 * 2, "2.0 MB"},
		{1024 * 1024 * 1024, "1.0 GB"},
		{1024 * 1024 * 1024 * 3, "3.0 GB"},
	}
	for _, tc := range tests {
		got := FormatSize(tc.bytes)
		if got != tc.want {
			t.Errorf("FormatSize(%d) = %q, want %q", tc.bytes, got, tc.want)
		}
	}
}
