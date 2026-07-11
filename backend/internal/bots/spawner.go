// Package internalbots wires the pkg/bots decision pipeline into a live game
// room: which mode a room runs, how many bots it wants, where their brains
// come from, and mode-specific (re)spawn rules (zombies waves, boss respawn,
// practice respawn).
package internalbots

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"zhuch/pkg/bots"
	"zhuch/pkg/brain"
	"zhuch/pkg/engine"
)

type Mode string

const (
	ModeFFA      Mode = "ffa" // no bots
	ModeZombies  Mode = "zombies"
	ModeBoss     Mode = "boss"
	ModePractice Mode = "practice"
)

// ValidMode reports whether s is one of the known mode strings.
func ValidMode(s string) bool {
	switch Mode(s) {
	case ModeFFA, ModeZombies, ModeBoss, ModePractice:
		return true
	default:
		return false
	}
}

// Mode-specific tuning (contracts have no numbers for these; picked here).
const (
	zombieTarget         = 25
	zombieWaveSize       = 5
	zombieWaveInterval   = 100 // ticks
	zombieHealthFraction = 0.3
	zombieFireCooldown   = 1 << 30 // effectively never; melee only

	bossHealthMultiplier = 20
	bossRadiusMultiplier = 3
	bossRespawnDelay     = 200 // ticks

	practiceTarget       = 4
	practiceRespawnDelay = 100 // ticks
)

// entry tracks one live-or-pending bot slot.
type entry struct {
	bot       *bots.Bot
	respawnAt int // 0 = no respawn scheduled
}

// BotSet owns the bots for a single room. Act is called once per tick, on
// the game-loop goroutine, immediately before Game.Tick (see manager.go) —
// same locking assumption as pkg/bots/obs.go: never call it concurrently
// with Game.Tick.
type BotSet struct {
	Mode      Mode
	ModelName string
	Target    int // desired bot count (zombies/practice); boss is always 1

	newPolicy func() bots.Policy // nil for ffa (no bots to build)

	entries    []*entry
	spawnIndex int // monotonic: bot naming + Offset stagger
}

// NewBotSet builds a BotSet for a room. mode "" behaves like "ffa" (no
// bots). count <= 0 uses the mode's default target. modelName is loaded
// from backend/models/<name>.json via pkg/brain; if that file is missing or
// invalid, bots fall back to the frozen ScriptedPolicy (contracts §9).
func NewBotSet(mode string, count int, modelName string) *BotSet {
	m := Mode(mode)
	if m == "" {
		m = ModeFFA
	}
	bs := &BotSet{Mode: m, ModelName: modelName, Target: count}
	if m == ModeFFA {
		return bs
	}
	bs.newPolicy = loadPolicyFactory(modelName)
	switch m {
	case ModeZombies:
		if bs.Target <= 0 {
			bs.Target = zombieTarget
		}
	case ModeBoss:
		bs.Target = 1
	case ModePractice:
		if bs.Target <= 0 {
			bs.Target = practiceTarget
		}
	}
	return bs
}

// loadPolicyFactory returns a function that builds a fresh Policy per bot.
// MLPPolicy just wraps a shared *brain.MLP (Forward doesn't mutate, so
// sharing is safe); Scripted keeps per-instance strafe state, so each bot
// gets its own.
func loadPolicyFactory(modelName string) func() bots.Policy {
	path := filepath.Join("models", modelName+".json")
	f, err := os.Open(path)
	if err != nil {
		slog.Warn("bot model file not found, falling back to scripted policy", "model", modelName, "path", path, "error", err)
		return func() bots.Policy { return &bots.Scripted{} }
	}
	defer f.Close()

	m, err := brain.LoadModel(f)
	if err != nil {
		slog.Warn("bot model failed to load, falling back to scripted policy", "model", modelName, "path", path, "error", err)
		return func() bots.Policy { return &bots.Scripted{} }
	}
	slog.Info("bot model loaded", "model", modelName, "sizes", m.Sizes)
	return func() bots.Policy { return bots.MLPPolicy{M: m} }
}

// BotCount reports how many bot tanks this set currently tracks (used for
// the zhuch_bots metric).
func (bs *BotSet) BotCount() int { return len(bs.entries) }

