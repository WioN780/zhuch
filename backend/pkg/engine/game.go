package engine

import (
	"math/rand"
	"sync"
	"time"
)

// GameConfig holds all the global variables
type GameConfig struct {
	TicksPerSecond int     `json:"ticks_per_second"`
	Friction       float64 `json:"friction"`
	MapWidth       float64 `json:"map_width"`
	MapHeight      float64 `json:"map_height"`
	CellSize       float64 `json:"cell_size"`
	MaxFood        int     `json:"max_food"`

	// Tank Defaults
	TankRadius       float64 `json:"tank_radius"`
	TankMaxHealth    float64 `json:"tank_max_health"`
	TankBodyDamage   float64 `json:"tank_body_damage"`
	TankWeight       float64 `json:"tank_weight"`
	TankMaxSpeed     float64 `json:"tank_max_speed"`
	TankAcceleration float64 `json:"tank_acceleration"`
	ViewRange        float64 `json:"view_range"`

	TankRegenRate      float64 `json:"tank_regen_rate"`
	TankQuickRegenRate float64 `json:"tank_quick_regen_rate"`
	TankRegenCooldown  int     `json:"tank_regen_cooldown"`
	TankFireCooldown   int     `json:"tank_fire_cooldown"`

	// Bullet Defaults
	BulletMuzzleSpeed float64 `json:"bullet_muzzle_speed"`
	BulletWeight      float64 `json:"bullet_weight"`
	BulletRadius      float64 `json:"bullet_radius"`
	BulletDamage      float64 `json:"bullet_damage"`
	BulletLifespan    int     `json:"bullet_lifespan"`
	RecoilPower       float64 `json:"recoil_power"`

	// Food Configs
	FoodConfigs map[FoodType]FoodConfig `json:"food_configs"`
}

type FoodConfig struct {
	Health     float64 `json:"health"`
	ScoreValue float64 `json:"score_value"`
	BodyDamage float64 `json:"body_damage"`
	Weight     float64 `json:"weight"`
	Size       float64 `json:"size"`
}

func DefaultConfig() GameConfig {
	return GameConfig{
		TicksPerSecond: 20,
		Friction:       0.9,
		MapWidth:       2000,
		MapHeight:      2000,
		CellSize:       100,
		MaxFood:        50,

		TankRadius:       20.0,
		TankMaxHealth:    100.0,
		TankBodyDamage:   20.0,
		TankWeight:       10.0,
		TankMaxSpeed:     15.0,
		TankAcceleration: 1.5,
		ViewRange:        800.0,

		TankRegenRate:      0.02,
		TankQuickRegenRate: 0.8,
		TankRegenCooldown:  100,
		TankFireCooldown:   10,

		BulletMuzzleSpeed: 15.0,
		BulletWeight:      1.0,
		BulletRadius:      5.0,
		BulletDamage:      15.0,
		BulletLifespan:    60,
		RecoilPower:       2.0,

		FoodConfigs: map[FoodType]FoodConfig{
			FoodSquare:   {Health: 10, ScoreValue: 10, BodyDamage: 5, Weight: 0.5, Size: 15},
			FoodTriangle: {Health: 30, ScoreValue: 25, BodyDamage: 10, Weight: 0.8, Size: 12},
			FoodPentagon: {Health: 100, ScoreValue: 100, BodyDamage: 20, Weight: 2.0, Size: 20},
		},
	}
}

type PerformanceMetrics struct {
	TickDuration time.Duration
	EntityCount  int
}

// Game represents a single game
type Game struct {
	Config      GameConfig
	Arena       *Arena
	Entities    []Entity
	mu          sync.Mutex
	IsActive    bool
	CurrentTick int
	Metrics     PerformanceMetrics
}

// NewGame acts as the factory for the room
func NewGame(config GameConfig) *Game {
	return &Game{
		Config:   config,
		Arena:    NewArena(config.MapWidth, config.MapHeight),
		Entities: make([]Entity, 0),
		IsActive: false,
	}
}

// Tick executes exactly one frame of game logic
func (g *Game) Tick() {
	start := time.Now()

	g.mu.Lock()
	defer g.mu.Unlock()

	g.CurrentTick++

	survivors := g.Entities[:0]
	foodCount := 0

	for _, entity := range g.Entities {
		entity.TickCalculation(g.Config.Friction)

		pos := entity.GetPosition()
		if pos.X < 0 || pos.X > g.Config.MapWidth || pos.Y < 0 || pos.Y > g.Config.MapHeight {
			entity.SetHealth(0)
		}

		if entity.IsAlive() {
			survivors = append(survivors, entity)
			if _, ok := entity.(*Food); ok {
				foodCount++
			}
		} else {
			// Entity just died, process rewards
			g.processDeath(entity)
		}
	}

	// clean the garbage
	for i := len(survivors); i < len(g.Entities); i++ {
		g.Entities[i] = nil
	}

	g.Entities = survivors

	// Spawn food if needed
	for foodCount < g.Config.MaxFood {
		g.spawnRandomFood()
		foodCount++
	}

	g.CheckAllCollisions(g.Arena)

	g.Metrics.TickDuration = time.Since(start)
	g.Metrics.EntityCount = len(g.Entities)
}

func (g *Game) GetVisibleEntities(pos Vector2, viewRange float64) []Entity {
	g.mu.Lock()
	defer g.mu.Unlock()

	visible := make([]Entity, 0)
	rangeSq := viewRange * viewRange

	for _, entity := range g.Entities {
		if pos.DistanceSquaredTo(entity.GetPosition()) <= rangeSq {
			visible = append(visible, entity)
		}
	}
	return visible
}

func (g *Game) spawnRandomFood() {
	x := rand.Float64() * g.Config.MapWidth
	y := rand.Float64() * g.Config.MapHeight

	roll := rand.Float64()
	var fType FoodType
	if roll < 0.6 {
		fType = FoodSquare
	} else if roll < 0.9 {
		fType = FoodTriangle
	} else {
		fType = FoodPentagon
	}

	food := NewFood(g.Config.FoodConfigs[fType], fType, Vector2{X: x, Y: y})
	g.Entities = append(g.Entities, food)
}

func (g *Game) processDeath(victim Entity) {
	attackerID := victim.GetLastAttackerID()
	if attackerID == "" {
		return
	}

	// Find the killer
	var killer *Tank
	for _, e := range g.Entities {
		if e.GetID() == attackerID {
			if t, ok := e.(*Tank); ok {
				killer = t
			}
			break
		}
	}

	if killer == nil {
		return
	}

	// Award based on victim type
	switch v := victim.(type) {
	case *Tank:
		killer.Kills++
		killer.Score += v.Score / 2 // Large bonus for killing a player
	case *Food:
		killer.Score += v.ScoreValue
	}
}

// Reset clears the game state
func (g *Game) Reset() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.Entities = make([]Entity, 0)
	g.CurrentTick = 0
}
