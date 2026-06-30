// Copyright (c) the go-ruby-i18n/i18n authors
//
// SPDX-License-Identifier: BSD-3-Clause

package i18n

import "testing"

func TestInterpolateBasics(t *testing.T) {
	v := map[string]Value{"name": "X", "count": 3}
	cases := []struct {
		in, want string
	}{
		{"plain", "plain"},
		{"hi %{name}", "hi X"},
		{"100%%", "100%"},
		{"%{name|word} end", "X end"}, // pipe form
		{"%<count>d items", "3 items"},
		{"%<count>05d", "00003"},
		{"no placeholders", "no placeholders"},
	}
	for _, c := range cases {
		got, err := interpolate(c.in, v, keepMissingInterpolation)
		if err != nil {
			t.Fatalf("%q: %v", c.in, err)
		}
		if got != c.want {
			t.Errorf("interpolate(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestInterpolateReservedKey(t *testing.T) {
	_, err := interpolate("x %{scope} y", map[string]Value{"scope": "S"}, keepMissingInterpolation)
	rk, ok := err.(*ReservedInterpolationKey)
	if !ok {
		t.Fatalf("err = %T", err)
	}
	if rk.Key != "scope" {
		t.Errorf("reserved key = %q", rk.Key)
	}
	if rk.Error() == "" {
		t.Error("empty reserved error")
	}
	// Escaped %%{scope} is NOT reserved.
	got, err := interpolate("x %%{scope} y", map[string]Value{}, keepMissingInterpolation)
	if err != nil {
		t.Fatalf("escaped reserved: %v", err)
	}
	if got != "x %{scope} y" {
		t.Errorf("escaped = %q", got)
	}
}

func TestInterpolateMissingRaise(t *testing.T) {
	_, err := interpolate("hi %{name}", map[string]Value{}, RaiseMissingInterpolation)
	mia, ok := err.(*MissingInterpolationArgument)
	if !ok {
		t.Fatalf("err = %T", err)
	}
	if mia.Key != "name" || mia.Error() == "" {
		t.Errorf("mia = %+v", mia)
	}
	// Angle form missing + raise.
	_, err = interpolate("%<n>d", map[string]Value{}, RaiseMissingInterpolation)
	if _, ok := err.(*MissingInterpolationArgument); !ok {
		t.Fatalf("angle raise err = %T", err)
	}
}

func TestInterpolateMissingKeepLiteral(t *testing.T) {
	got, _ := interpolate("hi %{name}", map[string]Value{}, keepMissingInterpolation)
	if got != "hi %{name}" {
		t.Errorf("brace keep = %q", got)
	}
	got, _ = interpolate("%<n>d", map[string]Value{}, keepMissingInterpolation)
	if got != "%<n>d" {
		t.Errorf("angle keep = %q", got)
	}
}

func TestInterpolateCustomHandler(t *testing.T) {
	h := func(key string, _ map[string]Value, _ string) (Value, error) {
		if key == "n" {
			return 7, nil // angle form applies sprintf to the substituted value
		}
		return "[" + key + "]", nil
	}
	got, _ := interpolate("hi %{name} %<n>d", map[string]Value{}, h)
	if got != "hi [name] 7" {
		t.Errorf("custom handler = %q", got)
	}
}

func TestSprintfSpecVerbs(t *testing.T) {
	cases := []struct {
		spec string
		v    Value
		want string
	}{
		{"d", 42, "42"},
		{"i", 42, "42"},
		{"u", 42, "42"},
		{"05d", 7, "00007"},
		{"x", 255, "ff"},
		{"X", 255, "FF"},
		{"o", 8, "10"},
		{"b", 5, "101"},
		{"c", 65, "A"},
		{"f", 3.14, "3.140000"},
		{".2f", 3.14159, "3.14"},
		{"e", 1000.0, "1.000000e+03"},
		{"g", 0.0001, "0.0001"},
		{"s", "hey", "hey"},
		{"p", "hey", `"hey"`}, // Ruby %p -> inspect-style quoting
		{"v", 42, "42"},       // default verb branch
	}
	for _, c := range cases {
		got := sprintfSpec(c.spec, c.v)
		if got != c.want {
			t.Errorf("sprintfSpec(%q, %v) = %q, want %q", c.spec, c.v, got, c.want)
		}
	}
}

func TestSprintfSpecStringTypes(t *testing.T) {
	// integer spec applied to string/float/int64 values via toInt.
	if got := sprintfSpec("d", "12"); got != "12" {
		t.Errorf("d string = %q", got)
	}
	if got := sprintfSpec("d", int64(9)); got != "9" {
		t.Errorf("d int64 = %q", got)
	}
	if got := sprintfSpec("d", 2.9); got != "2" {
		t.Errorf("d float = %q", got)
	}
	if got := sprintfSpec("d", true); got != "0" {
		t.Errorf("d bool = %q", got)
	}
	// float spec via toFloat over various types.
	if got := sprintfSpec(".1f", "2.5"); got != "2.5" {
		t.Errorf("f string = %q", got)
	}
	if got := sprintfSpec(".0f", 3); got != "3" {
		t.Errorf("f int = %q", got)
	}
	if got := sprintfSpec(".0f", int64(4)); got != "4" {
		t.Errorf("f int64 = %q", got)
	}
	if got := sprintfSpec(".0f", true); got != "0" {
		t.Errorf("f bool = %q", got)
	}
}

func TestRubyToS(t *testing.T) {
	cases := []struct {
		v    Value
		want string
	}{
		{nil, ""},
		{"s", "s"},
		{true, "true"},
		{42, "42"},
		{int64(7), "7"},
		{3.5, "3.5"},
		{[]Value{1}, "[1]"},
	}
	for _, c := range cases {
		if got := rubyToS(c.v); got != c.want {
			t.Errorf("rubyToS(%v) = %q, want %q", c.v, got, c.want)
		}
	}
}
