package loom_test

import (
	"fmt"
	"testing"

	"github.com/andrewbreksa/loom"
)

// ── Env ────────────────────────────────────────────────────────────────────

func TestEnvImmutable(t *testing.T) {
	e1 := loom.NewEnv().Set("x", 1)
	e2 := e1.Set("x", 2)
	if loom.Int(e1.Get("x")) != 1 {
		t.Error("e1 mutated")
	}
	if loom.Int(e2.Get("x")) != 2 {
		t.Error("e2 wrong")
	}
}

func TestEnvNamespace(t *testing.T) {
	e := loom.NewEnv().
		Set("pit.1.1.stones", 4).
		Set("pit.1.2.stones", 4).
		Set("pit.2.1.stones", 4)
	ns := e.Namespace("pit.1")
	if len(ns) != 2 {
		t.Errorf("expected 2, got %d", len(ns))
	}
	if _, ok := ns["pit.2.1.stones"]; ok {
		t.Error("pit.2 leaked into pit.1")
	}
}

func TestEnvDiff(t *testing.T) {
	e1 := loom.NewEnv().Set("x", 1).Set("y", 2)
	e2 := e1.Set("y", 3).Set("z", 4)
	diff := e1.Diff(e2)
	keys := map[string]bool{}
	for _, r := range diff {
		keys[r.Key] = true
	}
	if !keys["y"] {
		t.Error("y should have changed")
	}
	if !keys["z"] {
		t.Error("z should be new")
	}
	if keys["x"] {
		t.Error("x unchanged")
	}
}

func TestEnvLength(t *testing.T) {
	e := loom.NewEnv().
		Set("items.0", "a").Set("items.1", "b").Set("items.2", "c").
		Set("other", "x")
	if e.Length("items") != 3 {
		t.Errorf("expected 3, got %d", e.Length("items"))
	}
}

// ── Sugar ──────────────────────────────────────────────────────────────────

func TestSpread(t *testing.T) {
	decls := loom.Spread("pit.1.{N}.stones", 4, loom.IntRange("N", 1, 4))
	if len(decls) != 3 {
		t.Errorf("expected 3, got %d", len(decls))
	}
	keys := map[string]bool{}
	for _, d := range decls {
		keys[d.Key] = true
	}
	for _, k := range []string{"pit.1.1.stones", "pit.1.2.stones", "pit.1.3.stones"} {
		if !keys[k] {
			t.Errorf("missing %s", k)
		}
	}
}

func TestSpread2D(t *testing.T) {
	decls := loom.Spread("cells.{R}.{C}", "empty",
		loom.IntRange("R", 0, 3), loom.IntRange("C", 0, 3))
	if len(decls) != 9 {
		t.Errorf("expected 9, got %d", len(decls))
	}
}

func TestChain(t *testing.T) {
	decls := loom.Chain("turn.after", []string{"alice", "bob", "carol"}, true)
	m := map[string]any{}
	for _, d := range decls {
		m[d.Key] = d.Value
	}
	if m["turn.after.alice"] != "bob" {
		t.Error("alice→bob")
	}
	if m["turn.after.bob"] != "carol" {
		t.Error("bob→carol")
	}
	if m["turn.after.carol"] != "alice" {
		t.Error("carol→alice")
	}
}

func TestPair(t *testing.T) {
	decls := loom.Pair("opposite", [][2]string{{"north", "south"}})
	m := map[string]any{}
	for _, d := range decls {
		m[d.Key] = d.Value
	}
	if m["opposite.north"] != "south" {
		t.Error("north→south")
	}
	if m["opposite.south"] != "north" {
		t.Error("south→north")
	}
}

// ── Actions ────────────────────────────────────────────────────────────────

func TestActionPermitEnforced(t *testing.T) {
	rt := loom.New().
		Ref("wallet.alice", 10).Ref("wallet.bob", 0).
		Action("transfer", transferAction{}).
		Build()

	err := rt.Dispatch("transfer", map[string]any{"from": "wallet.alice", "to": "wallet.bob", "amount": 100})
	if err == nil {
		t.Fatal("expected PermitError")
	}
	if _, ok := err.(loom.PermitError); !ok {
		t.Errorf("expected PermitError, got %T", err)
	}
}

