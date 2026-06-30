// Copyright (c) the go-ruby-i18n/i18n authors
//
// SPDX-License-Identifier: BSD-3-Clause

// Package i18n is a pure-Go (no cgo) port of Ruby's I18n core — the
// interpreter-independent translate/localize logic of the `i18n` gem's
// I18n::Backend::Simple. It does dotted-key lookup over an in-memory translation
// store, %{name} / %<count>d interpolation, :count pluralization, :default and
// fallback-locale chains, the "Translation missing: …" behaviour, :scope, and
// strftime-style localization of a host-supplied Date/Time — matching the gem
// byte-for-byte.
//
// Loading translation *files* (YAML/Ruby) and constructing Date/Time values are
// seams the host (go-embedded-ruby / rbgo) fills: it parses a file to a nested
// Hash and calls StoreTranslations, and it hands Localize a Temporal. Everything
// the gem computes deterministically lives here, with no Ruby runtime.
package i18n

import "strings"

// I18n bundles the mutable global state the gem keeps in I18n.config: the active
// backend, the current and default locale, and the fallback chain. The package
// exposes a default instance through the top-level functions (I18n.t etc.), and
// New lets a host hold an isolated one.
type I18n struct {
	backend       *Simple
	locale        string
	defaultLocale string
	// fallbacks maps a locale to the chain consulted when a key is missing
	// (I18n.fallbacks). A locale absent here falls back to the default locale.
	fallbacks map[string][]string
	// missingInterpolation decides the substitution for an absent placeholder.
	missingInterpolation MissingInterpolationHandler
}

// New returns an I18n with an empty Simple backend and the given default locale,
// which is also the initial current locale.
func New(defaultLocale string) *I18n {
	return &I18n{
		backend:              NewSimple(),
		locale:               defaultLocale,
		defaultLocale:        defaultLocale,
		fallbacks:            map[string][]string{},
		missingInterpolation: keepMissingInterpolation,
	}
}

// Backend returns the underlying Simple store so a host can load translations
// into it.
func (i *I18n) Backend() *Simple { return i.backend }

// Locale returns the current locale (I18n.locale).
func (i *I18n) Locale() string {
	if i.locale == "" {
		return i.defaultLocale
	}
	return i.locale
}

// SetLocale sets the current locale (I18n.locale=).
func (i *I18n) SetLocale(l string) { i.locale = l }

// DefaultLocale returns the default locale (I18n.default_locale).
func (i *I18n) DefaultLocale() string { return i.defaultLocale }

// SetDefaultLocale sets the default locale (I18n.default_locale=).
func (i *I18n) SetDefaultLocale(l string) { i.defaultLocale = l }

// AvailableLocales returns the locales with stored translations.
func (i *I18n) AvailableLocales() []string { return i.backend.AvailableLocales() }

// SetFallbacks sets the fallback chain for a locale (the list of locales tried,
// in order, when a key is missing in it). The locale itself need not be listed;
// it is always tried first.
func (i *I18n) SetFallbacks(locale string, chain ...string) {
	i.fallbacks[locale] = chain
}

// SetMissingInterpolationHandler installs the handler used when a %{...}
// placeholder has no value. The default keeps the placeholder literal.
func (i *I18n) SetMissingInterpolationHandler(h MissingInterpolationHandler) {
	if h == nil {
		h = keepMissingInterpolation
	}
	i.missingInterpolation = h
}

// fallbackChain returns the ordered list of locales to try for a lookup in
// locale: the locale itself, then its configured chain, then the default locale,
// de-duplicated.
func (i *I18n) fallbackChain(locale string) []string {
	seen := map[string]bool{}
	var chain []string
	add := func(l string) {
		if l != "" && !seen[l] {
			seen[l] = true
			chain = append(chain, l)
		}
	}
	add(locale)
	for _, l := range i.fallbacks[locale] {
		add(l)
	}
	add(i.defaultLocale)
	return chain
}

// Options configures a single Translate / Localize call, mirroring the gem's
// keyword options (:scope, :default, :count, :locale, interpolation values…).
type Options struct {
	// Locale overrides the current locale for this call (:locale).
	Locale string
	// Scope is a key prefix prepended to the lookup (:scope); each entry may be
	// dotted and is split on ".".
	Scope []string
	// Count drives pluralization and is also exposed to interpolation as
	// "count" (:count). Use a non-nil *int.
	Count *int
	// Default is the :default chain: each entry is tried in order. A string that
	// names no key is returned literally; a *Symbol entry is looked up as a key.
	Default []DefaultEntry
	// Values are the interpolation variables (the remaining keyword args).
	Values map[string]Value
	// Raise makes a missing translation return a *MissingTranslation error
	// instead of the "Translation missing: …" string (:raise / I18n.t!).
	Raise bool
}

// DefaultEntry is one element of a :default chain: either a literal string to
// return as-is, or a key to look up (Key set). Use Lit / Key helpers.
type DefaultEntry struct {
	Literal string
	IsKey   bool
	KeyName string
}