// Act runs mode-specific (re)spawn logic then steps every tracked bot.
func (bs *BotSet) Act(g *engine.Game, tick int) {
	switch bs.Mode {
	case ModeZombies:
		bs.actZombies(g, tick)
	case ModeBoss:
		bs.actBoss(g, tick)
	case ModePractice:
		bs.actPractice(g, tick)
	}
	for _, e := range bs.entries {
		e.bot.Step(g, tick)
	}
}

func randomSpawnPos(g *engine.Game) engine.Vector2 {
	// Seeded-random with a 100-unit wall margin, same convention as arena.
	return engine.Vector2{
		X: 100 + g.Rng.Float64()*(g.Config.MapWidth-200),
		Y: 100 + g.Rng.Float64()*(g.Config.MapHeight-200),
	}
}

func (bs *BotSet) newBot(t *engine.Tank) *bots.Bot {
	i := bs.spawnIndex
	bs.spawnIndex++
	return &bots.Bot{Tank: t, Brain: bs.newPolicy(), DecideEvery: 4, Offset: i % 4}
}

// --- zombies: wave-spawn up to target, no individual respawn ---

func (bs *BotSet) actZombies(g *engine.Game, tick int) {
	if tick%zombieWaveInterval != 0 {
		return
	}
	// Drop dead zombies before counting so the set doesn't grow forever.
	alive := bs.entries[:0]
	for _, e := range bs.entries {
		if e.bot.Tank.IsAlive() {
			alive = append(alive, e)
		}
	}
	bs.entries = alive

	deficit := bs.Target - len(bs.entries)
	if deficit <= 0 {
		return
	}
	n := zombieWaveSize
	if n > deficit {
		n = deficit
	}
	for i := 0; i < n; i++ {
		bs.entries = append(bs.entries, &entry{bot: bs.spawnZombie(g)})
	}
}

func (bs *BotSet) spawnZombie(g *engine.Game) *bots.Bot {
	name := fmt.Sprintf("zombie-%d", bs.spawnIndex+1)
	t := engine.NewTank(name, randomSpawnPos(g), &g.Config)
	t.MaxHealth *= zombieHealthFraction
	t.Health = t.MaxHealth
	t.FireCooldown = zombieFireCooldown
	t.IsBot = true
	g.SpawnTank(t)
	return bs.newBot(t)
}

// --- boss: single bot, respawns bossRespawnDelay ticks after death ---

func (bs *BotSet) actBoss(g *engine.Game, tick int) {
	if len(bs.entries) == 0 {
		bs.entries = append(bs.entries, &entry{bot: bs.spawnBoss(g)})
		return
	}
	e := bs.entries[0]
	if e.bot.Tank.IsAlive() {
		return
	}
	if e.respawnAt == 0 {
		e.respawnAt = tick + bossRespawnDelay
		return
	}
	if tick >= e.respawnAt {
		e.bot = bs.spawnBoss(g)
		e.respawnAt = 0
	}
}

func (bs *BotSet) spawnBoss(g *engine.Game) *bots.Bot {
	t := engine.NewTank("boss", randomSpawnPos(g), &g.Config)
	if c, ok := t.Object.(*engine.Circle); ok {
		c.Radius *= bossRadiusMultiplier
	}
	t.MaxHealth *= bossHealthMultiplier
	t.Health = t.MaxHealth
	t.IsBot = true
	g.SpawnTank(t)
	return bs.newBot(t)
}

// --- practice: fixed roster, each bot respawns practiceRespawnDelay ticks
// after its own death ---

func (bs *BotSet) actPractice(g *engine.Game, tick int) {
	for len(bs.entries) < bs.Target {
		bs.entries = append(bs.entries, &entry{bot: bs.spawnPracticeBot(g)})
	}
	for _, e := range bs.entries {
		if e.bot.Tank.IsAlive() {
			e.respawnAt = 0
			continue
		}
		if e.respawnAt == 0 {
			e.respawnAt = tick + practiceRespawnDelay
			continue
		}
		if tick >= e.respawnAt {
			e.bot = bs.spawnPracticeBot(g)
			e.respawnAt = 0
		}
	}
}

func (bs *BotSet) spawnPracticeBot(g *engine.Game) *bots.Bot {
	name := fmt.Sprintf("bot-%d", bs.spawnIndex+1)
	t := engine.NewTank(name, randomSpawnPos(g), &g.Config)
	t.IsBot = true
	g.SpawnTank(t)
	return bs.newBot(t)
}
