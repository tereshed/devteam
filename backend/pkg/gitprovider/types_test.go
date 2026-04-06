package gitprovider

import "testing"

func TestAuthor_String(t *testing.T) {
	got := Author{Name: "Bot", Email: "b@b.io"}.String()
	want := "Bot <b@b.io>"
	if got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
}

func TestAuthor_String_Empty(t *testing.T) {
	got := Author{}.String()
	want := " <>"
	if got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
}
