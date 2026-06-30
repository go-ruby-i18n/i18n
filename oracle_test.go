// Copyright (c) the go-ruby-i18n/i18n authors
//
// SPDX-License-Identifier: BSD-3-Clause

package i18n

import (
	"os/exec"
	"strings"
	"testing"
	"time"
)

// rubyI18n locates a `ruby` that can `require "i18n"`. The oracle tests skip
// themselves when it is absent (the Windows lane, the qemu cross-arch lanes, and
// any host without the gem), so the deterministic suite alone drives the 100%
// coverage gate there. The CI ubuntu/macos lanes `gem install i18n` so the MRI
// oracle actually runs.
func rubyI18n(t *testing.T) string {
	t.Helper()
	bin, err := exec.LookPath("ruby")
	if err != nil {
		t.Skip("ruby not on PATH; skipping i18n-gem oracle")
	}
	// Confirm the gem loads.
	if err := exec.Command(bin, "-e", "require 'i18n'").Run(); err != nil {
		t.Skip("i18n gem not installed; skipping oracle")
	}
	return bin
}

// runRuby executes a Ruby script (with a shared preamble that sets up a Simple
// backend) and returns its trimmed stdout. The script $stdout.binmode's itself so
// Windows text-mode never mangles the bytes.
func runRuby(t *testing.T, bin, body string) string {
	t.Helper()
	const preamble = `
$stdout.binmode
require 'i18n'
require 'date'
I18n.enforce_available_locales = false
I18n.backend = I18n::Backend::Simple.new
`
	cmd := exec.Command(bin, "-e", preamble+body)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ruby error: %v\nbody:\n%s\noutput:\n%s", err, body, out)
	}
	return strings.TrimRight(string(out), "\n")
}

// oracleEN is the translation tree both sides load: the Go store here and the
// Ruby store via store_translations in each oracle script's setup.
const oracleStoreRuby = `
I18n.backend.store_translations(:en, {
  hello: "Hello",
  greeting: "Hello, %{name}!",
  fmt: "%<count>d items",
  scoped: { msg: "scoped val" },
  apples: { zero: "no apples", one: "one apple", other: "%{count} apples" },
})
I18n.locale = :en
`

func oracleStoreGo() *I18n {
	i := New("en")
	i.Backend().StoreTranslations("en", map[string]Value{
		"hello":    "Hello",
		"greeting": "Hello, %{name}!",
		"fmt":      "%<count>d items",
		"scoped":   map[string]Value{"msg": "scoped val"},
		"apples": map[string]Value{
			"zero":  "no apples",
			"one":   "one apple",
			"other": "%{count} apples",
		},
	})
	return i
}

func TestOracleTranslate(t *testing.T) {
	bin := rubyI18n(t)
	i := oracleStoreGo()

	type tc struct {
		name string
		rb   string // ruby expression producing the value (after store)
		got  func() Value
	}
	cases := []tc{
		{"basic", `I18n.t(:hello)`, func() Value { v, _ := i.Translate("hello", nil); return v }},
		{"dotted", `I18n.t("scoped.msg")`, func() Value { v, _ := i.Translate("scoped.msg", nil); return v }},
		{"scope", `I18n.t(:msg, scope: :scoped)`, func() Value {
			v, _ := i.Translate("msg", &Options{Scope: []string{"scoped"}})
			return v
		}},
		{"interp", `I18n.t(:greeting, name: "World")`, func() Value {
			v, _ := i.Translate("greeting", &Options{Values: map[string]Value{"name": "World"}})
			return v
		}},
		{"sprintf", `I18n.t(:fmt, count: 5)`, func() Value {
			v, _ := i.Translate("fmt", &Options{Count: ip(5)})
			return v
		}},
		{"missing-arg", `I18n.t(:greeting)`, func() Value {
			v, _ := i.Translate("greeting", nil)
			return v
		}},
		{"plural0", `I18n.t(:apples, count: 0)`, func() Value {
			v, _ := i.Translate("apples", &Options{Count: ip(0)})
			return v
		}},
		{"plural1", `I18n.t(:apples, count: 1)`, func() Value {
			v, _ := i.Translate("apples", &Options{Count: ip(1)})
			return v
		}},
		{"pluralN", `I18n.t(:apples, count: 9)`, func() Value {
			v, _ := i.Translate("apples", &Options{Count: ip(9)})
			return v
		}},
		{"missing", `I18n.t(:nope)`, func() Value { v, _ := i.Translate("nope", nil); return v }},
		{"missing-dotted", `I18n.t("a.b.c")`, func() Value { v, _ := i.Translate("a.b.c", nil); return v }},
		{"missing-scope", `I18n.t(:nope, scope: :scoped)`, func() Value {
			v, _ := i.Translate("nope", &Options{Scope: []string{"scoped"}})
			return v
		}},
		{"default-lit", `I18n.t(:nope, default: "fallback")`, func() Value {
			v, _ := i.Translate("nope", &Options{Default: []DefaultEntry{Lit("fallback")}})
			return v
		}},
		{"default-key", `I18n.t(:nope, default: :hello)`, func() Value {
			v, _ := i.Translate("nope", &Options{Default: []DefaultEntry{Key("hello")}})
			return v
		}},
		{"default-chain", `I18n.t(:nope, default: [:also_nope, :hello])`, func() Value {
			v, _ := i.Translate("nope", &Options{Default: []DefaultEntry{Key("also_nope"), Key("hello")}})
			return v
		}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			want := runRuby(t, bin, oracleStoreRuby+"print "+c.rb+".to_s")
			got := AsString(c.got())
			if got != want {
				t.Errorf("%s: go=%q ruby=%q", c.name, got, want)
			}
		})
	}
}

