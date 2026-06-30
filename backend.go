// Copyright (c) the go-ruby-i18n/i18n authors
//
// SPDX-License-Identifier: BSD-3-Clause

package i18n

import "sort"

// Simple is a port of I18n::Backend::Simple: the in-memory translation store
// (a per-locale nested symbol-keyed Hash) plus the lookup, pluralization and
// interpolation logic the default I18n backend runs over it. Loading translation
// *files* into the store is the host's job (the seam) — StoreTranslations takes
// the already-parsed nested data and deep-merges it, exactly as the gem's
// store_translations does after its YAML/Ruby loader returns.
type Simple struct {
	// translations maps locale name -> its symbol-keyed sub-tree.
	translations map[string]map[string]Value
	// pluralRules overrides the per-locale plural category rule; an absent locale
	// uses the built-in CLDR table, then the Simple zero/one/other rule.
	pluralRules map[string]PluralRule
}

// NewSimple returns an empty Simple backend.
func NewSimple() *Simple {
	return &Simple{
		translations: map[string]map[string]Value{},
		pluralRules:  map[string]PluralRule{},
	}
}

// StoreTranslations deep-merges data (a nested symbol-keyed tree for one locale)
// into the store. data may use map[string]any/[]any (a generic loader's output)
// or the canonical Value model; it is normalized on the way in. This is the
// file-loading seam: rbgo parses a YAML/Ruby file to this shape and calls here.
func (s *Simple) StoreTranslations(locale string, data map[string]Value) {
	n, _ := normalize(data).(map[string]Value)
	cur := s.translations[locale]
	if cur == nil {
		cur = map[string]Value{}
		s.translations[locale] = cur
	}
	deepMerge(cur, n)
}

// RegisterPluralRule sets the plural-category rule for a locale, overriding the
// built-in table. It lets a host install the rails-i18n pluralization data's rule
// for any locale.
func (s *Simple) RegisterPluralRule(locale string, rule PluralRule) {
	s.pluralRules[locale] = rule
}

// AvailableLocales returns the locales that have stored translations, sorted for
// determinism (I18n.available_locales).
func (s *Simple) AvailableLocales() []string {
	out := make([]string, 0, len(s.translations))
	for l := range s.translations {
		out = append(out, l)
	}
	sort.Strings(out)
	return out
}

// pluralRuleFor resolves the plural rule for a locale.
func (s *Simple) pluralRuleFor(locale string) PluralRule {
	if r, ok := s.pluralRules[locale]; ok {
		return r
	}
	if r, ok := builtinPluralRules[locale]; ok {
		return r
	}
	return simplePluralRule
}

// lookup resolves segments in a locale, returning the raw node and whether it was
// found. It does not interpolate or pluralize.
func (s *Simple) lookup(locale string, segments []string) (Value, bool) {
	root := s.translations[locale]
	if root == nil {
		return nil, false
	}
	return lookupNode(root, segments)
}

// pluralize selects the plural form of entry for count, mirroring
// I18n::Backend::Base#pluralize: a non-Hash entry (or absent count) is returned
// as-is; otherwise the rule picks a category, with :zero preferred when count==0
// and a :zero key exists, and a missing selected key raises
// InvalidPluralizationData.
func (s *Simple) pluralize(locale string, entry Value, count int) (Value, error) {
	m, ok := entry.(map[string]Value)
	if !ok {
		return entry, nil
	}
	// "attributes" is stripped before deciding (matches the gem).
	if _, has := m["attributes"]; has {
		filtered := make(map[string]Value, len(m))
		for k, v := range m {
			if k != "attributes" {
				filtered[k] = v
			}
		}
		m = filtered
	}
	key := s.pluralRuleFor(locale)(count)
	if count == 0 {
		if _, has := m["zero"]; has {
			key = "zero"
		}
	}
	val, has := m[key]
	if !has {
		return nil, &InvalidPluralizationData{Entry: m, Count: count, Key: key}
	}
	return val, nil
}
