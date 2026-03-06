# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
go test ./...                        # run all tests
go test -run TestName ./...          # run a single test
go build ./...                       # build
```

## Architecture

Loom is a Go library (`github.com/formix/loom`) implementing a reactive functional state machine built on seven primitives. The flow is:

**Assembly → Build → Dispatch**

1. `Loom` (loom.go) — fluent assembler. Collects declarations via `Ref`, `Derived`, `Watch`, `Action`, `Pattern`, `Module`.
2. `Registry` (registry.go) — plain container holding all declarations before the runtime exists. `Module.Register()` returns a `*Registry`; `Loom.Module()` merges it in.
3. `Runtime` (runtime.go) — created by `Loom.Build()`. Owns the live `Env` and executes the step loop on `Dispatch`.

### State model

`Env` (env.go) is an **immutable** flat `map[string]any` with dot-separated keys as namespaces (e.g. `"player.alice.health"`). Every mutation returns a new `Env`; history is just a `[]Env` slice.

### The seven primitives (primitives.go)

| Primitive | Role |
|-----------|------|
| **Ref** | Initial key/value declaration |
| **Derived** | Pure function recomputed after every state change |
| **Watch** | Reactive callback fired when a matched key changes; may return `[]Rebind` to cascade |
| **Action** | State transition: `Permits()` guard + `Effect()` returning `[]Rebind` (never mutates) |
| **Pattern** | Named reusable `[]Rebind` factory, invoked via `StateView.Pattern(name, args...)` |
| **For** | `ForEach(ns, fn)` iterates a namespace and accumulates rebinds |
| **Apply** | `StateView.Apply(description)` converts `[]Rebind` or `map[string]any` to `[]Rebind` |

### Dispatch step (runtime.go)

```
Dispatch(name, args)
  → action.Permits(view, args)       // guard; returns PermitError if false
  → action.Effect(view, args)        // returns []Rebind
  → apply rebinds to Env
  → recomputeDerived()               // all Derived functions rerun
  → fireWatches(changedKeys)         // pattern-matched; cascades recursively
  → append to eventLog + history
```

Watch patterns use dot-separated glob (`*` wildcard) via `path.Match` after converting dots to slashes; also supports prefix matching.

### Sugar helpers (sugar.go)

- `Spread(pattern, value, ranges...)` — cartesian-product key initialization (e.g. a 3×3 grid)
- `Chain(ns, sequence, ring)` — linked sequence refs (e.g. turn order)
- `Pair(ns, pairs)` — symmetric bidirectional refs (e.g. opposites)

### Type coercions (env.go)

`String(v)`, `Int(v)`, `Float(v)`, `Bool(v)`, `Slice(v)` — safe coercions from `any` ref values. Use these instead of direct type assertions.

### Key conventions

- All state lives in flat dot-namespaced keys; `Namespace(prefix)` and `Length(prefix)` query sub-trees.
- `Rebind` / `R(key, value)` is the unit of state change — never mutate state directly.
- `StateView` is the read-only interface passed to all callbacks; use `view.Rebind(k, v)` to construct rebinds within callbacks.
- `R(key, value)` is a shorthand for `Rebind{Key: key, Value: value}`.
