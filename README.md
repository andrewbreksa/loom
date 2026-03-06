# Loom

Loom is a Go library for building reactive functional state machines with immutable state snapshots. You declare refs, actions, derived values, and watches, then dispatch actions through a runtime that applies pure state transitions and reactive cascades.

## Install

```bash
go get github.com/andrewbreksa/loom
```

```go
import "github.com/andrewbreksa/loom"
```

## Core Ideas

Loom follows a three-step lifecycle:

1. Assembly: declare refs, actions, watches, derived values, and patterns with `loom.New()`.
2. Build: create a runtime with `Build()`.
3. Dispatch: execute transitions with `Dispatch(name, args)`.

State is stored as an immutable flat map with dot-separated keys (for example, `player.alice.health`). Every change is represented as `[]Rebind`, which keeps transition logic pure and replayable.

## Quickstart (Runnable Example)

```go
package main

import (
	"fmt"

	"github.com/andrewbreksa/loom"
)

func main() {
	rt := loom.New().
		Ref("wallet.alice", 100).
		Ref("wallet.bob", 0).
		Action("transfer", loom.NewAction(
			func(s loom.StateView, args map[string]any) bool {
				from := loom.String(args["from"])
				amount := loom.Int(args["amount"])
				return loom.Int(s.Get(from)) >= amount
			},
			func(s loom.StateView, args map[string]any) []loom.Rebind {
				from := loom.String(args["from"])
				to := loom.String(args["to"])
				amount := loom.Int(args["amount"])
				return []loom.Rebind{
					s.Rebind(from, loom.Int(s.Get(from))-amount),
					s.Rebind(to, loom.Int(s.Get(to))+amount),
				}
			},
		)).
		Build()

	err := rt.Dispatch("transfer", map[string]any{
		"from": "wallet.alice",
		"to":   "wallet.bob",
		"amount": 30,
	})
	if err != nil {
		panic(err)
	}

	fmt.Println(rt.Get("wallet.alice")) // 70
	fmt.Println(rt.Get("wallet.bob"))   // 30
}
```

## The Seven Primitives

| Primitive | Purpose |
| --- | --- |
| `Ref` | Declare initial key/value state. |
| `Derived` | Pure value recomputed after state changes. |
| `Watch` | Reactive callback that runs when matching keys change. |
| `Action` | Guarded state transition (`Permits` + `Effect`). |
| `Pattern` | Named reusable effect (`[]Rebind` factory). |
| `For` | Namespace iteration via `ForEach(ns, fn)`. |
| `Apply` | Convert descriptions into `[]Rebind` via `StateView.Apply`. |

## Common Patterns

### Grid or Matrix Initialization

Use `Spread` to generate many refs from a template:

```go
decls := loom.Spread(
	"cells.{R}.{C}.owner",
	"none",
	loom.IntRange("R", 0, 3),
	loom.IntRange("C", 0, 3),
)
```

### Turn Order and Linked Sequences

Use `Chain` to encode next-player relationships:

```go
decls := loom.Chain("turn.after", []string{"alice", "bob", "carol"}, true)
```

### Symmetric Mappings

Use `Pair` for bidirectional lookups:

```go
decls := loom.Pair("opposite", [][2]string{{"north", "south"}})
```

## Runtime Behavior

`Dispatch(action, args)` runs this sequence:

1. Resolve action by name.
2. Execute `Permits(state, args)` guard.
3. Execute `Effect(state, args)` to produce `[]Rebind`.
4. Apply rebinds to create a new immutable state snapshot.
5. Recompute all `Derived` values.
6. Fire matching `Watch` handlers (including cascades).
7. Append event and history entries.

If `Permits` returns `false`, `Dispatch` returns `PermitError`.

## Event Sourcing and Replay

Runtimes keep history and an event log:

```go
events := rt.EventLog()
history := rt.History()

rt2 := loom.New().
	Ref("x", 0).
	Action("inc", loom.NewAction(
		func(_ loom.StateView, _ map[string]any) bool { return true },
		func(s loom.StateView, _ map[string]any) []loom.Rebind {
			return []loom.Rebind{s.Rebind("x", loom.Int(s.Get("x"))+1)}
		},
	)).
	Build()

_ = history
if err := rt2.Replay(events); err != nil {
	panic(err)
}
```

## Testing and Validation

```bash
go build ./...
go test ./...
go test -run TestTicTacToe ./...
go test -run TestChess ./...
```

## API Reference (At a Glance)

- Assembly: `New`, `Ref`, `Refs`, `Derived`, `Watch`, `Action`, `Pattern`, `Module`, `Build`
- Runtime: `Dispatch`, `Get`, `GetOr`, `Namespace`, `Length`, `Snapshot`, `History`, `EventLog`, `Replay`
- Helpers: `NewAction`, `R`, `ForEach`, `Spread`, `Chain`, `Pair`
- Type coercions: `String`, `Int`, `Float`, `Bool`, `Slice`

## License

This project is licensed under the GNU Affero General Public License v3.0 or later (`AGPL-3.0-or-later`).
See [LICENSE](LICENSE) for details.
