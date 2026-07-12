package bots

import (
	"zhuch/pkg/brain"
	"zhuch/pkg/engine"
)

// Policy maps an observation vector to a raw action vector.
type Policy interface {
	Act(obs []float64) []float64
}

// MLPPolicy adapts *brain.MLP to Policy.
type MLPPolicy struct{ M *brain.MLP }

func (p MLPPolicy) Act(obs []float64) []float64 { return p.M.Forward(obs) }

// Bot ties a tank to a policy with action-repeat (contracts §2).
type Bot struct {
	Tank        *engine.Tank
	Brain       Policy
	DecideEvery int // decide every N ticks (contract: 4)
	Offset      int // stagger: decides when (tick+Offset)%DecideEvery == 0

	obs [ObsSize]float64
}

// Step decides on decision ticks; between decisions the tank's last
// InputVector/Orientation persist and no fire is attempted.
func (b *Bot) Step(g *engine.Game, tick int) {
	if !b.Tank.IsAlive() {
		return
	}
	every := b.DecideEvery
	if every <= 0 {
		every = 1
	}
	if (tick+b.Offset)%every != 0 {
		return
	}
	BuildObservation(g, b.Tank, b.obs[:])
	Apply(b.Brain.Act(b.obs[:]), b.Tank, g, tick)
}