func TestActionEffectApplies(t *testing.T) {
	rt := loom.New().
		Ref("wallet.alice", 100).Ref("wallet.bob", 0).
		Action("transfer", transferAction{}).
		Build()

	if err := rt.Dispatch("transfer", map[string]any{"from": "wallet.alice", "to": "wallet.bob", "amount": 30}); err != nil {
		t.Fatal(err)
	}
	if loom.Int(rt.Get("wallet.alice")) != 70 {
		t.Errorf("alice: want 70, got %v", rt.Get("wallet.alice"))
	}
	if loom.Int(rt.Get("wallet.bob")) != 30 {
		t.Errorf("bob: want 30, got %v", rt.Get("wallet.bob"))
	}
}

// ── Watch ──────────────────────────────────────────────────────────────────

func TestWatchFiresOnChange(t *testing.T) {
	rt := loom.New().
		Ref("score", 0).Ref("high_score", 0).
		Action("set_score", loom.NewAction(
			func(_ loom.StateView, _ map[string]any) bool { return true },
			func(s loom.StateView, args map[string]any) []loom.Rebind {
				return []loom.Rebind{s.Rebind("score", args["value"])}
			},
		)).
		Watch("score", func(s loom.StateView, _ string, value any) []loom.Rebind {
			if v := loom.Int(value); v > loom.Int(s.Get("high_score")) {
				return []loom.Rebind{s.Rebind("high_score", v)}
			}
			return nil
		}).
		Build()

	rt.Dispatch("set_score", map[string]any{"value": 50})
	if loom.Int(rt.Get("high_score")) != 50 {
		t.Error("high_score should be 50")
	}
	rt.Dispatch("set_score", map[string]any{"value": 30})
	if loom.Int(rt.Get("high_score")) != 50 {
		t.Error("should still be 50")
	}
	rt.Dispatch("set_score", map[string]any{"value": 100})
	if loom.Int(rt.Get("high_score")) != 100 {
		t.Error("should be 100")
	}
}

func TestWatchWildcard(t *testing.T) {
	rt := loom.New().
		Ref("player.alice.health", 100).Ref("player.bob.health", 100).
		Ref("game.state", "active").
		Action("damage", loom.NewAction(
			func(_ loom.StateView, _ map[string]any) bool { return true },
			func(s loom.StateView, args map[string]any) []loom.Rebind {
				target := loom.String(args["target"])
				amount := loom.Int(args["amount"])
				current := loom.Int(s.Get("player." + target + ".health"))
				return []loom.Rebind{s.Rebind("player."+target+".health", current-amount)}
			},
		)).
		Watch("player.*.health", func(s loom.StateView, _ string, value any) []loom.Rebind {
			if loom.Int(value) <= 0 {
				return []loom.Rebind{s.Rebind("game.state", "over")}
			}
			return nil
		}).
		Build()

	rt.Dispatch("damage", map[string]any{"target": "alice", "amount": 100})
	if loom.String(rt.Get("game.state")) != "over" {
		t.Errorf("state should be over, got %v", rt.Get("game.state"))
	}
}

// ── Derived ────────────────────────────────────────────────────────────────

func TestDerivedRecomputes(t *testing.T) {
	rt := loom.New().
		Ref("a", 3).Ref("b", 4).
		Derived("hypotenuse", func(s loom.StateView) any {
			a, b := float64(loom.Int(s.Get("a"))), float64(loom.Int(s.Get("b")))
			return a*a + b*b
		}).
		Action("set_a", loom.NewAction(
			func(_ loom.StateView, _ map[string]any) bool { return true },
			func(s loom.StateView, args map[string]any) []loom.Rebind {
				return []loom.Rebind{s.Rebind("a", args["value"])}
			},
		)).
		Build()

	if loom.Float(rt.Get("hypotenuse")) != 25.0 {
		t.Errorf("expected 25, got %v", rt.Get("hypotenuse"))
	}
	rt.Dispatch("set_a", map[string]any{"value": 5})
	if loom.Float(rt.Get("hypotenuse")) != 41.0 {
		t.Errorf("expected 41, got %v", rt.Get("hypotenuse"))
	}
}

