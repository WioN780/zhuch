package engine

import (
	"math"

	"github.com/google/uuid"
)

type Entity interface {
	Collidable
	// Game Logic
	IsAlive() bool
	GetHealth() float64
	SetHealth(h float64)
	GetBodyDamage() float64

	ResetActionTimer()

	// Kill attribution
	GetLastAttackerID() string
	SetLastAttackerID(id string)

	TickCalculation(friction float64)

	CollisionAction(other Entity)
}

// ---------------------------------------------------------
// REUSABLE BASE
// ---------------------------------------------------------

type BaseEntity struct {
	MovingCollidable
	Health           float64
	MaxHealth        float64
	BodyDamage       float64
	TicksSinceAction int
	LastAttackerID   string
}

func (b *BaseEntity) OnCollision(other Collidable, normal Vector2, overlap float64) {
	b.MovingCollidable.OnCollision(other, normal, overlap)
	if ent, ok := other.(Entity); ok {
		b.CollisionAction(ent)
	}
}

func (b *BaseEntity) CollisionAction(other Entity) {
	if b.ID == other.GetID() {
		return
	}

	b.Health -= other.GetBodyDamage()
	b.ResetActionTimer()

	// Attribution
	b.SetLastAttackerID(other.GetID())
}

func (b *BaseEntity) IsAlive() bool               { return b.Health > 0 }
func (b *BaseEntity) GetHealth() float64          { return b.Health }
func (b *BaseEntity) SetHealth(h float64)         { b.Health = h }
func (b *BaseEntity) GetBodyDamage() float64      { return b.BodyDamage }
func (b *BaseEntity) ResetActionTimer()           { b.TicksSinceAction = 0 }
func (b *BaseEntity) GetLastAttackerID() string   { return b.LastAttackerID }
func (b *BaseEntity) SetLastAttackerID(id string) { b.LastAttackerID = id }

// -------- TANK --------
type Tank struct {
	BaseEntity
	Config           *GameConfig
	InputVector      Vector2
	Orientation      float64
	MaxSpeed         float64
	MoveAcceleration float64
	Score            float64
	Kills            int
	RegenRate        float64
	QuickRegenRate   float64
	RegenCooldown    int
	FireCooldown     int
	LastFireTick     int
	ViewRange        float64
}

var _ Entity = (*Tank)(nil)

func NewTank(startV Vector2, config *GameConfig) *Tank {
	return &Tank{
		BaseEntity: BaseEntity{
			MovingCollidable: MovingCollidable{
				ID:     uuid.New().String(),
				Vel:    Vector2{X: 0, Y: 0},
				Object: &Circle{Center: startV, Radius: config.TankRadius},
				Weight: config.TankWeight,
			},
			Health:           config.TankMaxHealth,
			MaxHealth:        config.TankMaxHealth,
			BodyDamage:       config.TankBodyDamage,
			TicksSinceAction: 0,
		},
		Config:           config,
		InputVector:      Vector2{0.0, 0.0},
		MaxSpeed:         config.TankMaxSpeed,
		MoveAcceleration: config.TankAcceleration,
		RegenRate:        config.TankRegenRate,
		QuickRegenRate:   config.TankQuickRegenRate,
		RegenCooldown:    config.TankRegenCooldown,
		FireCooldown:     config.TankFireCooldown,
		ViewRange:        config.ViewRange,
	}
}

func (t *Tank) TickCalculation(friction float64) {
	t.ApplyForce(t.InputVector.Scale(t.MoveAcceleration * t.Weight))
	t.SetPosition(t.GetPosition().Add(t.Vel))
	t.Vel = t.Vel.Scale(friction)
	speed := t.Vel.Length()
	if speed > t.MaxSpeed && t.MaxSpeed > 0 {
		t.Vel = t.Vel.Scale(t.MaxSpeed / speed)
	}
	if t.Health < t.MaxHealth {
		t.Health += t.RegenRate
		if t.TicksSinceAction >= t.RegenCooldown {
			t.Health += t.QuickRegenRate
		}
		if t.Health > t.MaxHealth {
			t.Health = t.MaxHealth
		}
	}
	t.TicksSinceAction++
}

func (t *Tank) CollisionAction(other Entity) { t.BaseEntity.CollisionAction(other) }

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
	offset := radius + bulletRadius + 1.0
	spawnPos := Vector2{X: pos.X + dirX*offset, Y: pos.Y + dirY*offset}

	return NewBullet(t.ID, spawnPos, bulletVelX, bulletVelY, bulletWeight, bulletRadius, bulletDamage, t.Config.BulletLifespan)
}

// -------- BULLET --------
type Bullet struct {
	BaseEntity
	LifespanTicks int
	OwnerID       string
}

var _ Entity = (*Bullet)(nil)

func NewBullet(ownerID string, startV Vector2, dirX, dirY, weight, radius, damage float64, lifespan int) *Bullet {
	return &Bullet{
		BaseEntity: BaseEntity{
			MovingCollidable: MovingCollidable{
				ID:     uuid.New().String(),
				Vel:    Vector2{X: dirX, Y: dirY},
				Object: &Circle{Center: startV, Radius: radius},
				Weight: weight,
			},
			Health:     1.0,
			MaxHealth:  1.0,
			BodyDamage: damage,
		},
		LifespanTicks: lifespan,
		OwnerID:       ownerID,
	}
}

func (b *Bullet) CollisionAction(other Entity) {
	if b.OwnerID == other.GetID() {
		return
	}
	b.BaseEntity.CollisionAction(other)
	b.Health = 0
}

func (b *Bullet) TickCalculation(friction float64) {
	b.SetPosition(b.GetPosition().Add(b.Vel))
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
			MovingCollidable: MovingCollidable{
				ID:     uuid.New().String(),
				Object: obj,
				Weight: config.Weight,
			},
			Health:     config.Health,
			MaxHealth:  config.Health,
			BodyDamage: config.BodyDamage,
		},
		Type:       fType,
		ScoreValue: config.ScoreValue,
	}
}

func (f *Food) TickCalculation(friction float64) {
	f.SetPosition(f.GetPosition().Add(f.Vel))
	f.Vel = f.Vel.Scale(friction)
}

func (f *Food) CollisionAction(other Entity) { f.BaseEntity.CollisionAction(other) }
