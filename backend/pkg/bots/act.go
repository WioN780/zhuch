package bots

import (
	"math"

	"zhuch/pkg/engine"
)

// Apply maps a raw 5-float network output onto the tank (contracts §2):
// InputVector = tanh(out[0..1]) clamped to unit length,
// Orientation = atan2(out[3], out[2]),
// fire iff out[4] > 0 (engine cooldown respected via Tank.Fire).
func Apply(out []float64, t *engine.Tank, g *engine.Game, tick int) {
	vx, vy := math.Tanh(out[0]), math.Tanh(out[1])
	if l := math.Hypot(vx, vy); l > 1 {
		vx, vy = vx/l, vy/l
	}
	t.InputVector = engine.Vector2{X: vx, Y: vy}
	t.Orientation = math.Atan2(out[3], out[2])
	if out[4] > 0 {
		if b := t.Fire(t.Orientation, tick); b != nil {
			g.SpawnBullet(b)
		}
	}
}
