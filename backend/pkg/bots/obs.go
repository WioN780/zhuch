// Package bots implements the bot observation/action pipeline (contracts §1, §2)
// and the scripted baseline (§9).
package bots

import (
	"math"
	"sort"

	"zhuch/pkg/engine"
)

// ObsSize is the observation vector length (contracts §1).
const ObsSize = 82

const (
	numSlots  = 8  // K nearest entities
	slotLen   = 9  // floats per entity slot
	slotsBase = 10 // first slot offset
)

// BuildObservation fills buf (len >= ObsSize) per contracts §1.
//
// Locking: it reads g.Entities WITHOUT taking g.mu. Callers must run on the
// game-loop goroutine between Tick calls (as the arena does) — never
// concurrently with Tick.
func BuildObservation(g *engine.Game, t *engine.Tank, buf []float64) {
	buf = buf[:ObsSize]
	for i := range buf {
		buf[i] = 0
	}
	cfg := &g.Config
	pos := t.GetPosition()

	// Own state.
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

	// K nearest entities (excluding self) within ViewRange, ascending distance.
	type cand struct {
		e  engine.Entity
		d2 float64
	}
	cands := make([]cand, 0, len(g.Entities))
	vr2 := vr * vr
	for _, e := range g.Entities {
		if e == engine.Entity(t) {
			continue
		}
		d2 := pos.DistanceSquaredTo(e.GetPosition())
		if d2 <= vr2 {
			cands = append(cands, cand{e, d2})
		}
	}
	// Stable so equal distances keep entity-slice order → deterministic.
	sort.SliceStable(cands, func(i, j int) bool { return cands[i].d2 < cands[j].d2 })

	for k := 0; k < numSlots && k < len(cands); k++ {
		fillSlot(buf[slotsBase+k*slotLen:slotsBase+(k+1)*slotLen], cands[k].e, t, cfg)
	}
}

// fillSlot writes one 9-float entity slot (contracts §1). slot has len 9.
func fillSlot(slot []float64, e engine.Entity, t *engine.Tank, cfg *engine.GameConfig) {
	rel := e.GetPosition().Sub(t.GetPosition())
	slot[0] = rel.X / t.ViewRange
	slot[1] = rel.Y / t.ViewRange
	if cfg.BulletMuzzleSpeed > 0 {
		relVel := e.GetVelocity().Sub(t.Vel)
		slot[2] = relVel.X / cfg.BulletMuzzleSpeed
		slot[3] = relVel.Y / cfg.BulletMuzzleSpeed
	}
	switch e.(type) {
	case *engine.Tank:
		slot[4] = 1
	case *engine.Bullet:
		slot[5] = 1
	case *engine.Food:
		slot[6] = 1
	}
	if cfg.TankRadius > 0 {
		slot[7] = engine.BoundingRadius(e.GetGeom()) / cfg.TankRadius
	}
	if mh := e.GetMaxHealth(); mh > 0 {
		slot[8] = e.GetHealth() / mh
	}
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
