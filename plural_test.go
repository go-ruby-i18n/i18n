// Copyright (c) the go-ruby-i18n/i18n authors
//
// SPDX-License-Identifier: BSD-3-Clause

package i18n

import "testing"

func TestSimplePluralRule(t *testing.T) {
	if simplePluralRule(1) != "one" || simplePluralRule(0) != "other" || simplePluralRule(5) != "other" {
		t.Error("simple rule")
	}
}

func TestCldrSlavicRule(t *testing.T) {
	cases := map[int]string{
		1: "one", 21: "one", 101: "one",
		2: "few", 3: "few", 24: "few",
		5: "many", 0: "many", 11: "many", 12: "many", 14: "many", 25: "many",
		-1: "other", // negative count falls through to the "other" category
	}
	for n, want := range cases {
		if got := cldrSlavicRule(n); got != want {
			t.Errorf("slavic(%d) = %q want %q", n, got, want)
		}
	}
}

func TestCldrWestSlavicRule(t *testing.T) {
	cases := map[int]string{1: "one", 2: "few", 4: "few", 5: "other", 0: "other"}
	for n, want := range cases {
		if got := cldrWestSlavicRule(n); got != want {
			t.Errorf("westslavic(%d) = %q want %q", n, got, want)
		}
	}
}

func TestCldrPolishRule(t *testing.T) {
	cases := map[int]string{1: "one", 2: "few", 3: "few", 4: "few", 22: "few", 5: "many", 0: "many", 12: "many"}
	for n, want := range cases {
		if got := cldrPolishRule(n); got != want {
			t.Errorf("polish(%d) = %q want %q", n, got, want)
		}
	}
}

func TestCldrArabicRule(t *testing.T) {
	cases := map[int]string{0: "zero", 1: "one", 2: "two", 3: "few", 10: "few", 11: "many", 99: "many", 100: "other", 101: "other"}
	for n, want := range cases {
		if got := cldrArabicRule(n); got != want {
			t.Errorf("arabic(%d) = %q want %q", n, got, want)
		}
	}
}

func TestCldrFrenchRule(t *testing.T) {
	cases := map[int]string{0: "one", 1: "one", 2: "other", 10: "other"}
	for n, want := range cases {
		if got := cldrFrenchRule(n); got != want {
			t.Errorf("french(%d) = %q want %q", n, got, want)
		}
	}
}

func TestPluralRuleForTableAndDefault(t *testing.T) {
	s := NewSimple()
	// Built-in table hit.
	if s.pluralRuleFor("pl")(5) != "many" {
		t.Error("pl from table")
	}
	// Unknown locale -> simple rule.
	if s.pluralRuleFor("xx")(1) != "one" {
		t.Error("unknown -> simple")
	}
	// Registered override.
	s.RegisterPluralRule("pl", func(int) string { return "one" })
	if s.pluralRuleFor("pl")(5) != "one" {
		t.Error("override")
	}
}
