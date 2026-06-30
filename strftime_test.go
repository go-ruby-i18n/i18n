// Copyright (c) the go-ruby-i18n/i18n authors
//
// SPDX-License-Identifier: BSD-3-Clause

package i18n

import (
	"testing"
	"time"
)

func TestStrftimeDirectives(t *testing.T) {
	loc := time.FixedZone("CEST", 2*3600)
	tm := TimeValue{T: time.Date(2026, 6, 30, 14, 5, 9, 123456789, loc)}
	cases := map[string]string{
		"%Y":   "2026",
		"%y":   "26",
		"%C":   "20",
		"%m":   "06",
		"%d":   "30",
		"%e":   "30",
		"%-d":  "30",
		"%H":   "14",
		"%k":   "14",
		"%I":   "02",
		"%l":   " 2",
		"%-l":  "2",
		"%M":   "05",
		"%S":   "09",
		"%L":   "123",
		"%3N":  "123",
		"%6N":  "123456",
		"%N":   "123456789",
		"%12N": "123456789000",
		"%p":   "PM",
		"%P":   "pm",
		"%j":   "181",
		"%u":   "2",
		"%w":   "2",
		"%A":   "Tuesday",
		"%a":   "Tue",
		"%B":   "June",
		"%b":   "Jun",
		"%h":   "Jun",
		"%z":   "+0200",
		"%Z":   "CEST",
		"%%":   "%",
		"%n":   "\n",
		"%t":   "\t",
		"%^a":  "TUE",
		"%-m":  "6",
		"%_d":  "30",
		"lit":  "lit",
		"%Q":   "%Q", // unknown directive emitted literally
	}
	for f, want := range cases {
		got := strftime(tm, f)
		if got != want {
			t.Errorf("strftime(%q) = %q, want %q", f, got, want)
		}
	}
}

func TestStrftimeMidnightNoonHour(t *testing.T) {
	mid := TimeValue{T: time.Date(2026, 1, 1, 0, 30, 0, 0, time.UTC)}
	if got := strftime(mid, "%I %P %k %l"); got != "12 am  0 12" {
		t.Errorf("midnight = %q", got)
	}
	noon := TimeValue{T: time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)}
	if got := strftime(noon, "%I %p"); got != "12 PM" {
		t.Errorf("noon = %q", got)
	}
}

func TestStrftimeSingleDigitPadding(t *testing.T) {
	// A single-digit day/second exercises the space-pad (%_, %e) and AM (%p) paths.
	tm := TimeValue{T: time.Date(2026, 6, 5, 9, 0, 5, 0, time.UTC)}
	cases := map[string]string{
		"%_S": " 5", // '_' flag space-pads to width 2
		"%e":  " 5", // %e space-pads day to 2
		"%-e": "5",  // %e with '-' flag: no pad
		"%p":  "AM", // hour 9 -> AM
		"%-d": "5",  // '-' flag: no pad
		"%d":  "05", // zero pad
		"%0e": "05", // explicit zero-pad flag on %e via pad()
	}
	for f, want := range cases {
		if got := strftime(tm, f); got != want {
			t.Errorf("strftime(%q) = %q, want %q", f, got, want)
		}
	}
}

func TestStrftimeColonOffsetAndSunday(t *testing.T) {
	loc := time.FixedZone("", 2*3600)
	tm := TimeValue{T: time.Date(2026, 6, 28, 0, 0, 0, 250000000, loc)} // Sunday
	if got := strftime(tm, "%:z"); got != "+02:00" {
		t.Errorf("%%:z = %q", got)
	}
	if got := strftime(tm, "%u %w"); got != "7 0" {
		t.Errorf("Sunday %%u/%%w = %q", got)
	}
	// %2L truncates the fractional seconds to two digits.
	if got := strftime(tm, "%2L"); got != "25" {
		t.Errorf("%%2L = %q", got)
	}
	// Small nsec needs left zero-padding (fracSeconds loop).
	small := TimeValue{T: time.Date(2026, 1, 1, 0, 0, 0, 5, time.UTC)}
	if got := strftime(small, "%N"); got != "000000005" {
		t.Errorf("%%N small = %q", got)
	}
	// %:z with no following z (trailing) emits verbatim.
	if got := strftime(tm, "x%:"); got != "x%:" {
		t.Errorf("trailing %%: = %q", got)
	}
}

