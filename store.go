// Copyright (c) the go-ruby-i18n/i18n authors
//
// SPDX-License-Identifier: BSD-3-Clause

package i18n

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Value is a translation-tree node. The host (rbgo) loads YAML/Ruby translation
// files into this shape — the file parsing is the seam; this library does the
// lookup, interpolation, pluralization and localization over it. A node is one
// of: nil, bool, int, int64, float64, string, []Value, or map[string]Value
// (a nested sub-tree, the Ruby symbol-keyed Hash). Keys are stored as plain
// strings (the symbol name without the leading colon), matching how the Simple
// backend symbolizes loaded data.
type Value any

// normalize coerces an arbitrary loaded value into the canonical Value model so
// callers may hand us map[string]any / []any from a generic YAML loader. It is
// the bridge the file-loading seam crosses.
func normalize(v any) Value {
	switch x := v.(type) {
	case nil, bool, int, int64, float64, string:
		return x
	case map[string]Value:
		out := make(map[string]Value, len(x))
		for k, val := range x {
			out[k] = normalize(val)
		}
		return out
	case map[string]any:
		out := make(map[string]Value, len(x))
		for k, val := range x {
			out[k] = normalize(val)
		}
		return out
	case []Value:
		out := make([]Value, len(x))
		for i, val := range x {
			out[i] = normalize(val)
		}
		return out
	case []any:
		out := make([]Value, len(x))
		for i, val := range x {
			out[i] = normalize(val)
		}
		return out
	default:
		return v
	}
}

// deepMerge merges src into dst (both symbol-keyed sub-trees) the way the Simple
// backend's store_translations does: a Hash value recurses (preserving sibling
// keys), any other value overwrites. dst is mutated and returned.
func deepMerge(dst, src map[string]Value) map[string]Value {
	for k, sv := range src {
		if sm, ok := sv.(map[string]Value); ok {
			if dm, ok := dst[k].(map[string]Value); ok {
				dst[k] = deepMerge(dm, sm)
				continue
			}
		}
		dst[k] = sv
	}
	return dst
}

// lookupNode walks a dotted/segmented key through a sub-tree, returning the node
// and whether every segment resolved through a Hash. A nil intermediate or a
// scalar where a Hash was needed yields (nil, false).
func lookupNode(root map[string]Value, segments []string) (Value, bool) {
	var cur Value = root
	for _, seg := range segments {
		m, ok := cur.(map[string]Value)
		if !ok {
			return nil, false
		}
		val, present := m[seg]
		if !present {
			return nil, false
		}
		cur = val
	}
	return cur, true
}

// splitKey turns a dotted key plus an optional scope into the flat segment list
// the store is walked with. The separator is "." (I18n.default_separator).
func splitKey(scope []string, key string) []string {
	var segs []string
	for _, s := range scope {
		segs = append(segs, splitDots(s)...)
	}
	segs = append(segs, splitDots(key)...)
	return segs
}

func splitDots(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, ".")
}

// inspectEntry renders a plural sub-hash the way Ruby's Hash#inspect does in the
// modern (3.4+) short form, e.g. {one: "one", other: "%{count} other"} — used in
// the InvalidPluralizationData message so it matches the gem byte-for-byte.
func inspectEntry(m map[string]Value) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s: %s", k, inspectValue(m[k])))
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

func inspectValues(m map[string]Value) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s: %s", k, inspectValue(m[k])))
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

// inspectValue renders a scalar the way Ruby's #inspect would for the shapes that
// appear in plural data / interpolation values.
func inspectValue(v Value) string {
	switch x := v.(type) {
	case string:
		return strconv.Quote(x)
	case nil:
		return "nil"
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
