// Copyright (c) the go-ruby-i18n/i18n authors
//
// SPDX-License-Identifier: BSD-3-Clause

package i18n

import (
	"reflect"
	"testing"
)

// newEN builds an I18n with a representative English store covering the shapes
// the tests exercise.
func newEN(t *testing.T) *I18n {
	t.Helper()
	i := New("en")
	i.Backend().StoreTranslations("en", map[string]Value{
		"hello":    "Hello",
		"greeting": "Hello, %{name}!",
		"scoped":   map[string]Value{"msg": "scoped val"},
		"apples": map[string]Value{
			"zero":  "no apples",
			"one":   "one apple",
			"other": "%{count} apples",
		},
		"fmt": "%<count>d items",
		"yes": true,
		"num": 42,
		"arr": []Value{"a %{name}", "b"},
	})
	return i
}

func ip(n int) *int { return &n }

func TestTranslateBasic(t *testing.T) {
	i := newEN(t)
	cases := []struct {
		key  string
		opts *Options
		want Value
	}{
		{"hello", nil, "Hello"},
		{"scoped.msg", nil, "scoped val"},
		{"msg", &Options{Scope: []string{"scoped"}}, "scoped val"},
		{"yes", nil, true},
		{"num", nil, 42},
	}
	for _, c := range cases {
		got, err := i.Translate(c.key, c.opts)
		if err != nil {
			t.Fatalf("%s: %v", c.key, err)
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("t(%q) = %#v, want %#v", c.key, got, c.want)
		}
	}
}

func TestTranslateInterpolation(t *testing.T) {
	i := newEN(t)
	got, _ := i.Translate("greeting", &Options{Values: map[string]Value{"name": "World"}})
	if got != "Hello, World!" {
		t.Errorf("greeting = %q", got)
	}
	got, _ = i.Translate("fmt", &Options{Count: ip(5)})
	if got != "5 items" {
		t.Errorf("fmt = %q", got)
	}
	// Extra interpolation values are ignored.
	got, _ = i.Translate("greeting", &Options{Values: map[string]Value{"name": "X", "extra": "Y"}})
	if got != "Hello, X!" {
		t.Errorf("greeting extra = %q", got)
	}
	// Missing placeholder left literal (default handler).
	got, _ = i.Translate("greeting", &Options{Values: map[string]Value{"other": "z"}})
	if got != "Hello, %{name}!" {
		t.Errorf("greeting missing = %q", got)
	}
}

func TestTranslateArrayInterpolation(t *testing.T) {
	i := newEN(t)
	got, _ := i.Translate("arr", &Options{Values: map[string]Value{"name": "X"}})
	want := []Value{"a X", "b"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("arr = %#v, want %#v", got, want)
	}
}

func TestTranslatePluralization(t *testing.T) {
	i := newEN(t)
	cases := []struct {
		count int
		want  Value
	}{
		{0, "no apples"},
		{1, "one apple"},
		{2, "2 apples"},
	}
	for _, c := range cases {
		got, err := i.Translate("apples", &Options{Count: ip(c.count)})
		if err != nil {
			t.Fatalf("count %d: %v", c.count, err)
		}
		if got != c.want {
			t.Errorf("apples count=%d = %q, want %q", c.count, got, c.want)
		}
	}
}

func TestPluralizationZeroWithoutZeroKey(t *testing.T) {
	i := New("en")
	i.Backend().StoreTranslations("en", map[string]Value{
		"x": map[string]Value{"one": "one", "other": "%{count} other"},
	})
	got, _ := i.Translate("x", &Options{Count: ip(0)})
	if got != "0 other" {
		t.Errorf("count=0 no zero key = %q", got)
	}
}

func TestPluralizationMissingOtherRaises(t *testing.T) {
	i := New("en")
	i.Backend().StoreTranslations("en", map[string]Value{
		"x": map[string]Value{"one": "one"},
	})
	_, err := i.Translate("x", &Options{Count: ip(5)})
	ipd, ok := err.(*InvalidPluralizationData)
	if !ok {
		t.Fatalf("err = %T %v", err, err)
	}
	want := `translation data {one: "one"} can not be used with :count => 5. key 'other' is missing.`
	if ipd.Error() != want {
		t.Errorf("err = %q\nwant %q", ipd.Error(), want)
	}
}

