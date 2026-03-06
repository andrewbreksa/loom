package loom

// Registry holds all declarations before the runtime is built.
// Modules register into a Registry.
// Loom assembles them into a Runtime.
type Registry struct {
	Refs     []RefDecl
	Derived  []DerivedDecl
	Watches  []WatchDecl
	Actions  map[string]Action
	Patterns map[string]PatternFn
}

func NewRegistry() *Registry {
	return &Registry{
		Actions:  make(map[string]Action),
		Patterns: make(map[string]PatternFn),
	}
}

func (r *Registry) AddRef(key string, value any) {
	r.Refs = append(r.Refs, RefDecl{Key: key, Value: value})
}

func (r *Registry) AddDerived(key string, name string, fn DerivedFn) {
	r.Derived = append(r.Derived, DerivedDecl{Key: key, Name: name, Fn: fn})
}

func (r *Registry) AddWatch(pattern string, name string, fn WatchFn) {
	r.Watches = append(r.Watches, WatchDecl{Pattern: pattern, Name: name, Fn: fn})
}

func (r *Registry) AddAction(name string, action Action) {
	r.Actions[name] = action
}

func (r *Registry) AddPattern(name string, fn PatternFn) {
	r.Patterns[name] = fn
}

func (r *Registry) Merge(other *Registry) {
	r.Refs    = append(r.Refs, other.Refs...)
	r.Derived = append(r.Derived, other.Derived...)
	r.Watches = append(r.Watches, other.Watches...)
	for k, v := range other.Actions {
		r.Actions[k] = v
	}
	for k, v := range other.Patterns {
		r.Patterns[k] = v
	}
}
