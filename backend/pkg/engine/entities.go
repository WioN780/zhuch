package engine

import (
	"math"

	"github.com/google/uuid"
)

type Entity interface {
	GetID() string
	GetObject() GeomObject

	// For Health and Lifespans
	IsAlive() bool
	GetHealth() float64
	SetHealth(h float64)

	GetPosition() Vector2
	SetPosition(pos Vector2)

	GetVelocity() Vector2
	SetVelocity(vel Vector2)

	GetBodyDamage() float64
	GetWeight() float64

	ResetActionTimer()
	ApplyForce(force Vector2)

	OnCollision(other Entity)

	// Kill attribution
	GetLastAttackerID() string
	SetLastAttackerID(id string)

	// Optionally applies friction of the surface
	TickCalculation(friction float64)
}

// ---------------------------------------------------------
// REUSABLE BASE
// ---------------------------------------------------------

type BaseEntity struct {
	ID               string
	Vel              Vector2
	Object           GeomObject
	Health           float64
	MaxHealth        float64
	BodyDamage       float64
	Weight           float64
	TicksSinceAction int
	LastAttackerID   string
}

func (b *BaseEntity) GetID() string           { return b.ID }
func (b *BaseEntity) GetObject() GeomObject   { return b.Object }
func (b *BaseEntity) IsAlive() bool           { return b.Health > 0 }
func (b *BaseEntity) GetPosition() Vector2    { return b.Object.GetCenter() }
func (b *BaseEntity) SetPosition(pos Vector2) { b.Object.SetCenter(pos) }
func (b *BaseEntity) GetVelocity() Vector2    { return b.Vel }
func (b *BaseEntity) SetVelocity(vel Vector2) { b.Vel = vel }
func (b *BaseEntity) GetBodyDamage() float64  { return b.BodyDamage }
func (b *BaseEntity) GetWeight() float64      { return b.Weight }
func (b *BaseEntity) SetWeight(w float64)     { b.Weight = w }
func (b *BaseEntity) ResetActionTimer()       { b.TicksSinceAction = 0 }

func (b *BaseEntity) GetLastAttackerID() string   { return b.LastAttackerID }
func (b *BaseEntity) SetLastAttackerID(id string) { b.LastAttackerID = id }

func (b *BaseEntity) ApplyForce(force Vector2) {
	b.Vel = b.Vel.Add(force)
}

func (b *BaseEntity) OnCollision(other Entity) {
	// Prevent an object from damaging itself
	if b.ID == other.GetID() {
		return
	}

	// Deal damage
	b.Health -= other.GetBodyDamage()
	b.ResetActionTimer()

	other.SetLastAttackerID(b.ID)
}

func (b *BaseEntity) GetHealth() float64  { return b.Health }
func (b *BaseEntity) SetHealth(h float64) { b.Health = h }

// ---------------------------------------------------------
// GAME OBJECTS
// ---------------------------------------------------------

// -------- TANK --------

type Tank struct {
	BaseEntity
	Config           *GameConfig
	InputVector      Vector2
	Orientation      float64
	MaxSpeed         float64
	MoveAcceleration float64

	// Stats
	Score float64
	Kills int

	// Regeneration configuration (Values per tick)
	RegenRate      float64 // Constant slow heal
	QuickRegenRate float64 // Fast heal after cooldown
	RegenCooldown  int     // Ticks to wait before QuickRegen works

	// Shooting configuration
	FireCooldown int // in ticks
	LastFireTick int
	ViewRange    float64
}

var _ Entity = (*Tank)(nil)

// Default constructor
func NewTank(startV Vector2, config *GameConfig) *Tank {
	return &Tank{
		BaseEntity: BaseEntity{
			ID:               uuid.New().String(),
			Vel:              Vector2{X: 0, Y: 0},
			Object:           &Circle{Center: startV, Radius: config.TankRadius},
			Health:           config.TankMaxHealth,
			MaxHealth:        config.TankMaxHealth,
			BodyDamage:       config.TankBodyDamage,
			Weight:           config.TankWeight,
			TicksSinceAction: 0,
		},
		Config:           config,
		InputVector:      Vector2{0.0, 0.0},
		MaxSpeed:         config.TankMaxSpeed,
		MoveAcceleration: config.TankAcceleration,
		Score:            0,
		Kills:            0,
		RegenRate:        config.TankRegenRate,
		QuickRegenRate:   config.TankQuickRegenRate,
		RegenCooldown:    config.TankRegenCooldown,
		FireCooldown:     config.TankFireCooldown,
		LastFireTick:     0,
		ViewRange:        config.ViewRange,
	}
}

