// Copyright (c) the go-ruby-i18n/i18n authors
//
// SPDX-License-Identifier: BSD-3-Clause

package i18n

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// reservedKeys are the option names I18n forbids as %{...} placeholders (they
// collide with translate options). Matches I18n::RESERVED_KEYS.
var reservedKeys = []string{
	"cascade", "deep_interpolation", "skip_interpolation", "default",
	"exception_handler", "fallback", "fallback_in_progress",
	"fallback_original_locale", "format", "object", "raise", "resolve",
	"scope", "separator", "throw",
}

// interpolation patterns mirror I18n::DEFAULT_INTERPOLATION_PATTERNS, applied in
// priority order: a literal "%%", a "%{name}" (optionally "%{name|word}"), and a
// sprintf-style "%<name>spec".
var (
	rxPercent  = regexp.MustCompile(`^%%`)
	rxBrace    = regexp.MustCompile(`^%\{([\w|]+)\}`)
	rxAngle    = regexp.MustCompile(`^%<(\w+)>([^\d]*?\d*\.?\d*[bBdiouxXeEfgGcps])`)
	rxReserved *regexp.Regexp
)

func init() {
	rxReserved = regexp.MustCompile(`(?:^|[^%])%\{(` + strings.Join(reservedKeys, "|") + `)\}`)
}

// MissingInterpolationHandler decides what to substitute for a placeholder whose
// value is absent. Returning an error aborts interpolation. The default keeps the
// placeholder text literal, matching the i18n gem's out-of-the-box behaviour
// (its default handler is overridable but, as shipped, leaves the text in place
// because the un-configured handler returns the string unchanged for the gsub).
type MissingInterpolationHandler func(key string, values map[string]Value, s string) (Value, error)

// RaiseMissingInterpolation is the handler that reproduces the gem's documented
// I18n::MissingInterpolationArgument behaviour. Install it via
// SetMissingInterpolationHandler to make an absent placeholder an error instead
// of being left literal.
func RaiseMissingInterpolation(key string, values map[string]Value, s string) (Value, error) {
	return nil, &MissingInterpolationArgument{Key: key, Values: values, String: s}
}

// keepMissingInterpolation leaves the placeholder literally in place.
func keepMissingInterpolation(key string, values map[string]Value, s string) (Value, error) {
	return nil, errKeepLiteral
}

var errKeepLiteral = fmt.Errorf("i18n: keep placeholder literal")

// interpolate substitutes %{...} / %<...> placeholders in s with values, the way
// I18n.interpolate does. A reserved key in a placeholder is an error. When a key
// is absent the handler decides the substitution.
func interpolate(s string, values map[string]Value, handler MissingInterpolationHandler) (string, error) {
	if loc := rxReserved.FindStringSubmatch(s); loc != nil {
		return "", &ReservedInterpolationKey{Key: loc[1], String: s}
	}

	var b strings.Builder
	i := 0
	interpolated := false
	for i < len(s) {
		rest := s[i:]
		if m := rxPercent.FindString(rest); m != "" {
			interpolated = true
			b.WriteByte('%')
			i += len(m)
			continue
		}
		if m := rxBrace.FindStringSubmatch(rest); m != nil {
			interpolated = true
			// "%{name|word}" — only the part before '|' names the value.
			name := m[1]
			if k := strings.IndexByte(name, '|'); k >= 0 {
				name = name[:k]
			}
			val, ok := lookupValue(values, name)
			if !ok {
				sub, err := handler(name, values, s)
				if err != nil {
					if err == errKeepLiteral {
						b.WriteString(m[0])
						i += len(m[0])
						continue
					}
					return "", err
				}
				val = sub
			}
			b.WriteString(rubyToS(val))
			i += len(m[0])
			continue
		}
		if m := rxAngle.FindStringSubmatch(rest); m != nil {
			interpolated = true
			name, spec := m[1], m[2]
			val, ok := lookupValue(values, name)
			if !ok {
				sub, err := handler(name, values, s)
				if err != nil {
					if err == errKeepLiteral {
						b.WriteString(m[0])
						i += len(m[0])
						continue
					}
					return "", err
				}
				val = sub
			}
			b.WriteString(sprintfSpec(spec, val))
			i += len(m[0])
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	if !interpolated {
		return s, nil
	}
	return b.String(), nil
}

// lookupValue resolves a placeholder name against the provided values. Names are
// symbol-equivalent, so "name" matches the "name" key.
func lookupValue(values map[string]Value, name string) (Value, bool) {
	v, ok := values[name]
	return v, ok
}

// rubyToS renders a value the way Ruby's String interpolation (#to_s) would for
// the substituted-in result of a %{...} placeholder.
func rubyToS(v Value) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case bool:
		return strconv.FormatBool(x)
	case int:
		return strconv.Itoa(x)
	case int64:
		return strconv.FormatInt(x, 10)
	case float64:
		return strconv.FormatFloat(x, 'g', -1, 64)
	default:
		return fmt.Sprintf("%v", x)
	}
}

// sprintfSpec applies a Ruby sprintf conversion spec (the part after "%<name>",
// e.g. "d", "05.2f", "x") to a value, mirroring Ruby's sprintf("%#{spec}", v).
func sprintfSpec(spec string, v Value) string {
	verb := spec[len(spec)-1]
	goSpec := "%" + spec
	switch verb {
	case 'd', 'i', 'u', 'b', 'B', 'o', 'x', 'X', 'c':
		// Ruby 'i'/'u' map to Go 'd'.
		if verb == 'i' || verb == 'u' {
			goSpec = "%" + spec[:len(spec)-1] + "d"
		}
		return fmt.Sprintf(goSpec, toInt(v))
	case 'e', 'E', 'f', 'g', 'G':
		return fmt.Sprintf(goSpec, toFloat(v))
	case 's', 'p':
		if verb == 'p' {
			goSpec = "%" + spec[:len(spec)-1] + "q"
		}
		return fmt.Sprintf(goSpec, rubyToS(v))
	default:
		return fmt.Sprintf(goSpec, v)
	}
}

func toInt(v Value) int64 {
	switch x := v.(type) {
	case int:
		return int64(x)
	case int64:
		return x
	case float64:
		return int64(x)
	case string:
		n, _ := strconv.ParseInt(x, 10, 64)
		return n
	default:
		return 0
	}
}

func toFloat(v Value) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case int:
		return float64(x)
	case int64:
		return float64(x)
	case string:
		f, _ := strconv.ParseFloat(x, 64)
		return f
	default:
		return 0
	}
}
