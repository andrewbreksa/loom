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
	reg *Registry
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

// Module loads all declarations from a Module.
func (l *Loom) Module(m Module) *Loom {
	l.reg.Merge(m.Register())
	return l
}

// Build returns a Runtime initialized with all registered declarations.
func (l *Loom) Build() *Runtime {
	return newRuntime(l.reg)
}

// ── Module interface ───────────────────────────────────────────────────────

// Module is the unit of organization in Loom.
// A game, a service, or a domain is a Module.
// Modules return a Registry of their declarations.
type Module interface {
	Register() *Registry
}
