package config

import (
	"testing"
	"time"
)

// These tests swap the process-wide time.Local, so each restores it and none
// may run in parallel.

func TestApplyTimeZoneSetsLocal(t *testing.T) {
	orig := time.Local
	t.Cleanup(func() { time.Local = orig })

	if err := applyTimeZone(" America/Chicago "); err != nil {
		t.Fatalf("applyTimeZone: %v", err)
	}
	if got := time.Now().Location().String(); got != "America/Chicago" {
		t.Fatalf("time.Now() zone = %q, want America/Chicago", got)
	}

	// The presenters resolve "Local" by name; it must follow too.
	loc, err := time.LoadLocation("Local")
	if err != nil {
		t.Fatal(err)
	}
	if loc.String() != "America/Chicago" {
		t.Fatalf(`LoadLocation("Local") = %q, want America/Chicago`, loc)
	}

	// The case this option exists for: 8pm on Dec 31 in Chicago is already
	// Jan 1 in UTC, and must stay in December for a local-zone cut.
	utc := time.Date(2026, time.January, 1, 2, 0, 0, 0, time.UTC)
	if y, m := utc.In(time.Local).Year(), utc.In(time.Local).Month(); y != 2025 || m != time.December {
		t.Fatalf("local date = %d-%s, want 2025-December", y, m)
	}
}

func TestApplyTimeZoneEmptyKeepsLocal(t *testing.T) {
	orig := time.Local
	t.Cleanup(func() { time.Local = orig })

	if err := applyTimeZone("  "); err != nil {
		t.Fatalf("applyTimeZone(blank): %v", err)
	}
	if time.Local != orig {
		t.Fatal("blank time_zone replaced time.Local")
	}
}

func TestApplyTimeZoneInvalid(t *testing.T) {
	orig := time.Local
	t.Cleanup(func() { time.Local = orig })

	if err := applyTimeZone("America/Nowhere"); err == nil {
		t.Fatal("expected an error for an unknown zone")
	}
	if time.Local != orig {
		t.Fatal("failed time_zone replaced time.Local")
	}
}

func TestTimeZoneEnvOverride(t *testing.T) {
	t.Setenv("TIME_ZONE", "Europe/London")
	cfg := &EnvConfig{TimeZone: "America/Chicago"}
	if got := envOverride(cfg).TimeZone; got != "Europe/London" {
		t.Fatalf("TimeZone = %q, want Europe/London from TIME_ZONE", got)
	}
}