// Lit builds a literal default (returned verbatim if reached).
func Lit(s string) DefaultEntry { return DefaultEntry{Literal: s} }

// Key builds a symbol default (looked up as a translation key if reached).
func Key(k string) DefaultEntry { return DefaultEntry{IsKey: true, KeyName: k} }

// Translate looks up key and returns its translation, applying scope, count
// pluralization, interpolation, :default chains and fallback locales — the
// behaviour of I18n.translate / I18n.t.
//
// A missing key yields the gem's "Translation missing: <locale>.<key>" string,
// unless opts.Raise is set, in which case a *MissingTranslation is returned.
func (i *I18n) Translate(key string, opts *Options) (Value, error) {
	if opts == nil {
		opts = &Options{}
	}
	locale := opts.Locale
	if locale == "" {
		locale = i.Locale()
	}

	values := i.interpolationValues(opts)

	// Try the key across the fallback chain.
	for _, loc := range i.fallbackChain(locale) {
		val, found, err := i.resolve(loc, key, opts.Scope, opts.Count, values)
		if err != nil {
			return nil, err
		}
		if found {
			return val, nil
		}
	}

	// Key missing everywhere — walk the :default chain in the primary locale.
	for _, d := range opts.Default {
		if !d.IsKey {
			// A literal default is still interpolated.
			return i.interpolateResult(locale, d.Literal, values)
		}
		for _, loc := range i.fallbackChain(locale) {
			val, found, err := i.resolve(loc, d.KeyName, opts.Scope, opts.Count, values)
			if err != nil {
				return nil, err
			}
			if found {
				return val, nil
			}
		}
	}

	// Truly missing.
	miss := &MissingTranslation{Locale: locale, Key: key, Scope: scopeForMessage(opts.Scope)}
	if opts.Raise {
		return nil, miss
	}
	return miss.Message(), nil
}

// resolve performs a single-locale lookup + pluralize + interpolate. found is
// false when the key is absent in that locale.
func (i *I18n) resolve(locale, key string, scope []string, count *int, values map[string]Value) (Value, bool, error) {
	segments := splitKey(scope, key)
	node, ok := i.backend.lookup(locale, segments)
	if !ok {
		return nil, false, nil
	}
	if count != nil {
		p, err := i.backend.pluralize(locale, node, *count)
		if err != nil {
			return nil, false, err
		}
		node = p
	}
	out, err := i.interpolateResult(locale, node, values)
	if err != nil {
		return nil, false, err
	}
	return out, true, nil
}

// interpolationValues assembles the values map: the explicit Values plus a
// "count" entry when Count is set (so %{count} works alongside pluralization).
func (i *I18n) interpolationValues(opts *Options) map[string]Value {
	if opts.Count == nil && len(opts.Values) == 0 {
		return nil
	}
	out := make(map[string]Value, len(opts.Values)+1)
	for k, v := range opts.Values {
		out[k] = v
	}
	if opts.Count != nil {
		if _, has := out["count"]; !has {
			out["count"] = *opts.Count
		}
	}
	return out
}

// interpolateResult interpolates a resolved node when it is a string (or, for an
// array/hash, recurses into string leaves), leaving non-string scalars untouched
// — matching I18n::Backend::Base#interpolate.
func (i *I18n) interpolateResult(locale string, node Value, values map[string]Value) (Value, error) {
	if len(values) == 0 {
		return node, nil
	}
	switch x := node.(type) {
	case string:
		return interpolate(x, values, i.missingInterpolation)
	case []Value:
		out := make([]Value, len(x))
		for k, v := range x {
			r, err := i.interpolateResult(locale, v, values)
			if err != nil {
				return nil, err
			}
			out[k] = r
		}
		return out, nil
	default:
		return node, nil
	}
}

// scopeForMessage flattens scope entries (each possibly dotted) for the missing
// message path.
func scopeForMessage(scope []string) []string {
	var out []string
	for _, s := range scope {
		out = append(out, splitDots(s)...)
	}
	return out
}

// Exists reports whether key resolves to a translation in locale (defaulting to
// the current locale), mirroring I18n.exists?. It does not consult :default or
// fallback locales — only the asked-for locale.
func (i *I18n) Exists(key string, locale ...string) bool {
	loc := i.Locale()
	if len(locale) > 0 && locale[0] != "" {
		loc = locale[0]
	}
	_, ok := i.backend.lookup(loc, splitDots(key))
	return ok
}

// T is a short alias for Translate.
func (i *I18n) T(key string, opts *Options) (Value, error) { return i.Translate(key, opts) }

// AsString coerces a resolved Value to text for hosts that always want a string
// from Translate (the gem's t always returns a String for a found scalar). It
// renders the same way Ruby's #to_s would for the translate result shapes.
func AsString(v Value) string {
	switch x := v.(type) {
	case string:
		return x
	case nil:
		return ""
	default:
		return strings.TrimSpace(rubyToS(x))
	}
}
