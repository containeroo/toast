package checker

import (
	"testing"
)

func TestExtractScheme(t *testing.T) {
	t.Parallel()

	t.Run("Valid address with http scheme", func(t *testing.T) {
		t.Parallel()

		scheme, err := extractScheme("http://example.com")
		if err != nil {
			t.Fatalf("expected no error, got %q", err)
		}

		if scheme != "http" {
			t.Fatalf("expected scheme 'http', got %q", scheme)
		}
	})

	t.Run("Valid address with https scheme", func(t *testing.T) {
		t.Parallel()

		scheme, err := extractScheme("https://example.com")
		if err != nil {
			t.Fatalf("expected no error, got %q", err)
		}

		if scheme != "https" {
			t.Fatalf("expected scheme 'https', got %q", scheme)
		}
	})

	t.Run("Invalid address without scheme", func(t *testing.T) {
		t.Parallel()

		_, err := extractScheme("example.com")

		if err == nil {
			t.Fatal("expected an error, got none")
		}

		expected := "no scheme found in address: example.com"
		if err.Error() != expected {
			t.Errorf("expected error containing %q, got %q", expected, err)
		}
	})

	t.Run("Invalid address with scheme only", func(t *testing.T) {
		t.Parallel()

		scheme, err := extractScheme("ftp://")
		if err != nil {
			t.Fatalf("expected no error, got %q", err)
		}

		if scheme != "ftp" {
			t.Fatalf("expected scheme 'ftp', got %q", scheme)
		}
	})

	t.Run("Invalid address with missing colon", func(t *testing.T) {
		t.Parallel()

		_, err := extractScheme("http//example.com")
		if err == nil {
			t.Fatal("expected an error, got none")
		}

		expected := "no scheme found in address: http//example.com"
		if err.Error() != expected {
			t.Errorf("expected error containing %q, got %q", expected, err)
		}
	})
}
