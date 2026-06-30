// Copyright (c) the go-ruby-i18n/i18n authors
//
// SPDX-License-Identifier: BSD-3-Clause

package i18n

import (
	"regexp"
	"strings"
)

// Localize formats a Temporal (a host-supplied Date/Time) for a locale, the way
// I18n.localize / I18n.l does:
//
//   - a named format (e.g. "long") is looked up under <type>.formats.<name>,
//     where <type> is "time" when the value carries a clock else "date"; an
//     unknown named format yields a *MissingTranslation (matching the gem, which
//     raises there);
//   - a literal format string is used as-is;
//   - %a/%A/%b/%B/%p/%P (and their %^ upcased variants) are substituted from the
//     locale's date.day_names / month_names / time.am|pm before the value is
//     strftime'd, exactly as I18n::Backend::Base#translate_localization_format.
//
// opts.Locale overrides the current locale; opts.Values supply nothing here but
// are accepted for symmetry.
func (i *I18n) Localize(obj Temporal, format string, named bool, opts *Options) (string, error) {
	if opts == nil {
		opts = &Options{}
	}
	locale := opts.Locale
	if locale == "" {
		locale = i.Locale()
	}

	typ := "date"
	if obj.HasTime() {
		typ = "time"
	}

	pattern := format
	if named {
		key := typ + ".formats." + format
		node, ok := i.backend.lookup(locale, splitDots(key))
		if !ok {
			// Fall back through the locale chain like translate does.
			for _, loc := range i.fallbackChain(locale) {
				if n, found := i.backend.lookup(loc, splitDots(key)); found {
					node = n
					ok = true
					break
				}
			}
		}
		if !ok {
			return "", &MissingTranslation{Locale: locale, Key: key}
		}
		s, isStr := node.(string)
		if !isStr {
			return "", &MissingTranslation{Locale: locale, Key: key}
		}
		pattern = s
	}

	resolved := i.translateLocalizationFormat(locale, obj, pattern)
	return strftime(obj, resolved), nil
}

// rxLocFormat matches the locale-aware name directives the gem pre-substitutes:
// %[^]a %[^]A %[^]b %[^]B %[^]p %[^]P.
var rxLocFormat = regexp.MustCompile(`%(\^?)([aAbBpP])`)

// translateLocalizationFormat substitutes the day/month/meridian name directives
// from the locale's translation data, leaving the rest of the format for
// strftime. A missing names table makes the gem return the format unchanged (it
// rescues MissingTranslationData); we mirror that by leaving the directive as-is,
// which lets strftime emit MRI's own English names.
func (i *I18n) translateLocalizationFormat(locale string, obj Temporal, format string) string {
	out := rxLocFormat.ReplaceAllStringFunc(format, func(m string) string {
		sub := rxLocFormat.FindStringSubmatch(m)
		up := sub[1] == "^"
		conv := sub[2]
		var name string
		var ok bool
		switch conv {
		case "a":
			name, ok = i.localeName(locale, "date.abbr_day_names", obj.Wday())
		case "A":
			name, ok = i.localeName(locale, "date.day_names", obj.Wday())
		case "b":
			name, ok = i.localeName(locale, "date.abbr_month_names", obj.Month())
		case "B":
			name, ok = i.localeName(locale, "date.month_names", obj.Month())
		case "p", "P":
			h := 0
			if obj.HasTime() {
				h = obj.Hour()
			}
			mer := "am"
			if h >= 12 {
				mer = "pm"
			}
			name, ok = i.localeScalar(locale, "time."+mer)
			if ok {
				if conv == "p" {
					name = strings.ToUpper(name)
				} else {
					name = strings.ToLower(name)
				}
				return name
			}
		}
		if !ok {
			return m // leave directive for strftime (English fallback)
		}
		if up {
			name = strings.ToUpper(name)
		}
		return name
	})
	return out
}

// localeName fetches the idx-th entry of a names array (day_names indexed by wday,
// month_names indexed by month with a nil 0th slot).
func (i *I18n) localeName(locale, key string, idx int) (string, bool) {
	for _, loc := range i.fallbackChain(locale) {
		node, ok := i.backend.lookup(loc, splitDots(key))
		if !ok {
			continue
		}
		arr, ok := node.([]Value)
		if !ok || idx < 0 || idx >= len(arr) {
			return "", false
		}
		s, ok := arr[idx].(string)
		return s, ok
	}
	return "", false
}

// localeScalar fetches a scalar string under key (used for time.am / time.pm).
func (i *I18n) localeScalar(locale, key string) (string, bool) {
	for _, loc := range i.fallbackChain(locale) {
		node, ok := i.backend.lookup(loc, splitDots(key))
		if !ok {
			continue
		}
		s, ok := node.(string)
		return s, ok
	}
	return "", false
}

// L is a short alias for Localize.
func (i *I18n) L(obj Temporal, format string, named bool, opts *Options) (string, error) {
	return i.Localize(obj, format, named, opts)
}
