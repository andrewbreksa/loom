package loom

import (
	"fmt"
	"strings"
)

// ── Spread ────────────────────────────────────────────────────────────────

// SpreadRange is one axis of a spread.
type SpreadRange struct {
	Name   string
	Values []any
}

// Range creates a SpreadRange from a slice.
func Range(name string, values ...any) SpreadRange {
	return SpreadRange{Name: name, Values: values}
}

// IntRange creates a SpreadRange for integers [start, end).
func IntRange(name string, start, end int) SpreadRange {
	vals := make([]any, end-start)
	for i := range vals {
		vals[i] = start + i
	}
	return SpreadRange{Name: name, Values: vals}
}

// Spread generates a flat list of (key, value) pairs from a pattern and ranges.
//
//	Spread("pit.{N}.stones", 4, IntRange("N", 1, 7))
//	→ [("pit.1.stones", 4), ("pit.2.stones", 4), ...]
func Spread(pattern string, value any, ranges ...SpreadRange) []RefDecl {
	if len(ranges) == 0 {
		return []RefDecl{{Key: pattern, Value: value}}
	}

	type binding struct{ name, val string }

	var product func(ranges []SpreadRange) [][]binding
	product = func(rs []SpreadRange) [][]binding {
		if len(rs) == 0 {
			return [][]binding{{}}
		}
		rest := product(rs[1:])
		var result [][]binding
		for _, v := range rs[0].Values {
			for _, row := range rest {
				b := binding{rs[0].Name, fmt.Sprintf("%v", v)}
				result = append(result, append([]binding{b}, row...))
			}
		}
		return result
	}

	var decls []RefDecl
	for _, bindings := range product(ranges) {
		key := pattern
		val := fmt.Sprintf("%v", value)
		for _, b := range bindings {
			key = strings.ReplaceAll(key, "{"+b.name+"}", b.val)
			val = strings.ReplaceAll(val, "{"+b.name+"}", b.val)
		}
		// if value wasn't a string template, keep original value
		decls = append(decls, RefDecl{Key: key, Value: value})
	}
	return decls
}

// ── Chain ─────────────────────────────────────────────────────────────────

// Chain generates linked ref pairs from a sequence.
//
//	Chain("turn.after", []string{"alice", "bob", "carol"}, true)
//	→ turn.after.alice=bob, turn.after.bob=carol, turn.after.carol=alice
func Chain(namespace string, sequence []string, ring bool) []RefDecl {
	var decls []RefDecl
	for i, item := range sequence {
		if i+1 < len(sequence) {
			decls = append(decls, RefDecl{
				Key:   namespace + "." + item,
				Value: sequence[i+1],
			})
		} else if ring {
			decls = append(decls, RefDecl{
				Key:   namespace + "." + item,
				Value: sequence[0],
			})
		}
	}
	return decls
}

// ── Pair ──────────────────────────────────────────────────────────────────

// Pair generates symmetric ref pairs.
//
//	Pair("opposite", [][2]string{{"north", "south"}, {"east", "west"}})
//	→ opposite.north=south, opposite.south=north, ...
func Pair(namespace string, pairs [][2]string) []RefDecl {
	var decls []RefDecl
	for _, p := range pairs {
		decls = append(decls,
			RefDecl{Key: namespace + "." + p[0], Value: p[1]},
			RefDecl{Key: namespace + "." + p[1], Value: p[0]},
		)
	}
	return decls
}