// ── Pattern ────────────────────────────────────────────────────────────────

func TestPattern(t *testing.T) {
	rt := loom.New().
		Ref("game.state", "active").Ref("game.winner", "none").Ref("score", 0).
		Pattern("wins", func(s loom.StateView, args ...any) []loom.Rebind {
			return []loom.Rebind{s.Rebind("game.state", "over"), s.Rebind("game.winner", args[0].(string))}
		}).
		Watch("score", func(s loom.StateView, _ string, value any) []loom.Rebind {
			if loom.Int(value) >= 10 {
				return s.Pattern("wins", "alice")
			}
			return nil
		}).
		Action("add_score", loom.NewAction(
			func(_ loom.StateView, _ map[string]any) bool { return true },
			func(s loom.StateView, _ map[string]any) []loom.Rebind {
				return []loom.Rebind{s.Rebind("score", loom.Int(s.Get("score"))+10)}
			},
		)).
		Build()

	rt.Dispatch("add_score", nil)
	if loom.String(rt.Get("game.state")) != "over" {
		t.Error("state should be over")
	}
	if loom.String(rt.Get("game.winner")) != "alice" {
		t.Error("winner should be alice")
	}
}

// ── History and Replay ─────────────────────────────────────────────────────

func TestHistoryAndReplay(t *testing.T) {
	l := loom.New().
		Ref("x", 0).
		Action("inc", loom.NewAction(
			func(_ loom.StateView, _ map[string]any) bool { return true },
			func(s loom.StateView, _ map[string]any) []loom.Rebind {
				return []loom.Rebind{s.Rebind("x", loom.Int(s.Get("x"))+1)}
			},
		))

	rt := l.Build()
	rt.Dispatch("inc", nil)
	rt.Dispatch("inc", nil)
	rt.Dispatch("inc", nil)

	if loom.Int(rt.Get("x")) != 3 {
		t.Errorf("expected 3, got %v", rt.Get("x"))
	}
	if len(rt.History()) != 3 {
		t.Errorf("expected 3 history entries, got %d", len(rt.History()))
	}

	rt2 := l.Build()
	if err := rt2.Replay(rt.EventLog()); err != nil {
		t.Fatal(err)
	}
	if loom.Int(rt2.Get("x")) != 3 {
		t.Errorf("replay: expected 3, got %v", rt2.Get("x"))
	}
}

// ── Tic Tac Toe ────────────────────────────────────────────────────────────

