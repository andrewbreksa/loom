package loom

import (
	"fmt"
	"path"
	"strings"
)

// PermitError is returned when an action's Permits() returns false.
type PermitError struct {
	Action string
}

func (e PermitError) Error() string {
	return fmt.Sprintf("action %q not permitted in current state", e.Action)
}

// Event is a record in the event log.
type Event struct {
	Seq    int
	Action string
	Args   map[string]any
	Before Env
	After  Env
}

// Runtime is the fold.
type Runtime struct {
	reg          *Registry
	env          Env
	derivedCache map[string]any
	history      []Env
	eventLog     []Event
	seq          int
}

func newRuntime(reg *Registry) *Runtime {
	rt := &Runtime{
		reg:          reg,
		env:          NewEnv(),
		derivedCache: make(map[string]any),
	}
	rt.initEnv()
	return rt
}

func (rt *Runtime) initEnv() {
	for _, decl := range rt.reg.Refs {
		rt.env = rt.env.Set(decl.Key, decl.Value)
	}
	rt.recomputeDerived()
}

// Dispatch runs an action by name.
func (rt *Runtime) Dispatch(name string, args map[string]any) error {
	if args == nil {
		args = map[string]any{}
	}
	action, ok := rt.reg.Actions[name]
	if !ok {
		return fmt.Errorf("unknown action: %q", name)
	}

	view := rt.stateView()

	if !action.Permits(view, args) {
		return PermitError{Action: name}
	}

	rebinds := action.Effect(view, args)
	return rt.applyRebinds(rebinds, name, args)
}

func (rt *Runtime) applyRebinds(rebinds []Rebind, source string, args map[string]any) error {
	before := rt.env.Snapshot()
	rt.history = append(rt.history, before)

	for _, r := range rebinds {
		rt.env = rt.env.Set(r.Key, r.Value)
	}
	rt.recomputeDerived()

	changed := changedKeys(before, rt.env)
	rt.fireWatches(changed, source)

	rt.seq++
	rt.eventLog = append(rt.eventLog, Event{
		Seq:    rt.seq,
		Action: source,
		Args:   args,
		Before: before,
		After:  rt.env.Snapshot(),
	})
	return nil
}

func (rt *Runtime) fireWatches(changed map[string]bool, source string) {
	var cascades [][]Rebind

	for _, watch := range rt.reg.Watches {
		for key := range changed {
			if matchPattern(watch.Pattern, key) {
				view   := rt.stateView()
				value  := rt.env.Get(key)
				result := watch.Fn(view, key, value)
				if len(result) > 0 {
					cascades = append(cascades, result)
				}
				break
			}
		}
	}

	for _, rebinds := range cascades {
		before := rt.env.Snapshot()
		for _, r := range rebinds {
			rt.env = rt.env.Set(r.Key, r.Value)
		}
		rt.recomputeDerived()
		second := changedKeys(before, rt.env)
		if len(second) > 0 {
			rt.fireWatches(second, "watch:"+source)
		}
	}
}

func (rt *Runtime) recomputeDerived() {
	view := rt.stateView()
	for _, decl := range rt.reg.Derived {
		value := decl.Fn(view)
		rt.derivedCache[decl.Key] = value
		rt.env = rt.env.Set(decl.Key, value)
	}
}

// ── State Access ──────────────────────────────────────────────────────────

func (rt *Runtime) Get(key string) any {
	if v, ok := rt.derivedCache[key]; ok {
		return v
	}
	return rt.env.Get(key)
}

func (rt *Runtime) GetOr(key string, def any) any {
	v := rt.Get(key)
	if v == nil {
		return def
	}
	return v
}

func (rt *Runtime) Namespace(prefix string) map[string]any { return rt.env.Namespace(prefix) }
func (rt *Runtime) Length(prefix string) int               { return rt.env.Length(prefix) }
func (rt *Runtime) Snapshot() Env                          { return rt.env.Snapshot() }
func (rt *Runtime) History() []Env                         { return append([]Env{}, rt.history...) }
func (rt *Runtime) EventLog() []Event                      { return append([]Event{}, rt.eventLog...) }

func (rt *Runtime) Replay(events []Event) error {
	for _, e := range events {
		if err := rt.Dispatch(e.Action, e.Args); err != nil {
			return fmt.Errorf("replay seq %d: %w", e.Seq, err)
		}
	}
	return nil
}

// ── Internal ──────────────────────────────────────────────────────────────

func (rt *Runtime) stateView() StateView {
	return &runtimeView{
		env:          rt.env,
		derivedCache: rt.derivedCache,
		patterns:     rt.reg.Patterns,
	}
}

func changedKeys(before, after Env) map[string]bool {
	changed := make(map[string]bool)
	for _, r := range before.Diff(after) {
		changed[r.Key] = true
	}
	return changed
}

func matchPattern(pattern, key string) bool {
	if pattern == key {
		return true
	}
	if strings.Contains(pattern, "*") {
		// convert dot-separated to path for glob matching
		p := strings.ReplaceAll(pattern, ".", "/")
		k := strings.ReplaceAll(key, ".", "/")
		matched, err := path.Match(p, k)
		if err == nil && matched {
			return true
		}
	}
	// prefix match
	return strings.HasPrefix(key, strings.TrimRight(pattern, ".")+".")
}

// ── StateView impl ─────────────────────────────────────────────────────────

type runtimeView struct {
	env          Env
	derivedCache map[string]any
	patterns     map[string]PatternFn
}

func (v *runtimeView) Get(key string) any {
	if val, ok := v.derivedCache[key]; ok {
		return val
	}
	return v.env.Get(key)
}

func (v *runtimeView) GetOr(key string, def any) any {
	val := v.Get(key)
	if val == nil {
		return def
	}
	return val
}

func (v *runtimeView) Has(key string) bool                    { return v.env.Has(key) }
func (v *runtimeView) Namespace(prefix string) map[string]any { return v.env.Namespace(prefix) }
func (v *runtimeView) Length(prefix string) int               { return v.env.Length(prefix) }
func (v *runtimeView) Rebind(key string, value any) Rebind    { return Rebind{Key: key, Value: value} }

func (v *runtimeView) Pattern(name string, args ...any) []Rebind {
	fn, ok := v.patterns[name]
	if !ok {
		return nil
	}
	return fn(v, args...)
}

func (v *runtimeView) Apply(description any) []Rebind {
	switch d := description.(type) {
	case []Rebind:
		return d
	case map[string]any:
		var result []Rebind
		for k, val := range d {
			result = append(result, Rebind{Key: k, Value: val})
		}
		return result
	}
	return nil
}
