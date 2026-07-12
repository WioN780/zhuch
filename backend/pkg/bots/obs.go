// Package bots implements the bot observation/action pipeline (contracts §1, §2)
// and the scripted baseline (§9).
package bots

import (
	"math"
	"sort"

	"zhuch/pkg/engine"
)

// ObsSize is the observation vector length (contracts §1, v2: obstacles added).
const ObsSize = 90

const (
	numSlots  = 8  // K nearest entities/obstacles
	slotLen   = 10 // floats per slot (v2: +is_obstacle)
	slotsBase = 10 // first slot offset
)

// cand is one candidate for a K-nearest slot: a living entity or a static
// obstacle, normalized to the same shape so fillSlot doesn't care which.
type cand struct {
	pos    engine.Vector2
	vel    engine.Vector2 // zero for obstacles
	radius float64
	health float64 // fraction; obstacles report 1.0
	kind   int     // 0 tank, 1 bullet, 2 food, 3 obstacle (one-hot offset)
	d2     float64
}

// BuildObservation fills buf (len >= ObsSize) per contracts §1.
//
// Locking: it reads g.Entities/g.Arena.Obstacles WITHOUT taking g.mu. Callers
// must run on the game-loop goroutine between Tick calls (as the arena does)
// — never concurrently with Tick.
func BuildObservation(g *engine.Game, t *engine.Tank, buf []float64) {
	buf = buf[:ObsSize]
	for i := range buf {
		buf[i] = 0
	}
	cfg := &g.Config
	pos := t.GetPosition()

	// Own state (unchanged, contracts §1).
	buf[0] = t.Health / t.MaxHealth
	if cfg.TankMaxSpeed > 0 {
		buf[1] = t.Vel.X / cfg.TankMaxSpeed
		buf[2] = t.Vel.Y / cfg.TankMaxSpeed
	}
	buf[3] = math.Sin(t.Orientation)
	buf[4] = math.Cos(t.Orientation)
	if t.FireCooldown > 0 {
		remaining := float64(t.FireCooldown - (g.CurrentTick - t.LastFireTick))
		buf[5] = clip01(remaining / float64(t.FireCooldown))
	}
	vr := t.ViewRange
	buf[6] = clip01(pos.X / vr)
	buf[7] = clip01((cfg.MapWidth - pos.X) / vr)
	buf[8] = clip01(pos.Y / vr)
	buf[9] = clip01((cfg.MapHeight - pos.Y) / vr)

	// K nearest entities (excluding self) + static obstacles within
	// ViewRange, ascending distance.
	vr2 := vr * vr
	cands := make([]cand, 0, len(g.Entities)+len(g.Arena.Obstacles))
	for _, e := range g.Entities {
		if e == engine.Entity(t) {
			continue
		}
		d2 := pos.DistanceSquaredTo(e.GetPosition())
		if d2 > vr2 {
			continue
		}
		kind := 0
		switch e.(type) {
		case *engine.Tank:
			kind = 0
		case *engine.Bullet:
			kind = 1
		case *engine.Food:
			kind = 2
		default:
			continue
		}
		healthFrac := 0.0
		if mh := e.GetMaxHealth(); mh > 0 {
			healthFrac = e.GetHealth() / mh
		}
		cands = append(cands, cand{
			pos: e.GetPosition(), vel: e.GetVelocity(),
			radius: engine.BoundingRadius(e.GetGeom()), health: healthFrac,
			kind: kind, d2: d2,
		})
	}
	for _, obs := range g.Arena.Obstacles {
		opos := obs.GetCenter()
		d2 := pos.DistanceSquaredTo(opos)
		if d2 > vr2 {
			continue
		}
		cands = append(cands, cand{
			pos: opos, radius: engine.BoundingRadius(obs),
			health: 1.0, kind: 3, d2: d2,
		})
	}
	// Stable so equal distances keep insertion order → deterministic.
	sort.SliceStable(cands, func(i, j int) bool { return cands[i].d2 < cands[j].d2 })

	for k := 0; k < numSlots && k < len(cands); k++ {
		fillSlot(buf[slotsBase+k*slotLen:slotsBase+(k+1)*slotLen], cands[k], t, cfg)
	}
}

// fillSlot writes one 10-float slot (contracts §1, v2). slot has len 10.
func fillSlot(slot []float64, c cand, t *engine.Tank, cfg *engine.GameConfig) {
	rel := c.pos.Sub(t.GetPosition())
	slot[0] = rel.X / t.ViewRange
	slot[1] = rel.Y / t.ViewRange
	if cfg.BulletMuzzleSpeed > 0 {
		relVel := c.vel.Sub(t.Vel)
		slot[2] = relVel.X / cfg.BulletMuzzleSpeed
		slot[3] = relVel.Y / cfg.BulletMuzzleSpeed
	}
	slot[4+c.kind] = 1 // is_tank/is_bullet/is_food/is_obstacle one-hot
	if cfg.TankRadius > 0 {
		slot[8] = c.radius / cfg.TankRadius
	}
	slot[9] = c.health
}

func clip01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
