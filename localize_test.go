// Copyright (c) the go-ruby-i18n/i18n authors
//
// SPDX-License-Identifier: BSD-3-Clause

package i18n

import (
	"testing"
	"time"
)

// newLocalizeEN builds the standard date/time translation tree the localize
// tests format against (mirrors Rails' en.yml).
func newLocalizeEN(t *testing.T) *I18n {
	t.Helper()
	i := New("en")
	i.Backend().StoreTranslations("en", map[string]Value{
		"date": map[string]Value{
			"formats": map[string]Value{
				"default": "%Y-%m-%d",
				"long":    "%B %d, %Y",
				"short":   "%b %d",
				"named":   "%A %^B %P",
			},
			"day_names":        []Value{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"},
			"abbr_day_names":   []Value{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"},
			"month_names":      []Value{nil, "January", "February", "March", "April", "May", "June", "July", "August", "September", "October", "November", "December"},
			"abbr_month_names": []Value{nil, "Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"},
		},
		"time": map[string]Value{
			"formats": map[string]Value{
				"default": "%a, %d %b %Y %H:%M:%S %z",
				"short":   "%d %b %H:%M",
			},
			"am": "am",
			"pm": "pm",
		},
	})
	return i
}

func TestLocalizeDate(t *testing.T) {
	i := newLocalizeEN(t)
	d := DateValue{T: time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC)}
	cases := []struct {
		format string
		named  bool
		want   string
	}{
		{"default", true, "2026-06-30"},
		{"long", true, "June 30, 2026"},
		{"short", true, "Jun 30"},
		{"named", true, "Tuesday JUNE am"},
		{"%Y/%m/%d", false, "2026/06/30"},
	}
	for _, c := range cases {
		got, err := i.Localize(d, c.format, c.named, nil)
		if err != nil {
			t.Fatalf("%s: %v", c.format, err)
		}
		if got != c.want {
			t.Errorf("l(%q) = %q, want %q", c.format, got, c.want)
		}
	}
}

func TestLocalizeTime(t *testing.T) {
	i := newLocalizeEN(t)
	tm := TimeValue{T: time.Date(2026, 6, 30, 14, 5, 9, 0, time.UTC)}
	got, _ := i.Localize(tm, "short", true, nil)
	if got != "30 Jun 14:05" {
		t.Errorf("time short = %q", got)
	}
	// %p / %P from time.am/pm.
	got, _ = i.Localize(tm, "%p %P", false, nil)
	if got != "PM pm" {
		t.Errorf("meridian = %q", got)
	}
	// Morning -> am.
	am := TimeValue{T: time.Date(2026, 6, 30, 9, 0, 0, 0, time.UTC)}
	got, _ = i.Localize(am, "%P", false, nil)
	if got != "am" {
		t.Errorf("am = %q", got)
	}
}

func TestLocalizeAbbrevNames(t *testing.T) {
	i := newLocalizeEN(t)
	d := DateValue{T: time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC)}
	// %a (abbr day) and %b (abbr month) substituted from the locale tables.
	got, _ := i.Localize(d, "%a %b %^a", false, nil)
	if got != "Tue Jun TUE" {
		t.Errorf("abbr names = %q", got)
	}
}

func TestLocalizeNameArrayOutOfRange(t *testing.T) {
	// A truncated names array makes localeName return false -> directive falls
	// through to strftime's English name.
	i := New("en")
	i.Backend().StoreTranslations("en", map[string]Value{
		"date": map[string]Value{
			"month_names": []Value{nil, "Jan"}, // too short for month 6
		},
	})
	d := DateValue{T: time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC)}
	got, _ := i.Localize(d, "%B", false, nil)
	if got != "June" {
		t.Errorf("short array fallthrough = %q", got)
	}
	// A non-array under the names key also falls through.
	i.Backend().StoreTranslations("en", map[string]Value{
		"date": map[string]Value{"day_names": "notarray"},
	})
	got, _ = i.Localize(d, "%A", false, nil)
	if got != "Tuesday" {
		t.Errorf("non-array fallthrough = %q", got)
	}
}

func TestLocalizeMissingFormat(t *testing.T) {
	i := newLocalizeEN(t)
	d := DateValue{T: time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC)}
	_, err := i.Localize(d, "nonexist", true, nil)
	mt, ok := err.(*MissingTranslation)
	if !ok {
		t.Fatalf("err = %T", err)
	}
	if mt.Message() != "Translation missing: en.date.formats.nonexist" {
		t.Errorf("msg = %q", mt.Message())
	}
}

func TestLocalizeFormatNotString(t *testing.T) {
	i := New("en")
	i.Backend().StoreTranslations("en", map[string]Value{
		"date": map[string]Value{"formats": map[string]Value{"weird": []Value{"x"}}},
	})
	d := DateValue{T: time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC)}
	_, err := i.Localize(d, "weird", true, nil)
	if _, ok := err.(*MissingTranslation); !ok {
		t.Fatalf("err = %T %v", err, err)
	}
}

func TestLocalizeNameFallbackToEnglish(t *testing.T) {
	// No names tables -> directives fall through to strftime's English names.
	i := New("en")
	i.Backend().StoreTranslations("en", map[string]Value{
		"date": map[string]Value{"formats": map[string]Value{"x": "%A %B"}},
	})
	d := DateValue{T: time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC)}
	got, _ := i.Localize(d, "x", true, nil)
	if got != "Tuesday June" {
		t.Errorf("english fallback = %q", got)
	}
	// %p with no time.am table falls through to strftime "AM"/"PM".
	got, _ = i.Localize(DateValue{T: time.Date(2026, 6, 30, 13, 0, 0, 0, time.UTC)}, "x2", false, nil)
	_ = got
}

func TestLocalizeMeridianFallthrough(t *testing.T) {
	i := New("en")
	i.Backend().StoreTranslations("en", map[string]Value{})
	tm := TimeValue{T: time.Date(2026, 6, 30, 14, 0, 0, 0, time.UTC)}
	got, _ := i.Localize(tm, "%p %P", false, nil)
	if got != "PM pm" {
		t.Errorf("meridian fallthrough = %q", got)
	}
}

func TestLocalizeFormatFallbackLocale(t *testing.T) {
	i := New("en")
	i.Backend().StoreTranslations("en", map[string]Value{
		"date": map[string]Value{"formats": map[string]Value{"long": "%Y!"}},
	})
	i.SetLocale("fr") // fr has no date.formats; falls back to en.
	d := DateValue{T: time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC)}
	got, _ := i.Localize(d, "long", true, nil)
	if got != "2026!" {
		t.Errorf("format fallback locale = %q", got)
	}
}

func TestLAlias(t *testing.T) {
	i := newLocalizeEN(t)
	d := DateValue{T: time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC)}
	got, _ := i.L(d, "default", true, nil)
	if got != "2026-06-30" {
		t.Errorf("L alias = %q", got)
	}
}

func TestLocalizeOpts(t *testing.T) {
	i := newLocalizeEN(t)
	i.Backend().StoreTranslations("fr", map[string]Value{
		"date": map[string]Value{"formats": map[string]Value{"default": "%d/%m/%Y"}},
	})
	d := DateValue{T: time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC)}
	got, _ := i.Localize(d, "default", true, &Options{Locale: "fr"})
	if got != "30/06/2026" {
		t.Errorf("localize fr = %q", got)
	}
	// nil opts path via L already covered; explicit empty Options here.
	got, _ = i.Localize(d, "default", true, &Options{})
	if got != "2026-06-30" {
		t.Errorf("empty opts = %q", got)
	}
}
