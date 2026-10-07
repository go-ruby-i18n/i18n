<p align="center"><img src="https://raw.githubusercontent.com/go-ruby-i18n/brand/main/social/go-ruby-i18n-i18n.png" alt="go-ruby-i18n/i18n" width="720"></p>

# i18n — go-ruby-i18n

[![Docs](https://img.shields.io/badge/docs-mkdocs--material-DC2626)](https://go-ruby-i18n.github.io/docs/)
[![License](https://img.shields.io/badge/license-BSD--3--Clause-blue)](LICENSE)
[![Go](https://img.shields.io/badge/go-1.27.1%2B-00ADD8)](https://go.dev/dl/)
[![Coverage](https://img.shields.io/badge/coverage-100%25-1a7f37)](#tests--coverage)

**A pure-Go (no cgo) reimplementation of Ruby's [`i18n`](https://github.com/ruby-i18n/i18n)
gem core** — the interpreter-independent translate/localize logic of
`I18n::Backend::Simple`. It does dotted-key lookup over an in-memory translation
store, `%{name}` / `%<count>d` interpolation, `:count` pluralization, `:default`
and fallback-locale chains, the `Translation missing: …` behaviour, `:scope`, and
strftime-style localization of a host-supplied Date/Time — matching the `i18n`
gem **byte-for-byte** (validated by a differential MRI oracle).

It is the I18n backend for
[go-embedded-ruby](https://github.com/go-embedded-ruby/ruby), but is a
**standalone, reusable** module with no dependency on the Ruby runtime — a sibling
of [go-ruby-yaml](https://github.com/go-ruby-yaml/yaml), [go-ruby-erb](https://github.com/go-ruby-erb/erb)
and [go-ruby-regexp](https://github.com/go-ruby-regexp/regexp).

> **What it is — and isn't.** Looking a key up in a store, selecting a plural
> form, interpolating placeholders, walking a `:default` / fallback chain, and
> substituting a strftime pattern are fully deterministic and need **no
> interpreter**, so they live here as pure Go. Two things are *seams* the host
> fills: parsing translation **files** (YAML / Ruby / JSON) into the nested Hash
> the store holds, and constructing the **Date / Time** values `Localize`
> formats. rbgo loads the files and calls `StoreTranslations`, and hands
> `Localize` a `Temporal`; everything the gem computes deterministically is here.

## Features

Faithful port of `I18n.translate` / `I18n.localize`, validated against the real
`i18n` gem on every supported platform:

- **Dotted-key lookup** over a per-locale nested symbol-keyed store
  (`"foo.bar.baz"`), with `:scope` prefixing.
- **Interpolation** — `%{name}` (and the `%{name|word}` form), the literal `%%`,
  and sprintf-style `%<count>d` / `%<x>05.2f`; reserved-key collisions raise
  `ReservedInterpolationKey`; a missing placeholder is left literal by default
  (the gem's out-of-the-box behaviour) or raises via a configurable handler.
- **Pluralization** — `:count` selects `:zero` / `:one` / `:other` exactly as the
  Simple backend's `pluralization_key` does, with a missing form raising
  `InvalidPluralizationData` (message byte-identical to the gem). CLDR
  `:one`/`:few`/`:many`/`:zero`/`:two` rules for ru/uk/cs/sk/pl/ar/fr/pt ship in
  a registry, and any locale's rule is overridable.
- **Fallbacks** — `:default` chains (a literal string returned as-is, a symbol
  looked up as a key) and fallback-locale chains (`I18n.fallbacks`), always
  falling through to the default locale.
- **Missing-translation** — the `Translation missing: <locale>.<key>` string, or
  a `*MissingTranslation` error with `:raise` / the `T!`-style flag.
- **`I18n.localize`** — `<date|time>.formats.<name>` lookup plus strftime
  substitution, with `%a/%A/%b/%B/%p/%P` (and `%^` upcased variants) drawn from
  the locale's `day_names` / `month_names` / `am` / `pm` tables, and a built-in
  strftime covering the numeric / zone / fractional-second directives.
- **Store model** — `I18n::Backend::Simple`-style nested Hash with **deep-merge**
  of stored translations, plus `Locale` / `DefaultLocale` / `AvailableLocales`
  and `Exists`.

CGO-free, dependency-free, **100% test coverage**, `gofmt` + `go vet` clean, and
green across the six 64-bit Go targets (amd64, arm64, riscv64, loong64, ppc64le,
s390x) and three OSes (Linux, macOS, Windows).

## Install

```sh
go get github.com/go-ruby-i18n/i18n
```

## Usage

```go
package main

import (
	"fmt"
	"time"

	"github.com/go-ruby-i18n/i18n"
)

func main() {
	I := i18n.New("en") // default + current locale

	// The host loads a translation file and hands the nested Hash to the store
	// (the file-parsing seam). Here we store it directly.
	I.Backend().StoreTranslations("en", map[string]i18n.Value{
		"greeting": "Hello, %{name}!",
		"apples": map[string]i18n.Value{
			"zero":  "no apples",
			"one":   "one apple",
			"other": "%{count} apples",
		},
		"date": map[string]i18n.Value{
			"formats": map[string]i18n.Value{"long": "%B %d, %Y"},
			"month_names": []i18n.Value{nil, "January", "February", "March",
				"April", "May", "June", "July", "August", "September",
				"October", "November", "December"},
		},
	})

	g, _ := I.Translate("greeting", &i18n.Options{Values: map[string]i18n.Value{"name": "World"}})
	fmt.Println(g) // Hello, World!

	n := 2
	a, _ := I.Translate("apples", &i18n.Options{Count: &n})
	fmt.Println(a) // 2 apples

	miss, _ := I.Translate("nope", nil)
	fmt.Println(miss) // Translation missing: en.nope

	// Localize a host-supplied Date/Time (the clock-value seam). A Go time.Time
	// adapts through DateValue / TimeValue.
	d := i18n.DateValue{T: time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC)}
	l, _ := I.Localize(d, "long", true, nil) // named format -> date.formats.long
	fmt.Println(l)                           // June 30, 2026
}
```

## The seams

Two boundaries the host (rbgo) crosses; everything else is pure Go here.

| Seam | Host provides | This library does |
| ---- | ------------- | ----------------- |
| Translation **files** | parses YAML/Ruby/JSON to a nested `map[string]Value` and calls `StoreTranslations` (deep-merged) | lookup, interpolation, pluralization, fallback, missing |
| **Date / Time** values | a `Temporal` (Ruby `Date`/`Time`/`DateTime`); a Go `time.Time` adapts via `DateValue` / `TimeValue` | format lookup + locale name substitution + strftime |

## API

```go
type I18n struct{ /* … */ }
func New(defaultLocale string) *I18n

func (i *I18n) Translate(key string, opts *Options) (Value, error) // I18n.t
func (i *I18n) T(key string, opts *Options) (Value, error)         // alias
func (i *I18n) Localize(obj Temporal, format string, named bool, opts *Options) (string, error) // I18n.l
func (i *I18n) L(obj Temporal, format string, named bool, opts *Options) (string, error)        // alias
func (i *I18n) Exists(key string, locale ...string) bool           // I18n.exists?

func (i *I18n) Locale() string;        func (i *I18n) SetLocale(string)
func (i *I18n) DefaultLocale() string; func (i *I18n) SetDefaultLocale(string)
func (i *I18n) AvailableLocales() []string
func (i *I18n) SetFallbacks(locale string, chain ...string)
func (i *I18n) SetMissingInterpolationHandler(MissingInterpolationHandler)
func (i *I18n) Backend() *Simple

type Options struct {
	Locale  string
	Scope   []string
	Count   *int
	Default []DefaultEntry
	Values  map[string]Value
	Raise   bool
}
func Lit(string) DefaultEntry  // a literal default
func Key(string) DefaultEntry  // a symbol default (looked up)

type Simple struct{ /* … */ }
func NewSimple() *Simple
func (s *Simple) StoreTranslations(locale string, data map[string]Value) // deep-merge
func (s *Simple) RegisterPluralRule(locale string, rule PluralRule)
func (s *Simple) AvailableLocales() []string

type Temporal interface{ /* Year/Month/Day/Hour/…/HasTime */ }
type TimeValue struct{ T time.Time } // Temporal with a clock
type DateValue struct{ T time.Time } // Temporal without a clock

type MissingTranslation struct{ Locale, Key string; Scope []string }
type InvalidPluralizationData struct{ /* … */ }
type MissingInterpolationArgument struct{ /* … */ }
type ReservedInterpolationKey struct{ /* … */ }
```

## Tests & coverage

The suite pairs deterministic, ruby-free tests (which alone hold coverage at
100%, so the qemu cross-arch and Windows lanes pass the gate) with a
**differential MRI oracle**: each translate / pluralize / interpolate / fallback
/ missing / localize case is run through the real `i18n` gem and compared
byte-for-byte. The oracle scripts `$stdout.binmode` so Windows text-mode never
pollutes the bytes, and skip themselves where the gem / `ruby` is absent.

```sh
COVERPKG=$(go list ./... | paste -sd, -)
go test -race -coverpkg="$COVERPKG" -coverprofile=cover.out ./...
go tool cover -func=cover.out | tail -1   # 100.0%
```

## License

BSD-3-Clause — see [LICENSE](LICENSE). Copyright the go-ruby-i18n/i18n authors.

## WebAssembly

Being pure Go (CGO=0), this library also compiles to **WebAssembly** — both
`GOOS=js GOARCH=wasm` (browser / Node.js) and `GOOS=wasip1 GOARCH=wasm` (WASI).
CI builds both targets on every push, alongside the six 64-bit native/qemu arches.

```sh
GOOS=js     GOARCH=wasm go build ./...   # browser / Node
GOOS=wasip1 GOARCH=wasm go build ./...   # WASI (wasmtime, wasmer, wasmedge, …)
```
