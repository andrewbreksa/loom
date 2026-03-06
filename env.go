// Package loom implements a reactive functional programming framework
// built on seven primitives: ref, derived, watch, action, pattern, for, apply.
//
// The entire runtime is one function:
//
//	func step(env Env, action Action) Env {
//	    if !action.Permits(env) { return env }
//	    rebinds := action.Effect(env)
//	    newEnv  := env.Apply(rebinds)
//	    watches := fireWatches(env, newEnv)
//	    return fold(newEnv, watches)
//	}
package loom

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Rebind is a description of a state change.
// Actions return []Rebind — they never mutate directly.
type Rebind struct {
	Key   string
	Value any
}

func R(key string, value any) Rebind {
	return Rebind{Key: key, Value: value}
}

// Env is an immutable snapshot of the ref graph.
// Every Rebind produces a new Env.
// History is a slice of Envs — free, because immutable.
type Env struct {
	data map[string]any
}

func NewEnv() Env {
	return Env{data: make(map[string]any)}
}

func envFrom(data map[string]any) Env {
	cp := make(map[string]any, len(data))
	for k, v := range data {
		cp[k] = v
	}
	return Env{data: cp}
}

// Get returns the value for a key, or nil if not set.
func (e Env) Get(key string) any {
	return e.data[key]
}

// GetOr returns the value for a key, or the default if not set.
func (e Env) GetOr(key string, def any) any {
	if v, ok := e.data[key]; ok {
		return v
	}
	return def
}

// Set returns a new Env with the key rebound to value.
func (e Env) Set(key string, value any) Env {
	cp := make(map[string]any, len(e.data)+1)
	for k, v := range e.data {
		cp[k] = v
	}
	cp[key] = value
	return Env{data: cp}
}

// Apply returns a new Env with all rebinds applied.
func (e Env) Apply(rebinds []Rebind) Env {
	env := e
	for _, r := range rebinds {
		env = env.Set(r.Key, r.Value)
	}
	return env
}

// Namespace returns all keys that start with prefix.
func (e Env) Namespace(prefix string) map[string]any {
	p := strings.TrimRight(prefix, ".") + "."
	result := make(map[string]any)
	for k, v := range e.data {
		if strings.HasPrefix(k, p) {
			result[k] = v
		}
	}
	return result
}

// Length returns the count of keys under a namespace prefix.
func (e Env) Length(prefix string) int {
	return len(e.Namespace(prefix))
}

// Has returns true if the key exists.
func (e Env) Has(key string) bool {
	_, ok := e.data[key]
	return ok
}

// Diff returns the keys that changed between e and other.
func (e Env) Diff(other Env) []Rebind {
	seen := make(map[string]bool)
	var changed []Rebind

	for k, v := range other.data {
		seen[k] = true
		old := e.data[k]
		if !equal(old, v) {
			changed = append(changed, Rebind{Key: k, Value: v})
		}
	}
	for k := range e.data {
		if !seen[k] {
			changed = append(changed, Rebind{Key: k, Value: nil})
		}
	}
	return changed
}

// Snapshot returns a copy of the current env.
func (e Env) Snapshot() Env {
	return envFrom(e.data)
}

// ToMap returns the underlying data as a plain map.
func (e Env) ToMap() map[string]any {
	cp := make(map[string]any, len(e.data))
	for k, v := range e.data {
		cp[k] = v
	}
	return cp
}

// String returns a JSON representation.
func (e Env) String() string {
	b, _ := json.MarshalIndent(e.data, "", "  ")
	return string(b)
}

// equal does a simple equality check across basic types.
func equal(a, b any) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	// JSON round-trip for deep equality
	aj, _ := json.Marshal(a)
	bj, _ := json.Marshal(b)
	return string(aj) == string(bj)
}

// ── Type helpers ──────────────────────────────────────────────────────────

// String coerces a ref value to string.
func String(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}

// Int coerces a ref value to int.
func Int(v any) int {
	switch t := v.(type) {
	case int:
		return t
	case int64:
		return int(t)
	case float64:
		return int(t)
	case json.Number:
		n, _ := t.Int64()
		return int(n)
	}
	return 0
}

// Float coerces a ref value to float64.
func Float(v any) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case int:
		return float64(t)
	case json.Number:
		f, _ := t.Float64()
		return f
	}
	return 0
}

// Bool coerces a ref value to bool.
func Bool(v any) bool {
	if b, ok := v.(bool); ok {
		return b
	}
	return false
}

// Slice coerces a ref value to []any.
func Slice(v any) []any {
	if s, ok := v.([]any); ok {
		return s
	}
	return nil
}
