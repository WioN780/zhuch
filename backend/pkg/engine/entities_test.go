package engine_test

import (
	"testing"

	"zhuch/pkg/engine"
)

// TestBulletHitsTankAttribution exercises the accuracy-stat wiring:
// Tank.Fire increments ShotsFired and stamps Bullet.Owner, and a bullet
// damaging another tank increments the owner's HitsTank (contracts
// §accuracy stats). Tanks are stationary (no bot AI) so this only exercises
// the collision/attribution path, not chase behavior.
func TestBulletHitsTankAttribution(t *testing.T) {
	cfg := engine.DefaultConfig()
	cfg.ObstacleCount = 0 // deterministic, obstacle-free scenario
	g := engine.NewGameSeeded(cfg, 1)

	shooter := engine.NewTank("shooter", engine.Vector2{X: 100, Y: 100}, &g.Config)
	g.SpawnTank(shooter)
	target := engine.NewTank("target", engine.Vector2{X: 300, Y: 100}, &g.Config)
	g.SpawnTank(target)

	b := shooter.Fire(0, shooter.FireCooldown) // orientation 0 -> fires toward +X, straight at target
	if b == nil {
		t.Fatal("Fire returned nil, expected a bullet")
	}
	if shooter.ShotsFired != 1 {
		t.Fatalf("ShotsFired = %d, want 1", shooter.ShotsFired)
	}
	if b.Owner != shooter {
		t.Fatal("bullet Owner not set to the firing tank")
	}
	g.SpawnBullet(b)

	for i := 0; i < 15 && shooter.HitsTank == 0; i++ {
		g.Tick()
	}
	if shooter.HitsTank != 1 {
		t.Fatalf("HitsTank = %d, want 1", shooter.HitsTank)
	}
}

// TestBulletHitsFoodAttribution: same wiring, but the bullet hits food.
func TestBulletHitsFoodAttribution(t *testing.T) {
	cfg := engine.DefaultConfig()
	cfg.ObstacleCount = 0
	g := engine.NewGameSeeded(cfg, 1)

	shooter := engine.NewTank("shooter", engine.Vector2{X: 100, Y: 100}, &g.Config)
	g.SpawnTank(shooter)

	food := engine.NewFood(cfg.FoodConfigs[engine.FoodSquare], engine.FoodSquare, engine.Vector2{X: 300, Y: 100})
	food.ID = "food-test"
	g.Entities = append(g.Entities, food)

	b := shooter.Fire(0, shooter.FireCooldown)
	if b == nil {
		t.Fatal("Fire returned nil, expected a bullet")
	}
	g.SpawnBullet(b)

	for i := 0; i < 15 && shooter.HitsFood == 0; i++ {
		g.Tick()
	}
	if shooter.HitsFood != 1 {
		t.Fatalf("HitsFood = %d, want 1", shooter.HitsFood)
	}
}

// TestOwnBulletNoSelfHit: a tank overlapping its own bullet (possible after a
// ricochet) must take no damage and must not credit itself a HitsTank.
func TestOwnBulletNoSelfHit(t *testing.T) {
	cfg := engine.DefaultConfig()
	cfg.ObstacleCount = 0
	cfg.MaxFood = 0
	g := engine.NewGameSeeded(cfg, 1)

	shooter := engine.NewTank("shooter", engine.Vector2{X: 500, Y: 500}, &g.Config)
	g.SpawnTank(shooter)

	b := shooter.Fire(0, shooter.FireCooldown)
	if b == nil {
		t.Fatal("Fire returned nil, expected a bullet")
	}
	g.SpawnBullet(b)
	// Force the overlap: park the bullet on the tank with no velocity.
	b.SetPosition(shooter.GetPosition())
	b.SetVelocity(engine.Vector2{})
	shooter.Vel = engine.Vector2{}

	g.Tick()

	if shooter.HitsTank != 0 {
		t.Fatalf("HitsTank = %d, want 0 (own bullet must not count)", shooter.HitsTank)
	}
	if shooter.Health < shooter.MaxHealth {
		t.Fatalf("tank damaged by its own bullet: health %.1f", shooter.Health)
	}
	if !b.IsAlive() {
		t.Fatal("own bullet died on owner contact; owner guard should keep it alive")
	}
}

// TestBulletDiesOnObstacle: obstacles are not Entities, so without the
// Bullet.OnCollision override a bullet ricochets off them forever; an
// obstacle hit is a miss and must end the bullet.
func TestBulletDiesOnObstacle(t *testing.T) {
	cfg := engine.DefaultConfig()
	cfg.ObstacleCount = 0
	cfg.MaxFood = 0
	g := engine.NewGameSeeded(cfg, 1)
	g.Arena.Obstacles = append(g.Arena.Obstacles,
		&engine.Circle{Center: engine.Vector2{X: 300, Y: 100}, Radius: 60})

	shooter := engine.NewTank("shooter", engine.Vector2{X: 100, Y: 100}, &g.Config)
	g.SpawnTank(shooter)

	b := shooter.Fire(0, shooter.FireCooldown) // toward +X, straight at the rock
	if b == nil {
		t.Fatal("Fire returned nil, expected a bullet")
	}
	g.SpawnBullet(b)

	for i := 0; i < 15 && b.IsAlive(); i++ {
		g.Tick()
	}
	if b.IsAlive() {
		t.Fatal("bullet still alive after hitting an obstacle; expected it to die (miss)")
	}
	if shooter.HitsTank != 0 || shooter.HitsFood != 0 {
		t.Fatalf("obstacle hit counted as a hit: HitsTank=%d HitsFood=%d", shooter.HitsTank, shooter.HitsFood)
	}
}
