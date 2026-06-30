// Copyright (c) the go-ruby-i18n/i18n authors
//
// SPDX-License-Identifier: BSD-3-Clause

package i18n

// PluralRule maps a count to one of the CLDR plural category names
// ("zero", "one", "two", "few", "many", "other"). The Simple backend ships only
// the basic English-style rule (zero/one/other); richer rules — the CLDR
// categories the i18n-spec / rails-i18n pluralization data uses — are registered
// per locale and consulted by Translate when a :count is given.
type PluralRule func(count int) string

// simplePluralRule is exactly I18n::Backend::Base#pluralization_key: :zero when
// count == 0 and a :zero entry exists is decided at lookup time, so the rule
// itself returns "one" for 1 and "other" otherwise; the :zero special-case is
// applied by the caller (it depends on the entry's keys, not the locale).
func simplePluralRule(count int) string {
	if count == 1 {
		return "one"
	}
	return "other"
}

// cldrSlavicRule implements the CLDR "one/few/many/other" rule shared by Russian,
// Ukrainian and similar Slavic locales — the canonical example of a rule that
// needs :few and :many, included so the library can select those categories.
func cldrSlavicRule(count int) string {
	mod10 := count % 10
	mod100 := count % 100
	switch {
	case mod10 == 1 && mod100 != 11:
		return "one"
	case mod10 >= 2 && mod10 <= 4 && !(mod100 >= 12 && mod100 <= 14):
		return "few"
	case mod10 == 0 || (mod10 >= 5 && mod10 <= 9) || (mod100 >= 11 && mod100 <= 14):
		return "many"
	default:
		return "other"
	}
}

// cldrWestSlavicRule implements the CLDR "one/few/many/other" rule for Czech and
// Slovak (category boundaries differ from the East-Slavic rule above).
func cldrWestSlavicRule(count int) string {
	switch {
	case count == 1:
		return "one"
	case count >= 2 && count <= 4:
		return "few"
	default:
		return "other"
	}
}

// cldrPolishRule implements the CLDR "one/few/many/other" rule for Polish.
func cldrPolishRule(count int) string {
	if count == 1 {
		return "one"
	}
	mod10 := count % 10
	mod100 := count % 100
	switch {
	case mod10 >= 2 && mod10 <= 4 && !(mod100 >= 12 && mod100 <= 14):
		return "few"
	default:
		return "many"
	}
}

// cldrArabicRule implements the CLDR rule for Arabic, which exercises every
// category including "zero" and "two".
func cldrArabicRule(count int) string {
	mod100 := count % 100
	switch {
	case count == 0:
		return "zero"
	case count == 1:
		return "one"
	case count == 2:
		return "two"
	case mod100 >= 3 && mod100 <= 10:
		return "few"
	case mod100 >= 11 && mod100 <= 99:
		return "many"
	default:
		return "other"
	}
}

// cldrFrenchRule implements the CLDR rule for French/Portuguese-style locales
// where 0 and 1 are both "one".
func cldrFrenchRule(count int) string {
	if count == 0 || count == 1 {
		return "one"
	}
	return "other"
}

// builtinPluralRules is the default registry, keyed by locale. A locale absent
// here falls back to the Simple-backend rule.
var builtinPluralRules = map[string]PluralRule{
	"ru": cldrSlavicRule,
	"uk": cldrSlavicRule,
	"cs": cldrWestSlavicRule,
	"sk": cldrWestSlavicRule,
	"pl": cldrPolishRule,
	"ar": cldrArabicRule,
	"fr": cldrFrenchRule,
	"pt": cldrFrenchRule,
}
