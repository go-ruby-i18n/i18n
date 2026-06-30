// Copyright (c) the go-ruby-i18n/i18n authors
//
// SPDX-License-Identifier: BSD-3-Clause

package i18n

import (
	"math/big"
	"reflect"
	"testing"
)

func TestNormalizeShapes(t *testing.T) {
	in := map[string]any{
		"s":    "str",
		"n":    42,
		"b":    true,
		"f":    3.5,
		"nil":  nil,
		"arr":  []any{1, "two", []any{3}},
		"sub":  map[string]any{"k": "v"},
		"vsub": map[string]Value{"k": "v"},
		"varr": []Value{1, 2},
		"big":  big.NewInt(7), // default branch: passed through untouched
	}
	out := normalize(in).(map[string]Value)
	if out["s"] != "str" || out["n"] != 42 || out["b"] != true || out["f"] != 3.5 || out["nil"] != nil {
		t.Error("scalars")
	}
	if !reflect.DeepEqual(out["arr"], []Value{1, "two", []Value{3}}) {
		t.Errorf("arr = %#v", out["arr"])
	}
	if !reflect.DeepEqual(out["sub"], map[string]Value{"k": "v"}) {
		t.Errorf("sub = %#v", out["sub"])
	}
	if !reflect.DeepEqual(out["vsub"], map[string]Value{"k": "v"}) {
		t.Errorf("vsub = %#v", out["vsub"])
	}
	if !reflect.DeepEqual(out["varr"], []Value{1, 2}) {
		t.Errorf("varr = %#v", out["varr"])
	}
	if _, ok := out["big"].(*big.Int); !ok {
		t.Errorf("big = %#v", out["big"])
	}
}

func TestStoreTranslationsGenericMaps(t *testing.T) {
	i := New("en")
	// Store a sub-tree expressed with map[string]any/[]any from a generic loader.
	i.Backend().translations["en"] = nil // exercise the nil-init path explicitly
	i.Backend().StoreTranslations("en", map[string]Value{
		"a": map[string]any{"b": "c"},
	})
	v, _ := i.Translate("a.b", nil)
	if v != "c" {
		t.Errorf("generic map store = %v", v)
	}
}

func TestSplitDotsEmpty(t *testing.T) {
	if splitDots("") != nil {
		t.Error("empty splitDots should be nil")
	}
	got := splitDots("a.b")
	if !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Errorf("splitDots = %#v", got)
	}
}

func TestInspectValueShapes(t *testing.T) {
	cases := []struct {
		v    Value
		want string
	}{
		{"x", `"x"`},
		{nil, "nil"},
		{true, "true"},
		{42, "42"},
		{int64(7), "7"},
		{3.5, "3.5"},
		{[]Value{1}, "[1]"},
	}
	for _, c := range cases {
		if got := inspectValue(c.v); got != c.want {
			t.Errorf("inspectValue(%v) = %q, want %q", c.v, got, c.want)
		}
	}
}

func TestInspectEntryAndValuesSorted(t *testing.T) {
	m := map[string]Value{"one": "1", "other": "x"}
	if got := inspectEntry(m); got != `{one: "1", other: "x"}` {
		t.Errorf("inspectEntry = %q", got)
	}
	if got := inspectValues(m); got != `{one: "1", other: "x"}` {
		t.Errorf("inspectValues = %q", got)
	}
}

func TestErrorMessages(t *testing.T) {
	mia := &MissingInterpolationArgument{Key: "name", Values: map[string]Value{"x": 1}, String: "hi %{name}"}
	if mia.Error() == "" {
		t.Error("mia empty")
	}
	rk := &ReservedInterpolationKey{Key: "scope", String: "%{scope}"}
	if rk.Error() == "" {
		t.Error("rk empty")
	}
	ipd := &InvalidPluralizationData{Entry: map[string]Value{"one": "1"}, Count: 5, Key: "other"}
	if ipd.Error() == "" {
		t.Error("ipd empty")
	}
	mt := &MissingTranslation{Locale: "en", Key: "a.b", Scope: []string{"s"}}
	if mt.Error() != "Translation missing: en.s.a.b" {
		t.Errorf("mt = %q", mt.Error())
	}
	// Empty key path.
	mt2 := &MissingTranslation{Locale: "en", Key: ""}
	if mt2.Message() != "Translation missing: en" {
		t.Errorf("mt2 = %q", mt2.Message())
	}
}

