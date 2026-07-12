package bots

import (
	"math"
	"testing"

	"zhuch/pkg/engine"
)

func TestBuildObservation(t *testing.T) {
	cfg := engine.DefaultConfig() // ViewRange 800, TankMaxSpeed 15, MuzzleSpeed 20, TankRadius 20
	cfg.ObstacleCount = 0         // controlled scenario: place obstacles explicitly, not randomly
	g := engine.NewGameSeeded(cfg, 1)

	tank := engine.NewTank("me", engine.Vector2{X: 150, Y: 1000}, &g.Config)
	g.SpawnTank(tank)
	tank.Vel = engine.Vector2{X: 3, Y: -4.5}
	tank.Orientation = 0.5

	food := engine.NewFood(cfg.FoodConfigs[engine.FoodSquare], engine.FoodSquare, engine.Vector2{X: 250, Y: 1000})
	g.Entities = append(g.Entities, food)

	// One static obstacle, farther than the food so slot order is unambiguous.
	g.Arena.Obstacles = append(g.Arena.Obstacles, &engine.Circle{Center: engine.Vector2{X: 150, Y: 1300}, Radius: 80})

	var buf [ObsSize]float64
	BuildObservation(g, tank, buf[:])

	want := map[int]float64{
		0: 1.0,         // full health
		1: 3.0 / 15.0,  // vel.X / TankMaxSpeed
		2: -4.5 / 15.0, // vel.Y / TankMaxSpeed
		3: math.Sin(0.5),
		4: math.Cos(0.5),
		5: 1.0,           // tick 0, LastFireTick 0 → full cooldown remaining
		6: 150.0 / 800.0, // left wall
		7: 1.0,           // right wall clipped
		8: 1.0,           // top wall clipped
		9: 1.0,           // bottom wall clipped
		// slot 0: the food, rel (100, 0), dist 100
		10: 100.0 / 800.0,
		11: 0,
		12: -3.0 / 20.0,         // rel vel (0-3)/muzzle
		13: 4.5 / 20.0,          // (0-(-4.5))/muzzle
		14: 0,                   // is_tank
		15: 0,                   // is_bullet
		16: 1,                   // is_food
		17: 0,                   // is_obstacle
		18: 15.0 * 0.707 / 20.0, // square bounding radius / TankRadius
		19: 1.0,                 // food full health
		// slot 1: the obstacle, rel (0, 300), dist 300
		20: 0,
		21: 300.0 / 800.0,
		22: -3.0 / 20.0, // rel vel (0-3)/muzzle (obstacle vel is 0)
		23: 4.5 / 20.0,  // (0-(-4.5))/muzzle
		24: 0,           // is_tank
		25: 0,           // is_bullet
		26: 0,           // is_food
		27: 1,           // is_obstacle
		28: 80.0 / 20.0, // obstacle radius / TankRadius
		29: 1.0,         // obstacles report full "health"
	}
	for i := 0; i < ObsSize; i++ {
		exp := want[i] // absent keys (slots 2-7) must be zero-padded
		if math.Abs(buf[i]-exp) > 1e-12 {
			t.Errorf("obs[%d] = %v, want %v", i, buf[i], exp)
		}
	}
}