func TestTicTacToe(t *testing.T) {
	l := loom.New()

	for _, d := range loom.Spread("cells.{R}.{C}.owner", "none",
		loom.IntRange("R", 0, 3), loom.IntRange("C", 0, 3)) {
		l.Ref(d.Key, d.Value)
	}
	for _, d := range loom.Spread("cells.{R}.{C}.placed", false,
		loom.IntRange("R", 0, 3), loom.IntRange("C", 0, 3)) {
		l.Ref(d.Key, d.Value)
	}
	for _, d := range loom.Chain("turn.after", []string{"x", "o"}, true) {
		l.Ref(d.Key, d.Value)
	}

	l.Ref("turn.actor", "x").Ref("game.state", "active").Ref("game.winner", "none")

	l.Pattern("wins", func(s loom.StateView, args ...any) []loom.Rebind {
		return []loom.Rebind{s.Rebind("game.state", "over"), s.Rebind("game.winner", args[0].(string))}
	})

	lines := [][3][2]int{
		{{0, 0}, {0, 1}, {0, 2}}, {{1, 0}, {1, 1}, {1, 2}}, {{2, 0}, {2, 1}, {2, 2}},
		{{0, 0}, {1, 0}, {2, 0}}, {{0, 1}, {1, 1}, {2, 1}}, {{0, 2}, {1, 2}, {2, 2}},
		{{0, 0}, {1, 1}, {2, 2}}, {{0, 2}, {1, 1}, {2, 0}},
	}

	l.Watch("cells.*.*.owner", func(s loom.StateView, _ string, _ any) []loom.Rebind {
		for _, line := range lines {
			o := [3]string{}
			for i, c := range line {
				o[i] = loom.String(s.Get(fmt.Sprintf("cells.%d.%d.owner", c[0], c[1])))
			}
			if o[0] != "none" && o[0] == o[1] && o[1] == o[2] {
				return s.Pattern("wins", o[0])
			}
		}
		return nil
	})

	l.Action("place", loom.NewAction(
		func(s loom.StateView, args map[string]any) bool {
			r, c := loom.Int(args["row"]), loom.Int(args["col"])
			return loom.String(s.Get("game.state")) == "active" &&
				!loom.Bool(s.Get(fmt.Sprintf("cells.%d.%d.placed", r, c)))
		},
		func(s loom.StateView, args map[string]any) []loom.Rebind {
			r, c := loom.Int(args["row"]), loom.Int(args["col"])
			actor := loom.String(s.Get("turn.actor"))
			cell := fmt.Sprintf("cells.%d.%d", r, c)
			return []loom.Rebind{
				s.Rebind(cell+".owner", actor),
				s.Rebind(cell+".placed", true),
				s.Rebind("turn.actor", loom.String(s.Get("turn.after."+actor))),
			}
		},
	))

	rt := l.Build()
	place := func(r, c int) { rt.Dispatch("place", map[string]any{"row": r, "col": c}) }

	place(0, 0)
	place(1, 0)
	place(0, 1)
	place(1, 1)
	place(0, 2) // x wins top row

	if loom.String(rt.Get("game.state")) != "over" {
		t.Errorf("want over, got %v", rt.Get("game.state"))
	}
	if loom.String(rt.Get("game.winner")) != "x" {
		t.Errorf("want x, got %v", rt.Get("game.winner"))
	}
}

// ── helpers ────────────────────────────────────────────────────────────────

type transferAction struct{}

func (a transferAction) Permits(s loom.StateView, args map[string]any) bool {
	return loom.Int(s.Get(loom.String(args["from"]))) >= loom.Int(args["amount"])
}

func (a transferAction) Effect(s loom.StateView, args map[string]any) []loom.Rebind {
	from, to, amount := loom.String(args["from"]), loom.String(args["to"]), loom.Int(args["amount"])
	return []loom.Rebind{
		s.Rebind(from, loom.Int(s.Get(from))-amount),
		s.Rebind(to, loom.Int(s.Get(to))+amount),
	}
}

// ── Chess ──────────────────────────────────────────────────────────────────

