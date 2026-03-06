package loom

// The seven primitives.

// ── 1. Action ─────────────────────────────────────────────────────────────

// Action is a permitted state transition.
// Permits() is a pure guard.
// Effect() returns descriptions of changes, never executes them.
type Action interface {
	Permits(state StateView, args map[string]any) bool
	Effect(state StateView, args map[string]any) []Rebind
}

// ActionFunc is a convenience Action from two functions.
type ActionFunc struct {
	permits func(state StateView, args map[string]any) bool
	effect  func(state StateView, args map[string]any) []Rebind
}

func NewAction(
	permits func(state StateView, args map[string]any) bool,
	effect func(state StateView, args map[string]any) []Rebind,
) Action {
	return ActionFunc{permits: permits, effect: effect}
}

func (a ActionFunc) Permits(state StateView, args map[string]any) bool {
	return a.permits(state, args)
}
func (a ActionFunc) Effect(state StateView, args map[string]any) []Rebind {
	return a.effect(state, args)
}

// ── 2. Watch ──────────────────────────────────────────────────────────────

// WatchFn fires when a watched ref changes.
type WatchFn func(state StateView, key string, value any) []Rebind

// WatchDecl binds a pattern to a WatchFn.
type WatchDecl struct {
	Pattern string
	Name    string
	Fn      WatchFn
}

// ── 3. Derived ────────────────────────────────────────────────────────────

// DerivedFn is a pure function over the env.
type DerivedFn func(state StateView) any

// DerivedDecl binds a key to a DerivedFn.
type DerivedDecl struct {
	Key  string
	Name string
	Fn   DerivedFn
}

// ── 4. Pattern ────────────────────────────────────────────────────────────

// PatternFn is a named, reusable effect.
type PatternFn func(state StateView, args ...any) []Rebind

// ── 5. Ref ────────────────────────────────────────────────────────────────

// RefDecl is a ref declaration at init time.
type RefDecl struct {
	Key   string
	Value any
}

// ── 6. For ────────────────────────────────────────────────────────────────

// ForFn iterates over a namespace, producing rebinds.
type ForFn func(key string, value any) []Rebind

// ForEach runs fn over every key in namespace, returning all rebinds.
func ForEach(ns map[string]any, fn ForFn) []Rebind {
	var result []Rebind
	for k, v := range ns {
		result = append(result, fn(k, v)...)
	}
	return result
}

// ── 7. Apply ──────────────────────────────────────────────────────────────

// ApplyFn is the logic-layer boundary.
type ApplyFn func(description any) []Rebind

// ── StateView ──────────────────────────────────────────────────────────────

// StateView is the read-only view of the env exposed to Action, Watch,
// and Derived functions.
type StateView interface {
	Get(key string) any
	GetOr(key string, def any) any
	Has(key string) bool
	Namespace(prefix string) map[string]any
	Length(prefix string) int
	Pattern(name string, args ...any) []Rebind
	Rebind(key string, value any) Rebind
	Apply(description any) []Rebind
}