func TestPluralizeNonHashEntry(t *testing.T) {
	i := newEN(t)
	// :count on a scalar key: pluralize returns the scalar, count exposed to interp.
	got, _ := i.Translate("hello", &Options{Count: ip(3)})
	if got != "Hello" {
		t.Errorf("scalar with count = %q", got)
	}
}

func TestPluralizeAttributesStripped(t *testing.T) {
	i := New("en")
	i.Backend().StoreTranslations("en", map[string]Value{
		"x": map[string]Value{"one": "one", "other": "many", "attributes": "ignored"},
	})
	got, _ := i.Translate("x", &Options{Count: ip(2)})
	if got != "many" {
		t.Errorf("attributes = %q", got)
	}
}

func TestTranslateMissing(t *testing.T) {
	i := newEN(t)
	got, _ := i.Translate("nope", nil)
	if got != "Translation missing: en.nope" {
		t.Errorf("missing = %q", got)
	}
	got, _ = i.Translate("a.b.c", nil)
	if got != "Translation missing: en.a.b.c" {
		t.Errorf("missing dotted = %q", got)
	}
	got, _ = i.Translate("nope", &Options{Scope: []string{"scoped"}})
	if got != "Translation missing: en.scoped.nope" {
		t.Errorf("missing scoped = %q", got)
	}
	// Lookup through a scalar (cur is not a Hash).
	got, _ = i.Translate("hello.deeper", nil)
	if got != "Translation missing: en.hello.deeper" {
		t.Errorf("missing through scalar = %q", got)
	}
}

func TestTranslateRaise(t *testing.T) {
	i := newEN(t)
	_, err := i.Translate("nope", &Options{Raise: true})
	mt, ok := err.(*MissingTranslation)
	if !ok {
		t.Fatalf("err = %T", err)
	}
	if mt.Message() != "Translation missing: en.nope" {
		t.Errorf("msg = %q", mt.Message())
	}
}

func TestTranslateDefault(t *testing.T) {
	i := newEN(t)
	got, _ := i.Translate("nope", &Options{Default: []DefaultEntry{Lit("fallback")}})
	if got != "fallback" {
		t.Errorf("default lit = %q", got)
	}
	got, _ = i.Translate("nope", &Options{Default: []DefaultEntry{Key("hello")}})
	if got != "Hello" {
		t.Errorf("default key = %q", got)
	}
	// Chain: first key missing, second hits.
	got, _ = i.Translate("nope", &Options{Default: []DefaultEntry{Key("also_nope"), Key("hello")}})
	if got != "Hello" {
		t.Errorf("default chain = %q", got)
	}
	// Chain: key missing then literal.
	got, _ = i.Translate("nope", &Options{Default: []DefaultEntry{Key("also_nope"), Lit("lit")}})
	if got != "lit" {
		t.Errorf("default chain lit = %q", got)
	}
	// Literal default is interpolated.
	got, _ = i.Translate("nope", &Options{
		Default: []DefaultEntry{Lit("hi %{name}")},
		Values:  map[string]Value{"name": "Z"},
	})
	if got != "hi Z" {
		t.Errorf("default lit interp = %q", got)
	}
}

func TestExists(t *testing.T) {
	i := newEN(t)
	if !i.Exists("hello") {
		t.Error("hello should exist")
	}
	if i.Exists("nope") {
		t.Error("nope should not exist")
	}
	if !i.Exists("scoped.msg") {
		t.Error("scoped.msg should exist")
	}
	if !i.Exists("hello", "en") {
		t.Error("hello en should exist")
	}
	if i.Exists("hello", "fr") {
		t.Error("hello fr should not exist")
	}
}

func TestLocaleAccessors(t *testing.T) {
	i := New("en")
	if i.Locale() != "en" || i.DefaultLocale() != "en" {
		t.Fatal("initial locale")
	}
	i.SetLocale("fr")
	if i.Locale() != "fr" {
		t.Error("SetLocale")
	}
	i.SetLocale("")
	if i.Locale() != "en" {
		t.Error("empty locale falls back to default")
	}
	i.SetDefaultLocale("de")
	if i.DefaultLocale() != "de" {
		t.Error("SetDefaultLocale")
	}
}

func TestAvailableLocales(t *testing.T) {
	i := New("en")
	i.Backend().StoreTranslations("fr", map[string]Value{"x": "y"})
	i.Backend().StoreTranslations("en", map[string]Value{"x": "y"})
	got := i.AvailableLocales()
	if !reflect.DeepEqual(got, []string{"en", "fr"}) {
		t.Errorf("available = %#v", got)
	}
}

