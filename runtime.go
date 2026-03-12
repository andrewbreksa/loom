package loom

import (
	"context"
	"fmt"
	"path"
	"strings"
	"time"
)

// ── Invariant errors ───────────────────────────────────────────────────────

// InvariantViolation holds a single invariant failure.
type InvariantViolation struct {
	Invariant string
	Err       error
}

// InvariantError is returned when one or more invariants are violated after a
// dispatch or signal emission settles.
type InvariantError struct {
	Violations []InvariantViolation
}

func (e InvariantError) Error() string {
	msgs := make([]string, len(e.Violations))
	for i, v := range e.Violations {
		msgs[i] = fmt.Sprintf("%s: %s", v.Invariant, v.Err)
	}
	return "invariant violations: " + strings.Join(msgs, "; ")
}

// PermitError is returned when an action's Permits() returns false.
type PermitError struct {
	Action string
}

func (e PermitError) Error() string {
	return fmt.Sprintf("action %q not permitted in current state", e.Action)
}

// Event is a record in the event log.
// For action events, Action is set and Signal is empty.
// For signal events, Signal is set and Action is empty.
type Event struct {
	Seq    int
	Action string
	Signal string
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
	telemetry    *runtimeTelemetry
}

func newRuntime(reg *Registry, telemetryOptions TelemetryOptions) *Runtime {
	rt := &Runtime{
		reg:          reg,
		env:          NewEnv(),
		derivedCache: make(map[string]any),
		telemetry:    newRuntimeTelemetry(telemetryOptions),
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
	return rt.DispatchContext(context.Background(), name, args)
}

// DispatchContext runs an action by name with a caller-provided context for
// telemetry propagation.
func (rt *Runtime) DispatchContext(ctx context.Context, name string, args map[string]any) (err error) {
	if args == nil {
		args = map[string]any{}
	}

	ctx, span, started := rt.telemetry.startDispatch(ctx, name, args)
	result := "ok"
	rebindCount := 0
	watchCallbacks := 0
	defer func() {
		rt.telemetry.endDispatch(ctx, span, name, result, rebindCount, watchCallbacks, time.Since(started), err)
	}()

	action, ok := rt.reg.Actions[name]
	if !ok {
		result = "unknown_action"
		err = fmt.Errorf("unknown action: %q", name)
		return err
	}

	view := rt.stateView()

	if !action.Permits(view, args) {
		result = "not_permitted"
		err = PermitError{Action: name}
		return err
	}

	rebinds := action.Effect(view, args)
	rebindCount = len(rebinds)
	watchCallbacks, err = rt.applyRebinds(ctx, rebinds, name, args)
	if err != nil {
		result = "error"
	}
	return err
}

func (rt *Runtime) applyRebinds(ctx context.Context, rebinds []Rebind, source string, args map[string]any) (int, error) {
	before := rt.env.Snapshot()
	savedCache := rt.snapshotDerivedCache()
	rt.history = append(rt.history, before)

	for _, r := range rebinds {
		rt.env = rt.env.Set(r.Key, r.Value)
	}
	rt.recomputeDerived()

	changed := changedKeys(before, rt.env)
	watchCallbacks := rt.fireWatches(ctx, changed, source)

	if inv := rt.checkInvariants(); inv != nil {
		rt.env = before
		rt.derivedCache = savedCache
		rt.history = rt.history[:len(rt.history)-1]
		return 0, *inv
	}

	rt.seq++
	rt.eventLog = append(rt.eventLog, Event{
		Seq:    rt.seq,
		Action: source,
		Args:   args,
		Before: before,
		After:  rt.env.Snapshot(),
	})
	return watchCallbacks, nil
}

func (rt *Runtime) fireWatches(ctx context.Context, changed map[string]bool, source string) int {
	var cascades [][]Rebind
	watchCallbacks := 0

	for _, watch := range rt.reg.Watches {
		for key := range changed {
			if matchPattern(watch.Pattern, key) {
				view := rt.stateView()
				value := rt.env.Get(key)
				result := watch.Fn(view, key, value)
				watchCallbacks++
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
			watchCallbacks += rt.fireWatches(ctx, second, "watch:"+source)
		}
	}
	return watchCallbacks
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
		var err error
		if e.Signal != "" {
			err = rt.Emit(e.Signal, e.Args)
		} else {
			err = rt.Dispatch(e.Action, e.Args)
		}
		if err != nil {
			return fmt.Errorf("replay seq %d: %w", e.Seq, err)
		}
	}
	return nil
}

// Emit fires a named signal into the runtime. Signal handlers may return
// rebinds; these cascade through the normal watch/derived/invariant chain.
// The signal is captured in the event log.
func (rt *Runtime) Emit(name string, args map[string]any) error {
	return rt.EmitContext(context.Background(), name, args)
}

// EmitContext is Emit with a caller-provided context for telemetry propagation.
func (rt *Runtime) EmitContext(ctx context.Context, name string, args map[string]any) error {
	if args == nil {
		args = map[string]any{}
	}

	sig := Signal{Name: name, Args: args}
	view := rt.stateView()

	var rebinds []Rebind
	for _, decl := range rt.reg.Signals {
		if decl.Signal == name {
			rebinds = append(rebinds, decl.Fn(view, sig)...)
		}
	}

	_, err := rt.applySignal(ctx, sig, rebinds)
	return err
}

func (rt *Runtime) applySignal(ctx context.Context, sig Signal, rebinds []Rebind) (int, error) {
	before := rt.env.Snapshot()
	savedCache := rt.snapshotDerivedCache()
	rt.history = append(rt.history, before)

	for _, r := range rebinds {
		rt.env = rt.env.Set(r.Key, r.Value)
	}
	rt.recomputeDerived()

	changed := changedKeys(before, rt.env)
	watchCallbacks := rt.fireWatches(ctx, changed, "signal:"+sig.Name)

	if inv := rt.checkInvariants(); inv != nil {
		rt.env = before
		rt.derivedCache = savedCache
		rt.history = rt.history[:len(rt.history)-1]
		return 0, *inv
	}

	rt.seq++
	rt.eventLog = append(rt.eventLog, Event{
		Seq:    rt.seq,
		Signal: sig.Name,
		Args:   sig.Args,
		Before: before,
		After:  rt.env.Snapshot(),
	})
	return watchCallbacks, nil
}

// Select returns all env keys matching the named selector.
func (rt *Runtime) Select(name string) map[string]any {
	return rt.stateView().Select(name)
}

// ── Internal ──────────────────────────────────────────────────────────────

func (rt *Runtime) stateView() StateView {
	return &runtimeView{
		env:          rt.env,
		derivedCache: rt.derivedCache,
		patterns:     rt.reg.Patterns,
		selectors:    rt.reg.Selectors,
	}
}

func (rt *Runtime) snapshotDerivedCache() map[string]any {
	cp := make(map[string]any, len(rt.derivedCache))
	for k, v := range rt.derivedCache {
		cp[k] = v
	}
	return cp
}

func (rt *Runtime) checkInvariants() *InvariantError {
	if len(rt.reg.Invariants) == 0 {
		return nil
	}
	view := rt.stateView()
	var violations []InvariantViolation
	for _, decl := range rt.reg.Invariants {
		for _, err := range decl.Fn(view) {
			violations = append(violations, InvariantViolation{Invariant: decl.Name, Err: err})
		}
	}
	if len(violations) == 0 {
		return nil
	}
	return &InvariantError{Violations: violations}
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
	selectors    map[string]SelectorDecl
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

func (v *runtimeView) Select(name string) map[string]any {
	sel, ok := v.selectors[name]
	if !ok {
		return nil
	}
	result := make(map[string]any)
	for k, val := range v.env.ToMap() {
		if matchPattern(sel.Pattern, k) {
			result[k] = val
		}
	}
	return result
}
