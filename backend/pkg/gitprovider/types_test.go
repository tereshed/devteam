package gitprovider

import "testing"

func TestAuthor_Validate(t *testing.T) {
	t.Parallel()
	if err := (Author{}).Validate(); err != nil {
		t.Fatal(err)
	}
	if err := (Author{Name: "A", Email: ""}).Validate(); err == nil {
		t.Fatal("expected error")
	}
	if err := (Author{Name: "Bad\nX", Email: "a@b.c"}).Validate(); err == nil {
		t.Fatal("expected error")
	}
}

func TestAuthor_String(t *testing.T) {
	t.Parallel()
	got := Author{Name: "Bot", Email: "b@b.io"}.String()
	want := "Bot <b@b.io>"
	if got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
}

func TestAuthor_String_Empty(t *testing.T) {
	t.Parallel()
	got := Author{}.String()
	want := " <>"
	if got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
}