func TestChess(t *testing.T) {
	l := loom.New()

	// Standard starting position. Piece encoding: color prefix (w/b) + type (P N B R Q K).
	// Empty squares omitted from startPos — all 64 keys are declared; empty = "".
	startPos := map[string]string{
		"a1": "wR", "b1": "wN", "c1": "wB", "d1": "wQ", "e1": "wK", "f1": "wB", "g1": "wN", "h1": "wR",
		"a2": "wP", "b2": "wP", "c2": "wP", "d2": "wP", "e2": "wP", "f2": "wP", "g2": "wP", "h2": "wP",
		"a7": "bP", "b7": "bP", "c7": "bP", "d7": "bP", "e7": "bP", "f7": "bP", "g7": "bP", "h7": "bP",
		"a8": "bR", "b8": "bN", "c8": "bB", "d8": "bQ", "e8": "bK", "f8": "bB", "g8": "bN", "h8": "bR",
	}
	for _, file := range []string{"a", "b", "c", "d", "e", "f", "g", "h"} {
		for _, rank := range []string{"1", "2", "3", "4", "5", "6", "7", "8"} {
			sq := file + rank
			l.Ref("board."+sq, startPos[sq]) // "" for empty squares
		}
	}

	l.Ref("turn.current", "w")
	l.Ref("game.state", "active")

	l.Pattern("end_game", func(s loom.StateView, args ...any) []loom.Rebind {
		return []loom.Rebind{s.Rebind("game.state", args[0].(string))}
	})

	l.Derived("game.in_check", func(s loom.StateView) any {
		board := chessBoardMap(s)
		color := loom.String(s.Get("turn.current"))
		opp := "w"
		if color == "w" {
			opp = "b"
		}
		kingSq := chessFindKing(color, board)
		return chessIsAttacked(kingSq, opp, board)
	})

	l.Watch("board.*", func(s loom.StateView, _ string, _ any) []loom.Rebind {
		color := loom.String(s.Get("turn.current"))
		opp := "w"
		if color == "w" {
			opp = "b"
		}
		board := chessBoardMap(s)
		kingSq := chessFindKing(color, board)
		if !chessIsAttacked(kingSq, opp, board) {
			return nil
		}
		if !chessHasLegalMoves(color, board) {
			return s.Pattern("end_game", "checkmate")
		}
		return s.Pattern("end_game", "check")
	})

	l.Action("move", loom.NewAction(
		func(s loom.StateView, args map[string]any) bool {
			if loom.String(s.Get("game.state")) == "checkmate" {
				return false
			}
			from := loom.String(args["from"])
			to := loom.String(args["to"])
			board := chessBoardMap(s)
			piece := board[from]
			if piece == "" || chessPieceColor(piece) != loom.String(s.Get("turn.current")) {
				return false
			}
			for _, m := range chessLegalMoves(from, board) {
				if m == to {
					return true
				}
			}
			return false
		},
		func(s loom.StateView, args map[string]any) []loom.Rebind {
			from := loom.String(args["from"])
			to := loom.String(args["to"])
			piece := loom.String(s.Get("board." + from))
			color := loom.String(s.Get("turn.current"))
			next := "b"
			if color == "b" {
				next = "w"
			}
			return []loom.Rebind{
				s.Rebind("board."+from, ""),
				s.Rebind("board."+to, piece),
				s.Rebind("turn.current", next),
			}
		},
	))

	rt := l.Build()
	move := func(from, to string) {
		t.Helper()
		if err := rt.Dispatch("move", map[string]any{"from": from, "to": to}); err != nil {
			t.Fatalf("move %s→%s failed: %v", from, to, err)
		}
	}

	// Scholar's Mate: white wins in 4 moves (7 half-moves)
	move("e2", "e4") // 1. e4
	move("e7", "e5") // 1... e5
	move("f1", "c4") // 2. Bc4
	move("b8", "c6") // 2... Nc6
	move("d1", "h5") // 3. Qh5
	move("g8", "f6") // 3... Nf6?? (the losing blunder)
	move("h5", "f7") // 4. Qxf7# (checkmate)

	if loom.String(rt.Get("game.state")) != "checkmate" {
		t.Errorf("want checkmate, got %v", rt.Get("game.state"))
	}
	// Black king on e8 is in checkmate — it is black's turn to move but no legal escape exists.
	if loom.String(rt.Get("turn.current")) != "b" {
		t.Errorf("want black to move (checkmated), got %v", rt.Get("turn.current"))
	}
	// Derived should reflect the mated side is in check.
	if !loom.Bool(rt.Get("game.in_check")) {
		t.Error("game.in_check should be true")
	}
}

// ── Chess helpers ──────────────────────────────────────────────────────────

func chessFile(sq string) int { return int(sq[0] - 'a') } // 0=a … 7=h
func chessRank(sq string) int { return int(sq[1] - '1') } // 0=rank1 … 7=rank8

func chessSq(f, r int) string {
	if f < 0 || f > 7 || r < 0 || r > 7 {
		return ""
	}
	return string([]byte{byte('a' + f), byte('1' + r)})
}

func chessBoardMap(s loom.StateView) map[string]string {
	ns := s.Namespace("board")
	m := make(map[string]string, len(ns))
	for k, v := range ns {
		m[k[len("board."):]] = loom.String(v)
	}
	return m
}

