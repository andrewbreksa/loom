package loom

// Loom assembles declarations into a Runtime.
//
//	l := loom.New()
//	l.Ref("game.state", "active")
//	l.Action("attack", AttackAction{})
//	l.Watch("player.*.life", onLifeChange)
//	rt := l.Build()
//	rt.Dispatch("attack", map[string]any{"target": "bob", "amount": 5})
type Loom struct {
	reg       *Registry
	telemetry TelemetryOptions
}

// New creates a new Loom assembler.
func New() *Loom {
	return &Loom{reg: NewRegistry()}
}

// ── Declaration ────────────────────────────────────────────────────────────

func (l *Loom) Ref(key string, value any) *Loom {
	l.reg.AddRef(key, value)
	return l
}

// Refs adds multiple ref declarations at once.
func (l *Loom) Refs(decls ...RefDecl) *Loom {
	for _, d := range decls {
		l.reg.AddRef(d.Key, d.Value)
	}
	return l
}

func (l *Loom) Derived(key string, fn DerivedFn) *Loom {
	l.reg.AddDerived(key, key, fn)
	return l
}

func (l *Loom) Watch(pattern string, fn WatchFn) *Loom {
	l.reg.AddWatch(pattern, pattern, fn)
	return l
}

func (l *Loom) Action(name string, action Action) *Loom {
	l.reg.AddAction(name, action)
	return l
}

func (l *Loom) Pattern(name string, fn PatternFn) *Loom {
	l.reg.AddPattern(name, fn)
	return l
}

// Invariant registers a global rule evaluated on settled state after each
// dispatch or signal emission. Dispatch fails if any invariant returns errors.
func (l *Loom) Invariant(name string, fn InvariantFn) *Loom {
	l.reg.AddInvariant(name, fn)
	return l
}

// OnSignal registers a handler for the named signal.
// Handlers may return rebinds that cascade through the normal watch/derived chain.
func (l *Loom) OnSignal(signal string, fn OnSignalFn) *Loom {
	l.reg.AddSignal(signal, signal, fn)
	return l
}

// Selector registers a named, reusable scope over refs identified by pattern.
// The pattern uses the same dot-separated glob syntax as Watch patterns.
func (l *Loom) Selector(name string, pattern string) *Loom {
	l.reg.AddSelector(name, pattern)
	return l
}

// Module loads all declarations from a Module.
func (l *Loom) Module(m Module) *Loom {
	l.reg.Merge(m.Register())
	return l
}

// WithTelemetry configures OpenTelemetry integration for runtimes built from
// this Loom instance.
func (l *Loom) WithTelemetry(options TelemetryOptions) *Loom {
	l.telemetry = options
	return l
}

// Build returns a Runtime initialized with all registered declarations.
func (l *Loom) Build() *Runtime {
	return newRuntime(l.reg, l.telemetry)
}

// ── Module interface ───────────────────────────────────────────────────────

// Module is the unit of organization in Loom.
// A game, a service, or a domain is a Module.
// Modules return a Registry of their declarations.
type Module interface {
	Register() *Registry
}
