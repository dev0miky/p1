package compliance

import (
	"testing"
	"time"
)

func mustTime(t *testing.T, layout, s string) time.Time {
	t.Helper()
	v, err := time.Parse(layout, s)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestHoursAllowsWithinDefaultWindow(t *testing.T) {
	at := mustTime(t, time.RFC3339, "2026-05-19T14:30:00-04:00")
	d := checkHours(Input{Now: at, Timezone: "America/New_York"})
	if !d.Eligible {
		t.Fatalf("2:30pm ET monday should be eligible, got %v", d)
	}
}

func TestHoursBlocksBefore8AM(t *testing.T) {
	at := mustTime(t, time.RFC3339, "2026-05-19T07:30:00-04:00")
	d := checkHours(Input{Now: at, Timezone: "America/New_York"})
	if d.Eligible || d.Reason != "hours:before_open" {
		t.Fatalf("7:30am should be before_open, got %+v", d)
	}
}

func TestHoursBlocksAfter9PM(t *testing.T) {
	at := mustTime(t, time.RFC3339, "2026-05-19T21:01:00-04:00")
	d := checkHours(Input{Now: at, Timezone: "America/New_York"})
	if d.Eligible || d.Reason != "hours:after_close" {
		t.Fatalf("9:01pm should be after_close, got %+v", d)
	}
}

func TestHoursAllowedAt9PMBoundaryRejected(t *testing.T) {
	at := mustTime(t, time.RFC3339, "2026-05-19T21:00:00-04:00")
	d := checkHours(Input{Now: at, Timezone: "America/New_York"})
	if d.Eligible {
		t.Fatalf("exactly 9pm should be blocked (closing time exclusive), got %+v", d)
	}
}

func TestHoursAllowedAt8AMSharp(t *testing.T) {
	at := mustTime(t, time.RFC3339, "2026-05-19T08:00:00-04:00")
	d := checkHours(Input{Now: at, Timezone: "America/New_York"})
	if !d.Eligible {
		t.Fatalf("exactly 8am should be eligible, got %+v", d)
	}
}

func TestHoursBlocksSundayByDefault(t *testing.T) {
	at := mustTime(t, time.RFC3339, "2026-05-17T14:30:00-04:00")
	d := checkHours(Input{Now: at, Timezone: "America/New_York"})
	if d.Eligible || d.Reason != "hours:sunday_blocked" {
		t.Fatalf("sunday should be blocked, got %+v", d)
	}
}

func TestHoursAllowsSundayWhenExplicit(t *testing.T) {
	at := mustTime(t, time.RFC3339, "2026-05-17T14:30:00-04:00")
	d := checkHours(Input{Now: at, Timezone: "America/New_York", AllowSunday: true})
	if !d.Eligible {
		t.Fatalf("sunday with AllowSunday should be eligible, got %+v", d)
	}
}

func TestHoursTimezoneAware(t *testing.T) {
	at := mustTime(t, time.RFC3339, "2026-05-19T05:00:00-08:00")
	dPacific := checkHours(Input{Now: at, Timezone: "America/Los_Angeles"})
	dEastern := checkHours(Input{Now: at, Timezone: "America/New_York"})
	if dPacific.Eligible {
		t.Fatal("5am Pacific should be before_open in LA")
	}
	if !dEastern.Eligible {
		t.Fatal("5am Pacific = 8am Eastern, should be eligible in NYC")
	}
}

func TestHoursRejectsBadTimezone(t *testing.T) {
	at := time.Now()
	d := checkHours(Input{Now: at, Timezone: "Atlantis/Lost"})
	if d.Eligible || d.Reason[:10] != "hours:bad_" {
		t.Fatalf("expected bad_timezone error, got %+v", d)
	}
}

func TestHoursCustomWindow(t *testing.T) {
	at := mustTime(t, time.RFC3339, "2026-05-19T19:30:00-04:00")
	d := checkHours(Input{Now: at, Timezone: "America/New_York", OpenHour: 9, CloseHour: 18})
	if d.Eligible {
		t.Fatalf("7:30pm should be blocked by 9-18 window, got %+v", d)
	}
}

func TestHoursRejectsInvertedWindow(t *testing.T) {
	at := time.Now()
	d := checkHours(Input{Now: at, Timezone: "America/New_York", OpenHour: 21, CloseHour: 8})
	if d.Eligible || d.Reason != "hours:invalid_window" {
		t.Fatalf("inverted window should be invalid, got %+v", d)
	}
}
