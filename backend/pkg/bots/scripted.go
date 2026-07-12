package bots

import "math"

// Scripted is the frozen chaser/shooter baseline (contracts §9). It reads
// only the observation vector, so it exercises the exact same pipeline as
// learned policies. Not safe for concurrent use; one instance per bot.
type Scripted struct {
	n int // decision counter, drives strafe sign flips
}

func (s *Scripted) Act(obs []float64) []float64 {
	out := make([]float64, 5)
	out[4] = -1 // default: don't fire

	// Nearest tank slot, else nearest food slot (slots are distance-sorted).
	target, isTank := -1, false
	for k := 0; k < numSlots; k++ {
		o := slotsBase + k*slotLen
		if obs[o+4] == 1 {
			target, isTank = o, true
			break
		}
	}
	if target < 0 {
		for k := 0; k < numSlots; k++ {
			o := slotsBase + k*slotLen
			if obs[o+6] == 1 {
				target = o
				break
			}
		}
	}
	s.n++
	if target < 0 {
		return out // nothing visible: coast
	}

	dx, dy := obs[target], obs[target+1] // rel pos, normalized by ViewRange
	dist := math.Hypot(dx, dy)
	if dist == 0 {
		return out
	}
	ux, uy := dx/dist, dy/dist

	// Aim at target (atan2(out[3], out[2]) in Apply).
	out[2], out[3] = ux, uy

	// Move toward it; strafe perpendicular when chasing a tank.
	mx, my := ux, uy
	if isTank {
		sign := 1.0
		// ponytail: flips every 10 decisions = 40 ticks at the pinned DecideEvery=4
		if s.n%20 >= 10 {
			sign = -1.0
		}
		mx, my = ux-sign*0.6*uy, uy+sign*0.6*ux
		l := math.Hypot(mx, my)
		mx, my = mx/l, my/l
	}
	// Pre-tanh outputs: atanh(0.99·unit) survives Apply's tanh almost exactly.
	out[0], out[1] = math.Atanh(0.99*mx), math.Atanh(0.99*my)

	// Fire when cooldown ready and tank target within 0.9·ViewRange.
	if isTank && obs[5] == 0 && dist < 0.9 {
		out[4] = 1
	}
	return out
}