func TestOracleExists(t *testing.T) {
	bin := rubyI18n(t)
	i := oracleStoreGo()
	cases := map[string]string{
		"hello":      `I18n.exists?(:hello)`,
		"nope":       `I18n.exists?(:nope)`,
		"scoped.msg": `I18n.exists?("scoped.msg")`,
	}
	for key, rb := range cases {
		want := runRuby(t, bin, oracleStoreRuby+"print ("+rb+").to_s")
		got := "false"
		if i.Exists(key) {
			got = "true"
		}
		if got != want {
			t.Errorf("exists?(%q): go=%q ruby=%q", key, got, want)
		}
	}
}

func TestOraclePluralizationError(t *testing.T) {
	bin := rubyI18n(t)
	i := New("en")
	i.Backend().StoreTranslations("en", map[string]Value{"x": map[string]Value{"one": "one"}})

	rbBody := `
I18n.backend.store_translations(:en, { x: { one: "one" } })
I18n.locale = :en
begin
  I18n.t(:x, count: 5)
rescue I18n::InvalidPluralizationData => e
  print e.message
end
`
	want := runRuby(t, bin, rbBody)
	_, err := i.Translate("x", &Options{Count: ip(5)})
	if err == nil || err.Error() != want {
		t.Errorf("plural error: go=%v ruby=%q", err, want)
	}
}

// oracleLocalizeStore is the date/time tree both sides load for localize parity.
const oracleLocalizeRuby = `
I18n.backend.store_translations(:en, {
  date: {
    formats: { default: "%Y-%m-%d", long: "%B %d, %Y", short: "%b %d" },
    day_names: %w[Sunday Monday Tuesday Wednesday Thursday Friday Saturday],
    abbr_day_names: %w[Sun Mon Tue Wed Thu Fri Sat],
    month_names: [nil] + %w[January February March April May June July August September October November December],
    abbr_month_names: [nil] + %w[Jan Feb Mar Apr May Jun Jul Aug Sep Oct Nov Dec],
  },
  time: {
    formats: { default: "%a, %d %b %Y %H:%M:%S", short: "%d %b %H:%M" },
    am: "am", pm: "pm",
  },
})
I18n.locale = :en
`

func oracleLocalizeGo() *I18n {
	i := New("en")
	i.Backend().StoreTranslations("en", map[string]Value{
		"date": map[string]Value{
			"formats": map[string]Value{"default": "%Y-%m-%d", "long": "%B %d, %Y", "short": "%b %d"},
			"day_names": []Value{"Sunday", "Monday", "Tuesday", "Wednesday",
				"Thursday", "Friday", "Saturday"},
			"abbr_day_names":   []Value{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"},
			"month_names":      []Value{nil, "January", "February", "March", "April", "May", "June", "July", "August", "September", "October", "November", "December"},
			"abbr_month_names": []Value{nil, "Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"},
		},
		"time": map[string]Value{
			"formats": map[string]Value{"default": "%a, %d %b %Y %H:%M:%S", "short": "%d %b %H:%M"},
			"am":      "am", "pm": "pm",
		},
	})
	return i
}

func TestOracleLocalize(t *testing.T) {
	bin := rubyI18n(t)
	i := oracleLocalizeGo()

	d := DateValue{T: time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC)}
	tm := TimeValue{T: time.Date(2026, 6, 30, 14, 5, 9, 0, time.UTC)}

	dateRb := `d = Date.new(2026, 6, 30)`
	timeRb := `t = Time.utc(2026, 6, 30, 14, 5, 9)`

	type tc struct {
		name string
		rb   string
		got  string
	}
	mustL := func(obj Temporal, f string, named bool) string {
		s, err := i.Localize(obj, f, named, nil)
		if err != nil {
			t.Fatalf("localize %q: %v", f, err)
		}
		return s
	}
	cases := []tc{
		{"date-default", dateRb + `; print I18n.l(d)`, mustL(d, "default", true)},
		{"date-long", dateRb + `; print I18n.l(d, format: :long)`, mustL(d, "long", true)},
		{"date-short", dateRb + `; print I18n.l(d, format: :short)`, mustL(d, "short", true)},
		{"date-names", dateRb + `; print I18n.l(d, format: "%A %^B %a %b")`, mustL(d, "%A %^B %a %b", false)},
		{"time-default", timeRb + `; print I18n.l(t)`, mustL(tm, "default", true)},
		{"time-short", timeRb + `; print I18n.l(t, format: :short)`, mustL(tm, "short", true)},
		{"time-meridian", timeRb + `; print I18n.l(t, format: "%p %P %I:%M")`, mustL(tm, "%p %P %I:%M", false)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			want := runRuby(t, bin, oracleLocalizeRuby+c.rb)
			if c.got != want {
				t.Errorf("%s: go=%q ruby=%q", c.name, c.got, want)
			}
		})
	}
}

func TestOraclePluralRulesAgainstSimpleBackend(t *testing.T) {
	// The Simple backend (no Pluralization mixin) always uses zero/one/other; this
	// asserts our default rule matches it across a counts sweep so the headline
	// pluralization path is byte-identical to a stock i18n install.
	bin := rubyI18n(t)
	i := oracleStoreGo()
	for _, n := range []int{0, 1, 2, 5, 21, 100} {
		rb := oracleStoreRuby + "print I18n.t(:apples, count: " + itoa(n) + ")"
		want := runRuby(t, bin, rb)
		v, _ := i.Translate("apples", &Options{Count: ip(n)})
		if AsString(v) != want {
			t.Errorf("count=%d: go=%q ruby=%q", n, AsString(v), want)
		}
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	pos := len(buf)
	for n > 0 {
		pos--
		buf[pos] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}
