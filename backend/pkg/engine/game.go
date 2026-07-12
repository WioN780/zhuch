package engine

import (
	"math/rand"
	"strconv"
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
	ObstacleCount  int     `json:"obstacle_count"`

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
		MapWidth:       4000,
		MapHeight:      4000,
		CellSize:       100,
		MaxFood:        120,
		ObstacleCount:  12,

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

		BulletMuzzleSpeed: 20.0,
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
	TickDuration time.Duration `json:"tick_duration_ns"`
	EntityCount  int           `json:"entity_count"`
}

// Game represents a single game. Not thread-safe — must be accessed from one goroutine only.
type Game struct {
	Config      GameConfig
	Arena       *Arena
	Entities    []Entity
	Rng         *rand.Rand
	mu          sync.Mutex
	IsActive    bool
	CurrentTick int
	Metrics     PerformanceMetrics
	nextID      int

	// OnEvent, when set, is called for engine events ("kill", "death") while
	// g.mu is held: handlers must be fast, non-blocking, and must not call
	// back into Game. Used by the telemetry producers.
	OnEvent func(eventType, actor, target string, pos Vector2)
}

// NewGame acts as the factory for the room
func NewGame(config GameConfig) *Game {
	return NewGameSeeded(config, time.Now().UnixNano())
}

// NewGameSeeded creates a game whose randomness (food spawns, etc.) is
// fully determined by seed, so seeded games replay identically.
func NewGameSeeded(config GameConfig, seed int64) *Game {
	g := &Game{
		Config:   config,
		Arena:    NewArena(config.MapWidth, config.MapHeight),
		Entities: make([]Entity, 0),
		Rng:      rand.New(rand.NewSource(seed)),
		IsActive: false,
	}
	g.generateObstacles()
	return g
}

// generateObstacles seeds Arena.Obstacles deterministically from g.Rng:
// circle radius 60-140, positioned with a 250-unit margin from the walls,
// reject-and-retry (max 20 tries) on overlap with an already-placed
// obstacle so they don't fuse into each other.
func (g *Game) generateObstacles() {
	const margin, minR, maxR = 250.0, 60.0, 140.0
	for i := 0; i < g.Config.ObstacleCount; i++ {
		for try := 0; try < 20; try++ {
			r := minR + g.Rng.Float64()*(maxR-minR)
			c := &Circle{
				Center: Vector2{
					X: margin + g.Rng.Float64()*(g.Config.MapWidth-2*margin),
					Y: margin + g.Rng.Float64()*(g.Config.MapHeight-2*margin),
				},
				Radius: r,
			}
			if !obstacleOverlaps(g.Arena.Obstacles, c) {
				g.Arena.Obstacles = append(g.Arena.Obstacles, c)
				break
			}
		}
	}
}

func obstacleOverlaps(existing []GeomObject, c *Circle) bool {
	for _, obs := range existing {
		if oc, ok := obs.(*Circle); ok {
			rr := oc.Radius + c.Radius
			if oc.Center.DistanceSquaredTo(c.Center) < rr*rr {
				return true
			}
		}
	}
	return false
}

// insideObstacle reports whether pos lands inside any obstacle, padded by
// slack, so spawn logic can avoid embedding entities in one.
func (g *Game) insideObstacle(pos Vector2, slack float64) bool {
	for _, obs := range g.Arena.Obstacles {
		if c, ok := obs.(*Circle); ok {
			r := c.Radius + slack
			if pos.DistanceSquaredTo(c.Center) <= r*r {
				return true
			}
		}
	}
	return false
}

// SafeSpawnPos returns a random spawn position with a 100-unit wall margin,
// retrying (max 10 tries) any position that lands inside an obstacle (40-unit
// slack) so tanks don't spawn embedded in one. Used by both the live hub
// (Hub.Register) and the arena's default/random spawn.
func (g *Game) SafeSpawnPos() Vector2 {
	g.mu.Lock()
	defer g.mu.Unlock()
	const margin = 100.0
	pos := Vector2{
		X: margin + g.Rng.Float64()*(g.Config.MapWidth-2*margin),
		Y: margin + g.Rng.Float64()*(g.Config.MapHeight-2*margin),
	}
	for try := 0; try < 10 && g.insideObstacle(pos, 40.0); try++ {
		pos = Vector2{
			X: margin + g.Rng.Float64()*(g.Config.MapWidth-2*margin),
			Y: margin + g.Rng.Float64()*(g.Config.MapHeight-2*margin),
		}
	}
	return pos
}

// newID returns a deterministic per-game entity ID. Caller must hold g.mu.
// Collision pair-dedup and kill attribution compare IDs, so seeded replays
// need IDs that don't depend on uuid randomness.
func (g *Game) newID(prefix string) string {
	g.nextID++
	return prefix + "-" + strconv.Itoa(g.nextID)
}

// SpawnTank assigns a deterministic ID and appends the tank under g.mu.
func (g *Game) SpawnTank(t *Tank) {
	g.mu.Lock()
	defer g.mu.Unlock()
	t.ID = g.newID("tank")
	g.Entities = append(g.Entities, t)
}

// SpawnBullet assigns a deterministic ID and appends the bullet under g.mu.
func (g *Game) SpawnBullet(b *Bullet) {
	g.mu.Lock()
	defer g.mu.Unlock()
	b.ID = g.newID("bullet")
	g.Entities = append(g.Entities, b)
}

// Tick executes exactly one frame of game logic
func (g *Game) Tick() {
	start := time.Now()

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

// Tanks returns a snapshot of the currently alive tanks.
func (g *Game) Tanks() []*Tank {
	g.mu.Lock()
	defer g.mu.Unlock()
	tanks := make([]*Tank, 0, 8)
	for _, e := range g.Entities {
		if t, ok := e.(*Tank); ok {
			tanks = append(tanks, t)
		}
	}
	return tanks
}

// HasEntity reports whether an entity with the given ID is currently alive.
func (g *Game) HasEntity(id string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	for _, e := range g.Entities {
		if e.GetID() == id {
			return true
		}
	}
	return false
}

func (g *Game) GetVisibleEntities(pos Vector2, viewRange float64) []Entity {
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
	pos := Vector2{X: g.Rng.Float64() * g.Config.MapWidth, Y: g.Rng.Float64() * g.Config.MapHeight}
	for try := 0; try < 10 && g.insideObstacle(pos, 40.0); try++ {
		pos = Vector2{X: g.Rng.Float64() * g.Config.MapWidth, Y: g.Rng.Float64() * g.Config.MapHeight}
	}
	x, y := pos.X, pos.Y

	roll := g.Rng.Float64()
	var fType FoodType
	if roll < 0.6 {
		fType = FoodSquare
	} else if roll < 0.9 {
		fType = FoodTriangle
	} else {
		fType = FoodPentagon
	}

	food := NewFood(g.Config.FoodConfigs[fType], fType, Vector2{X: x, Y: y})
	food.ID = g.newID("food") // spawnRandomFood runs under g.mu (inside Tick)
	g.Entities = append(g.Entities, food)
}

func (g *Game) processDeath(victim Entity) {
	attackerID := victim.GetLastAttackerID()

	if g.OnEvent != nil {
		g.OnEvent("death", victim.GetID(), attackerID, victim.GetPosition())
	}

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

	if g.OnEvent != nil {
		g.OnEvent("kill", killer.GetID(), victim.GetID(), victim.GetPosition())
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
	g.Entities = make([]Entity, 0)
	g.CurrentTick = 0
}