func (t *Tank) Fire(target Vector2, currentTick int) *Bullet {
	if currentTick-t.LastFireTick < t.FireCooldown {
		return nil
	}

	t.LastFireTick = currentTick
	pos := t.GetPosition()
	radius := t.Config.TankRadius
	if circ, ok := t.Object.(*Circle); ok {
		radius = circ.Radius
	}

	// Calculate direction
	dx := target.X - pos.X
	dy := target.Y - pos.Y
	dist := math.Sqrt(dx*dx + dy*dy)
	if dist == 0 {
		return nil
	}

	dirX, dirY := dx/dist, dy/dist

	// Proportional scaling based on current Tank size relative to base
	scaleFactor := radius / t.Config.TankRadius
	bulletRadius := t.Config.BulletRadius * scaleFactor
	bulletWeight := t.Config.BulletWeight * scaleFactor
	bulletDamage := t.Config.BulletDamage * scaleFactor

	// Bullet Velocity = Muzzle Speed + Tank Velocity (Inertia)
	bulletVelX := (dirX * t.Config.BulletMuzzleSpeed) + t.Vel.X
	bulletVelY := (dirY * t.Config.BulletMuzzleSpeed) + t.Vel.Y

	// Recoil Force (Opposite of shot)
	recoilForce := Vector2{X: -dirX, Y: -dirY}.Scale(t.Config.RecoilPower * bulletWeight)
	t.ApplyForce(recoilForce)

	// Spawn exactly on the surface of the tank
	spawnPos := Vector2{X: pos.X + dirX*radius, Y: pos.Y + dirY*radius}

	return NewBullet(t.ID, spawnPos, bulletVelX, bulletVelY, bulletWeight, bulletRadius, bulletDamage, t.Config.BulletLifespan)
}

func (t *Tank) TickCalculation(friction float64) {
	// Every tank has it's own acceleration
	t.ApplyForce(t.InputVector.Scale(t.MoveAcceleration))

	// Move position
	currentPos := t.GetPosition()
	newPos := currentPos.Add(t.Vel)
	t.SetPosition(newPos)

	// Apply friction
	t.Vel = t.Vel.Scale(friction)

	// Cap speed
	speed := math.Sqrt(t.Vel.X*t.Vel.X + t.Vel.Y*t.Vel.Y)
	if speed > t.MaxSpeed && t.MaxSpeed > 0 {
		t.Vel.X = (t.Vel.X / speed) * t.MaxSpeed
		t.Vel.Y = (t.Vel.Y / speed) * t.MaxSpeed
	}

	// Only heal if damaged
	if t.Health < t.MaxHealth {
		t.Health += t.RegenRate

		// QuickRegen
		if t.TicksSinceAction >= t.RegenCooldown {
			t.Health += t.QuickRegenRate
		}

		// Cap Health
		if t.Health > t.MaxHealth {
			t.Health = t.MaxHealth
		}
	}

	t.TicksSinceAction++
}

// -------- BULLET --------

type Bullet struct {
	BaseEntity
	LifespanTicks int
	OwnerID       string
}

var _ Entity = (*Bullet)(nil)

// Default constructor
func NewBullet(ownerID string, startV Vector2, dirX, dirY, weight, radius, damage float64, lifespan int) *Bullet {
	return &Bullet{
		BaseEntity: BaseEntity{
			ID:         uuid.New().String(),
			Vel:        Vector2{X: dirX, Y: dirY},
			Object:     &Circle{Center: startV, Radius: radius},
			Health:     1.0,
			MaxHealth:  1.0,
			BodyDamage: damage,
			Weight:     weight,
		},
		LifespanTicks: lifespan,
		OwnerID:       ownerID,
	}
}

func (b *Bullet) OnCollision(other Entity) {
	if b.OwnerID == other.GetID() {
		return
	}

	b.BaseEntity.OnCollision(other)
	// Bullets are destroyed on impact
	b.Health = 0
}

func (b *Bullet) TickCalculation(friction float64) {
	// Bullets travel at constant speed
	currentPos := b.GetPosition()
	newPos := currentPos.Add(b.Vel)
	b.SetPosition(newPos)

	// Burn lifetime
	b.LifespanTicks--

	if b.LifespanTicks <= 0 {
		b.Health = 0
	}
}

// -------- FOOD --------

type FoodType string

const (
	FoodSquare   FoodType = "square"
	FoodTriangle FoodType = "triangle"
	FoodPentagon FoodType = "pentagon"
)

type Food struct {
	BaseEntity
	Type       FoodType
	ScoreValue float64
}

var _ Entity = (*Food)(nil)

func NewFood(config FoodConfig, fType FoodType, pos Vector2) *Food {
	var obj GeomObject

	switch fType {
	case FoodSquare:
		obj = &Square{Center: pos, SideLength: config.Size}
	case FoodTriangle:
		obj = &Triangle{Center: pos, Size: config.Size}
	case FoodPentagon:
		obj = &Pentagon{Center: pos, Size: config.Size}
	}

	return &Food{
		BaseEntity: BaseEntity{
			ID:         uuid.New().String(),
			Object:     obj,
			Health:     config.Health,
			MaxHealth:  config.Health,
			BodyDamage: config.BodyDamage,
			Weight:     config.Weight,
		},
		Type:       fType,
		ScoreValue: config.ScoreValue,
	}
}

func (f *Food) TickCalculation(friction float64) {
	// Food just drifts slightly or stays still
	currentPos := f.GetPosition()
	newPos := currentPos.Add(f.Vel)
	f.SetPosition(newPos)
	f.Vel = f.Vel.Scale(friction)
}