func TestDeepMergeStore(t *testing.T) {
	i := New("en")
	i.Backend().StoreTranslations("en", map[string]Value{"deep": map[string]Value{"a": "1"}})
	i.Backend().StoreTranslations("en", map[string]Value{"deep": map[string]Value{"b": "2"}})
	a, _ := i.Translate("deep.a", nil)
	b, _ := i.Translate("deep.b", nil)
	if a != "1" || b != "2" {
		t.Errorf("deep merge a=%v b=%v", a, b)
	}
	// Overwrite scalar-over-hash and hash-over-scalar branches.
	i.Backend().StoreTranslations("en", map[string]Value{"deep": "scalar"})
	v, _ := i.Translate("deep", nil)
	if v != "scalar" {
		t.Errorf("overwrite hash with scalar = %v", v)
	}
}

func TestFallbackLocales(t *testing.T) {
	i := New("en")
	i.Backend().StoreTranslations("en", map[string]Value{"only_en": "english", "shared": "en-shared"})
	i.Backend().StoreTranslations("fr", map[string]Value{"shared": "fr-shared"})
	i.SetLocale("fr")
	// only_en missing in fr -> default locale en.
	v, _ := i.Translate("only_en", nil)
	if v != "english" {
		t.Errorf("fallback to default = %v", v)
	}
	v, _ = i.Translate("shared", nil)
	if v != "fr-shared" {
		t.Errorf("present in fr = %v", v)
	}
	v, _ = i.Translate("nope", nil)
	if v != "Translation missing: fr.nope" {
		t.Errorf("missing reports primary locale = %v", v)
	}
}

func TestExplicitFallbackChain(t *testing.T) {
	i := New("en")
	i.Backend().StoreTranslations("en", map[string]Value{"a": "en-a"})
	i.Backend().StoreTranslations("de", map[string]Value{"a": "de-a", "b": "de-b"})
	i.SetFallbacks("de-AT", "de", "en")
	v, _ := i.Translate("b", &Options{Locale: "de-AT"})
	if v != "de-b" {
		t.Errorf("explicit chain = %v", v)
	}
	v, _ = i.Translate("a", &Options{Locale: "de-AT"})
	if v != "de-a" {
		t.Errorf("explicit chain a = %v", v)
	}
}

func TestLocaleOption(t *testing.T) {
	i := New("en")
	i.Backend().StoreTranslations("fr", map[string]Value{"hi": "Bonjour"})
	v, _ := i.Translate("hi", &Options{Locale: "fr"})
	if v != "Bonjour" {
		t.Errorf("locale option = %v", v)
	}
}

func TestTAlias(t *testing.T) {
	i := newEN(t)
	v, _ := i.T("hello", nil)
	if v != "Hello" {
		t.Errorf("T alias = %v", v)
	}
}

func TestAsString(t *testing.T) {
	if AsString("x") != "x" {
		t.Error("string")
	}
	if AsString(nil) != "" {
		t.Error("nil")
	}
	if AsString(42) != "42" {
		t.Error("int")
	}
}

func TestDefaultKeyMissingFallsThroughToMissing(t *testing.T) {
	i := newEN(t)
	// All defaults are keys that don't exist -> final missing message.
	v, _ := i.Translate("nope", &Options{Default: []DefaultEntry{Key("also_nope")}})
	if v != "Translation missing: en.nope" {
		t.Errorf("all-key default miss = %v", v)
	}
}

func TestPluralRuleSlavicAndRegister(t *testing.T) {
	i := New("ru")
	i.Backend().StoreTranslations("ru", map[string]Value{
		"items": map[string]Value{
			"one":   "%{count} штука",
			"few":   "%{count} штуки",
			"many":  "%{count} штук",
			"other": "%{count} штука",
		},
	})
	cases := map[int]string{1: "1 штука", 2: "2 штуки", 5: "5 штук", 21: "21 штука"}
	for n, want := range cases {
		got, err := i.Translate("items", &Options{Count: ip(n)})
		if err != nil {
			t.Fatalf("ru %d: %v", n, err)
		}
		if got != want {
			t.Errorf("ru count=%d = %q want %q", n, got, want)
		}
	}
	// Register a custom rule overriding the table.
	i.Backend().RegisterPluralRule("ru", func(int) string { return "other" })
	got, _ := i.Translate("items", &Options{Count: ip(2)})
	if got != "2 штука" {
		t.Errorf("custom rule = %q", got)
	}
}