func chessPieceColor(p string) string {
	if p == "" {
		return ""
	}
	return string(p[0])
}

func chessPieceType(p string) string {
	if p == "" {
		return ""
	}
	return string(p[1])
}

// chessLegalMoves returns pseudo-legal destinations for the piece at sq.
// Does not filter moves that leave own king in check.
func chessLegalMoves(sq string, board map[string]string) []string {
	piece := board[sq]
	if piece == "" {
		return nil
	}
	color := chessPieceColor(piece)
	pt := chessPieceType(piece)
	f, r := chessFile(sq), chessRank(sq)

	var moves []string

	add := func(target string) {
		if target == "" {
			return
		}
		t := board[target]
		if t == "" || chessPieceColor(t) != color {
			moves = append(moves, target)
		}
	}

	addRay := func(df, dr int) {
		for step := 1; step <= 7; step++ {
			target := chessSq(f+df*step, r+dr*step)
			if target == "" {
				break
			}
			t := board[target]
			if t != "" {
				if chessPieceColor(t) != color {
					moves = append(moves, target)
				}
				break
			}
			moves = append(moves, target)
		}
	}

	switch pt {
	case "P":
		dir, startRank := 1, 1
		if color == "b" {
			dir, startRank = -1, 6
		}
		fwd := chessSq(f, r+dir)
		if fwd != "" && board[fwd] == "" {
			moves = append(moves, fwd)
			if r == startRank {
				fwd2 := chessSq(f, r+dir*2)
				if fwd2 != "" && board[fwd2] == "" {
					moves = append(moves, fwd2)
				}
			}
		}
		for _, df := range []int{-1, 1} {
			cap := chessSq(f+df, r+dir)
			if cap != "" && board[cap] != "" && chessPieceColor(board[cap]) != color {
				moves = append(moves, cap)
			}
		}
	case "N":
		for _, d := range [][2]int{{2, 1}, {2, -1}, {-2, 1}, {-2, -1}, {1, 2}, {1, -2}, {-1, 2}, {-1, -2}} {
			add(chessSq(f+d[0], r+d[1]))
		}
	case "B":
		for _, d := range [][2]int{{1, 1}, {1, -1}, {-1, 1}, {-1, -1}} {
			addRay(d[0], d[1])
		}
	case "R":
		for _, d := range [][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
			addRay(d[0], d[1])
		}
	case "Q":
		for _, d := range [][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}, {1, 1}, {1, -1}, {-1, 1}, {-1, -1}} {
			addRay(d[0], d[1])
		}
	case "K":
		for _, d := range [][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}, {1, 1}, {1, -1}, {-1, 1}, {-1, -1}} {
			add(chessSq(f+d[0], r+d[1]))
		}
	}
	return moves
}

func chessFindKing(color string, board map[string]string) string {
	for sq, piece := range board {
		if piece == color+"K" {
			return sq
		}
	}
	return ""
}

func chessIsAttacked(sq string, byColor string, board map[string]string) bool {
	for s, piece := range board {
		if piece == "" || chessPieceColor(piece) != byColor {
			continue
		}
		for _, target := range chessLegalMoves(s, board) {
			if target == sq {
				return true
			}
		}
	}
	return false
}

// chessHasLegalMoves returns true if color has at least one move that does not
// leave their own king in check.
func chessHasLegalMoves(color string, board map[string]string) bool {
	opp := "w"
	if color == "w" {
		opp = "b"
	}
	for sq, piece := range board {
		if piece == "" || chessPieceColor(piece) != color {
			continue
		}
		for _, to := range chessLegalMoves(sq, board) {
			sim := make(map[string]string, len(board))
			for k, v := range board {
				sim[k] = v
			}
			sim[to] = piece
			sim[sq] = ""
			kingSq := chessFindKing(color, sim)
			if !chessIsAttacked(kingSq, opp, sim) {
				return true
			}
		}
	}
	return false
}