func TestSetMissingInterpolationHandler(t *testing.T) {
	i := New("en")
	i.Backend().StoreTranslations("en", map[string]Value{"g": "hi %{name}"})
	i.SetMissingInterpolationHandler(RaiseMissingInterpolation)
	_, err := i.Translate("g", &Options{Values: map[string]Value{"x": "y"}})
	if _, ok := err.(*MissingInterpolationArgument); !ok {
		t.Fatalf("err = %T %v", err, err)
	}
	// Reset to default (nil -> keep literal).
	i.SetMissingInterpolationHandler(nil)
	v, err := i.Translate("g", &Options{Values: map[string]Value{"x": "y"}})
	if err != nil || v != "hi %{name}" {
		t.Errorf("reset handler v=%v err=%v", v, err)
	}
}

func TestInterpolateResultPropagatesError(t *testing.T) {
	i := New("en")
	i.SetMissingInterpolationHandler(RaiseMissingInterpolation)
	i.Backend().StoreTranslations("en", map[string]Value{
		"arr": []Value{"ok %{name}"},
	})
	// Array element interpolation hits the raising handler -> error bubbles up.
	_, err := i.Translate("arr", &Options{Values: map[string]Value{"x": "y"}})
	if _, ok := err.(*MissingInterpolationArgument); !ok {
		t.Fatalf("array interp err = %T %v", err, err)
	}
}

func TestInterpolateResultNonStringNode(t *testing.T) {
	// Resolving to a Hash/scalar while interpolation values are present hits the
	// default (non-string, non-array) branch: the node is returned untouched.
	i := New("en")
	i.Backend().StoreTranslations("en", map[string]Value{
		"h": map[string]Value{"a": "1"},
		"n": 42,
	})
	got, _ := i.Translate("h", &Options{Values: map[string]Value{"name": "X"}})
	if !reflect.DeepEqual(got, map[string]Value{"a": "1"}) {
		t.Errorf("hash node = %#v", got)
	}
	got, _ = i.Translate("n", &Options{Values: map[string]Value{"name": "X"}})
	if got != 42 {
		t.Errorf("int node = %v", got)
	}
}

func TestResolveReservedKeyError(t *testing.T) {
	i := New("en")
	i.Backend().StoreTranslations("en", map[string]Value{"r": "x %{scope} y"})
	_, err := i.Translate("r", &Options{Values: map[string]Value{"scope": "S"}})
	if _, ok := err.(*ReservedInterpolationKey); !ok {
		t.Fatalf("reserved err = %T %v", err, err)
	}
}

func TestDefaultKeyPluralizeError(t *testing.T) {
	// A :default key that pluralizes to a missing form propagates the error.
	i := New("en")
	i.Backend().StoreTranslations("en", map[string]Value{
		"d": map[string]Value{"one": "one"},
	})
	_, err := i.Translate("nope", &Options{Count: ip(5), Default: []DefaultEntry{Key("d")}})
	if _, ok := err.(*InvalidPluralizationData); !ok {
		t.Fatalf("default pluralize err = %T %v", err, err)
	}
}

func TestTranslatePluralizeErrorPrimary(t *testing.T) {
	i := New("en")
	i.Backend().StoreTranslations("en", map[string]Value{"x": map[string]Value{"one": "1"}})
	_, err := i.Translate("x", &Options{Count: ip(2)})
	if _, ok := err.(*InvalidPluralizationData); !ok {
		t.Fatalf("err = %T", err)
	}
}