func TestStrftimeNegativeOffset(t *testing.T) {
	loc := time.FixedZone("", -5*3600)
	tm := TimeValue{T: time.Date(2026, 6, 30, 14, 0, 0, 0, loc)}
	if got := strftime(tm, "%z"); got != "-0500" {
		t.Errorf("neg offset = %q", got)
	}
}

func TestStrftimeUTCZone(t *testing.T) {
	tm := TimeValue{T: time.Date(2026, 6, 30, 14, 0, 0, 0, time.UTC)}
	if got := strftime(tm, "%Z"); got != "UTC" {
		t.Errorf("utc zone = %q", got)
	}
	if off := tm.ZoneOffset(); off != 0 {
		t.Errorf("utc offset = %d", off)
	}
}

func TestStrftimeTrailingPercent(t *testing.T) {
	tm := TimeValue{T: time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC)}
	if got := strftime(tm, "x%"); got != "x%" {
		t.Errorf("trailing %% = %q", got)
	}
	// Flags then end-of-string.
	if got := strftime(tm, "%-"); got != "%-" {
		t.Errorf("trailing flag = %q", got)
	}
}

func TestDateValueAdapter(t *testing.T) {
	d := DateValue{T: time.Date(2026, 6, 30, 14, 5, 9, 99, time.UTC)}
	if d.Year() != 2026 || d.Month() != 6 || d.Day() != 30 {
		t.Error("date ymd")
	}
	if d.Hour() != 0 || d.Min() != 0 || d.Sec() != 0 || d.Nsec() != 0 {
		t.Error("date clock should be zero")
	}
	if d.Wday() != 2 || d.Yday() != 181 {
		t.Error("date wday/yday")
	}
	if d.ZoneOffset() != 0 || d.ZoneName() != "" {
		t.Error("date zone")
	}
	if d.HasTime() {
		t.Error("date HasTime")
	}
}

func TestTimeValueAdapter(t *testing.T) {
	tm := TimeValue{T: time.Date(2026, 6, 30, 14, 5, 9, 7, time.UTC)}
	if tm.Year() != 2026 || tm.Month() != 6 || tm.Day() != 30 {
		t.Error("time ymd")
	}
	if tm.Hour() != 14 || tm.Min() != 5 || tm.Sec() != 9 || tm.Nsec() != 7 {
		t.Error("time clock")
	}
	if tm.Wday() != 2 || tm.Yday() != 181 {
		t.Error("time wday/yday")
	}
	if !tm.HasTime() {
		t.Error("time HasTime")
	}
	// Named local zone passes through.
	loc := time.FixedZone("PST", -8*3600)
	z := TimeValue{T: time.Date(2026, 1, 1, 0, 0, 0, 0, loc)}
	if z.ZoneName() != "PST" {
		t.Errorf("zone name = %q", z.ZoneName())
	}
	// Empty-name fixed zone returns "".
	anon := TimeValue{T: time.Date(2026, 1, 1, 0, 0, 0, 0, time.FixedZone("", 3600))}
	if anon.ZoneName() != "" {
		t.Errorf("anon zone = %q", anon.ZoneName())
	}
}

func TestNegativeYearModulo(t *testing.T) {
	// Year %y handles the modulo branch deterministically for a normal year.
	tm := TimeValue{T: time.Date(5, 6, 30, 0, 0, 0, 0, time.UTC)}
	if got := strftime(tm, "%y"); got != "05" {
		t.Errorf("%%y small year = %q", got)
	}
}
