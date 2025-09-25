package cli

import "testing"

func TestParseHostSpecPort(t *testing.T) {
	name, upstream, err := parseHostSpec("User:8080")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "user" || upstream != "http://localhost:8080" {
		t.Fatalf("unexpected result: %s %s", name, upstream)
	}
}

func TestParseHostSpecExplicitURL(t *testing.T) {
	name, upstream, err := parseHostSpec("api=http://127.0.0.1:9000")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "api" || upstream != "http://127.0.0.1:9000" {
		t.Fatalf("unexpected result: %s %s", name, upstream)
	}
}

func TestParseHostSpecInvalid(t *testing.T) {
	if _, _, err := parseHostSpec("bad"); err == nil {
		t.Fatalf("expected error for malformed spec")
	}
}
