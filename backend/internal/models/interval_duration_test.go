package models

import (
	"testing"
	"time"
)

func TestIntervalDuration_ScanString(t *testing.T) {
	cases := []struct {
		in       string
		expected time.Duration
	}{
		{"01:00:00", time.Hour},
		{"00:30:00", 30 * time.Minute},
		{"00:00:45", 45 * time.Second},
		{"00:00:00.500000", 500 * time.Millisecond},
		{"1 day 02:30:00", 24*time.Hour + 2*time.Hour + 30*time.Minute},
		{"2 days 00:00:00", 48 * time.Hour},
		{"30 minutes", 30 * time.Minute},
		{"3600 seconds", time.Hour},
		{"500000 microseconds", 500 * time.Millisecond},
		{"1 hour", time.Hour},
		{"5 mins", 5 * time.Minute}, // tolerated alias
	}
	for _, c := range cases {
		var d IntervalDuration
		if err := d.Scan(c.in); err != nil {
			t.Errorf("Scan(%q) returned error: %v", c.in, err)
			continue
		}
		if d.Duration() != c.expected {
			t.Errorf("Scan(%q) = %v, want %v", c.in, d.Duration(), c.expected)
		}
	}
}

func TestIntervalDuration_ScanNil(t *testing.T) {
	var d IntervalDuration
	if err := d.Scan(nil); err != nil {
		t.Fatalf("Scan(nil) error: %v", err)
	}
	if d != 0 {
		t.Errorf("Scan(nil) = %v, want 0", d)
	}
}

func TestIntervalDuration_ScanBytes(t *testing.T) {
	var d IntervalDuration
	if err := d.Scan([]byte("01:30:00")); err != nil {
		t.Fatalf("Scan([]byte) error: %v", err)
	}
	want := 90 * time.Minute
	if d.Duration() != want {
		t.Errorf("Scan([]byte) = %v, want %v", d.Duration(), want)
	}
}

func TestIntervalDuration_ValueRoundtrip(t *testing.T) {
	original := IntervalDuration(2 * time.Hour)
	v, err := original.Value()
	if err != nil {
		t.Fatalf("Value() error: %v", err)
	}
	s, ok := v.(string)
	if !ok {
		t.Fatalf("Value() returned %T, expected string", v)
	}

	var roundtrip IntervalDuration
	if err := roundtrip.Scan(s); err != nil {
		t.Fatalf("Scan after Value error: %v", err)
	}
	if roundtrip != original {
		t.Errorf("roundtrip: got %v, want %v (intermediate string=%q)", roundtrip, original, s)
	}
}

func TestIntervalDuration_ScanUnsupportedType(t *testing.T) {
	var d IntervalDuration
	if err := d.Scan(12345); err == nil {
		t.Error("expected error for int input, got nil")
	}
}

func TestIntervalDuration_ScanInvalid(t *testing.T) {
	var d IntervalDuration
	for _, bad := range []string{"not an interval", "01:02", "abc:def:ghi"} {
		if err := d.Scan(bad); err == nil {
			t.Errorf("Scan(%q) should fail, got nil", bad)
		}
	}
}
