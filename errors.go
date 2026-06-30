// Copyright (c) the go-ruby-i18n/i18n authors
//
// SPDX-License-Identifier: BSD-3-Clause

package i18n

import (
	"fmt"
	"strings"
)

// MissingTranslation is the error MRI's I18n raises (as
// I18n::MissingTranslationData / its MissingTranslation mixin) when a key does
// not resolve. Its Error() reproduces the gem's "Translation missing: <keys>"
// message verbatim, and Translate falls back to that same text (rather than
// raising) unless WithRaise / T! is used.
type MissingTranslation struct {
	Locale string   // the locale the lookup ran in
	Key    string   // the requested key (may be dotted)
	Scope  []string // any :scope prefix that was applied
}

// FullKey is the dotted "<locale>.<scope...>.<key>" path the gem prints.
func (e *MissingTranslation) keys() []string {
	parts := []string{e.Locale}
	parts = append(parts, e.Scope...)
	if e.Key != "" {
		parts = append(parts, strings.Split(e.Key, ".")...)
	}
	return parts
}

// Message returns the human string the gem substitutes for a missing key, e.g.
// "Translation missing: en.foo.bar". MRI capitalises the leading "Translation".
func (e *MissingTranslation) Message() string {
	return "Translation missing: " + strings.Join(e.keys(), ".")
}

func (e *MissingTranslation) Error() string { return e.Message() }

// InvalidPluralizationData mirrors I18n::InvalidPluralizationData: a :count was
// given but the entry lacks the plural key the rule selected (e.g. no :other).
type InvalidPluralizationData struct {
	Entry map[string]Value // the plural sub-hash
	Count int
	Key   string // the plural key that was expected but absent
}

func (e *InvalidPluralizationData) Error() string {
	return fmt.Sprintf("translation data %s can not be used with :count => %d. key '%s' is missing.",
		inspectEntry(e.Entry), e.Count, e.Key)
}

// MissingInterpolationArgument mirrors I18n::MissingInterpolationArgument: a
// placeholder in the string has no matching value. The Simple backend's default
// handler raises this; the default config installed here keeps the placeholder
// literal instead (matching the gem's out-of-the-box behaviour), so this is only
// produced when a raising handler is configured.
type MissingInterpolationArgument struct {
	Key    string
	Values map[string]Value
	String string
}

func (e *MissingInterpolationArgument) Error() string {
	return fmt.Sprintf("missing interpolation argument %q in %q (%s given)",
		e.Key, e.String, inspectValues(e.Values))
}

// ReservedInterpolationKey mirrors I18n::ReservedInterpolationKey: the string
// uses a %{...} placeholder whose name collides with a reserved option key
// (:scope, :default, …).
type ReservedInterpolationKey struct {
	Key    string
	String string
}

func (e *ReservedInterpolationKey) Error() string {
	return fmt.Sprintf("reserved key %q used in %q", e.Key, e.String)
}
