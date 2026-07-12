package engine_test

import (
	"fmt"
	"testing"

	"zhuch/pkg/bots"
	"zhuch/pkg/engine"
)

// runSeeded plays 500 ticks of a seeded game with 2 scripted bots and
// returns a full state snapshot.
func runSeeded(seed int64) []string {
	g := engine.NewGameSeeded(engine.DefaultConfig(), seed)
	var bs []*bots.Bot
	for i := 0; i < 2; i++ {
		tk := engine.NewTank(fmt.Sprintf("s%d", i), engine.Vector2{X: 600 + 800*float64(i), Y: 1000}, &g.Config)
		g.SpawnTank(tk)
		bs = append(bs, &bots.Bot{Tank: tk, Brain: &bots.Scripted{}, DecideEvery: 4, Offset: i % 4})
	}
	for tick := 0; tick < 500; tick++ {
		for _, b := range bs {
			b.Step(g, tick)
		}
		g.Tick()
	}
	snap := make([]string, 0, len(g.Entities))
	for _, e := range g.Entities {
		pos := e.GetPosition()
		line := fmt.Sprintf("%s %.17g %.17g %.17g", e.GetID(), pos.X, pos.Y, e.GetHealth())
		if t, ok := e.(*engine.Tank); ok {
			line += fmt.Sprintf(" score=%.17g kills=%d", t.Score, t.Kills)
		}
		snap = append(snap, line)
	}
	return snap
}

func TestSeededDeterminism(t *testing.T) {
	a, b := runSeeded(42), runSeeded(42)
	if len(a) != len(b) {
		t.Fatalf("entity counts differ: %d vs %d", len(a), len(b))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("replay diverged at entity %d:\n%s\nvs\n%s", i, a[i], b[i])
		}
	}
}

// TestConcurrentSpawn guards the g.mu locking in SpawnTank/SpawnBullet
// against Tick (meaningful under -race).
func TestConcurrentSpawn(t *testing.T) {
	g := engine.NewGameSeeded(engine.DefaultConfig(), 7)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 100; i++ {
			g.Tick()
		}
	}()
	for i := 0; i < 100; i++ {
		tk := engine.NewTank("t", engine.Vector2{X: 500, Y: 500}, &g.Config)
		// Fire before spawning: once spawned, Tick owns the tank's state.
		if b := tk.Fire(0, 1000); b != nil {
			g.SpawnBullet(b)
		}
		g.SpawnTank(tk)
	}
	<-done
}
