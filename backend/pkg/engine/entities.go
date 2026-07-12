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
	GetMaxHealth() float64
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
	// Self is the outer concrete entity (Tank/Bullet/Food), set by each
	// constructor. Go embedding gives no virtual dispatch: a plain
	// b.CollisionAction(ent) call from here would always resolve to
	// BaseEntity's own method, silently skipping Tank/Bullet/Food
	// overrides. Self lets OnCollision call through the Entity interface
	// so those overrides actually run.
	// json:"-": Self points back at the entity itself; these structs are
	// marshaled to clients every tick and a naive encode would recurse
	// forever.
	Self Entity `json:"-"`
}

func (b *BaseEntity) OnCollision(other Collidable, normal Vector2, overlap float64) {
	b.MovingCollidable.OnCollision(other, normal, overlap)
	ent, ok := other.(Entity)
	if !ok {
		return
	}
	if b.Self != nil {
		b.Self.CollisionAction(ent)
	} else {
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
func (b *BaseEntity) GetMaxHealth() float64       { return b.MaxHealth }
func (b *BaseEntity) SetHealth(h float64)         { b.Health = h }
func (b *BaseEntity) GetBodyDamage() float64      { return b.BodyDamage }
func (b *BaseEntity) ResetActionTimer()           { b.TicksSinceAction = 0 }
func (b *BaseEntity) GetLastAttackerID() string   { return b.LastAttackerID }
func (b *BaseEntity) SetLastAttackerID(id string) { b.LastAttackerID = id }

// -------- TANK --------
type Tank struct {
	BaseEntity
	Name              string
	Config            *GameConfig
	InputVector       Vector2
	Orientation       float64
	MaxSpeed          float64
	MoveAcceleration  float64
	Score             float64
	Kills             int
	RegenRate         float64
	QuickRegenRate    float64
	RegenCooldown     int
	BulletMuzzleSpeed float64
	FireCooldown      int
	LastFireTick      int
	ViewRange         float64
	IsBot             bool `json:"is_bot"`

	// Accuracy stats (raw counters; weighting/miss-rate lives in Python).
	ShotsFired int `json:"shots_fired"`
	HitsTank   int `json:"hits_tank"`
	HitsFood   int `json:"hits_food"`
}

var _ Entity = (*Tank)(nil)

func NewTank(name string, startV Vector2, config *GameConfig) *Tank {
	t := &Tank{
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
		Name:              name,
		Config:            config,
		InputVector:       Vector2{0.0, 0.0},
		MaxSpeed:          config.TankMaxSpeed,
		MoveAcceleration:  config.TankAcceleration,
		RegenRate:         config.TankRegenRate,
		QuickRegenRate:    config.TankQuickRegenRate,
		RegenCooldown:     config.TankRegenCooldown,
		BulletMuzzleSpeed: config.BulletMuzzleSpeed,
		FireCooldown:      config.TankFireCooldown,
		ViewRange:         config.ViewRange,
	}
	t.Self = t
	return t
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

func (t *Tank) CollisionAction(other Entity) {
	if b, ok := other.(*Bullet); ok {
		// Own bullet (e.g. after an obstacle ricochet): no damage and no
		// attribution — mirrors the owner guard in Bullet.CollisionAction.
		if b.OwnerID == t.ID {
			return
		}
		// Accuracy attribution: a bullet just damaged this tank.
		if b.Owner != nil {
			b.Owner.HitsTank++
		}
	}
	t.BaseEntity.CollisionAction(other)
}

func (t *Tank) Fire(orientation float64, currentTick int) *Bullet {
	if currentTick-t.LastFireTick < t.FireCooldown {
		return nil
	}

	t.LastFireTick = currentTick
	t.ShotsFired++
	pos := t.GetPosition()
	radius := t.Config.TankRadius
	if circ, ok := t.Object.(*Circle); ok {
		radius = circ.Radius
	}

	dirX, dirY := math.Cos(orientation), math.Sin(orientation)

	// Proportional scaling based on current Tank size relative to base
	scaleFactor := radius / t.Config.TankRadius
	bulletRadius := t.Config.BulletRadius * scaleFactor
	bulletWeight := t.Config.BulletWeight * scaleFactor
	bulletDamage := t.Config.BulletDamage * scaleFactor

	// Bullet Velocity = Muzzle Speed
	bulletVelX := (dirX * t.BulletMuzzleSpeed)
	bulletVelY := (dirY * t.BulletMuzzleSpeed)

	// Recoil Force (Opposite of shot)
	recoilForce := Vector2{X: -dirX, Y: -dirY}.Scale(t.Config.RecoilPower * bulletWeight)
	t.ApplyForce(recoilForce)

	// Spawn exactly on the surface of the tank
	offset := radius + bulletRadius + 1.0
	spawnPos := Vector2{X: pos.X + dirX*offset, Y: pos.Y + dirY*offset}

	b := NewBullet(t.ID, spawnPos, bulletVelX, bulletVelY, bulletWeight, bulletRadius, bulletDamage, t.Config.BulletLifespan)
	b.Owner = t
	return b
}

// -------- BULLET --------
type Bullet struct {
	BaseEntity
	LifespanTicks int
	OwnerID       string
	// Owner is the firing tank, used for accuracy-stat attribution
	// (contracts §accuracy). Never marshaled: bullets are serialized to
	// clients every tick and a *Tank pointer must not go over the wire.
	Owner *Tank `json:"-"`
}

var _ Entity = (*Bullet)(nil)

func NewBullet(ownerID string, startV Vector2, dirX, dirY, weight, radius, damage float64, lifespan int) *Bullet {
	b := &Bullet{
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
	b.Self = b
	return b
}

func (b *Bullet) CollisionAction(other Entity) {
	if b.OwnerID == other.GetID() {
		return
	}
	b.BaseEntity.CollisionAction(other)
	b.Health = 0
}

// OnCollision overrides BaseEntity's so bullets also die on static obstacles
// (not Entities, so CollisionAction never sees them). Without this a bullet
// ricochets off rocks and can fly back into its owner; an obstacle hit is a
// miss, so the bullet ends there.
func (b *Bullet) OnCollision(other Collidable, normal Vector2, overlap float64) {
	b.BaseEntity.OnCollision(other, normal, overlap)
	if _, isEntity := other.(Entity); !isEntity {
		b.Health = 0
	}
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
	f := &Food{
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
	f.Self = f
	return f
}

func (f *Food) TickCalculation(friction float64) {
	f.SetPosition(f.GetPosition().Add(f.Vel))
	f.Vel = f.Vel.Scale(friction)
}

func (f *Food) CollisionAction(other Entity) {
	f.BaseEntity.CollisionAction(other)
	if b, ok := other.(*Bullet); ok && b.Owner != nil {
		b.Owner.HitsFood++
	}
}
