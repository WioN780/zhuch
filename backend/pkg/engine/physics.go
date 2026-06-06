package engine

import (
	"sync"
)

// Collidable defines the interface for anything that can participate in the physics engine
type Collidable interface {
	GetID() string
	GetGeom() GeomObject

	GetPosition() Vector2
	SetPosition(pos Vector2)
	GetVelocity() Vector2
	SetVelocity(vel Vector2)

	GetWeight() float64
	IsStatic() bool

	ApplyForce(force Vector2)

	// Physics engine entry point
	OnCollision(other Collidable, normal Vector2, overlap float64)
}

type MovingCollidable struct {
	ID     string
	Vel    Vector2
	Object GeomObject
	Weight float64
}

func (m *MovingCollidable) GetID() string            { return m.ID }
func (m *MovingCollidable) GetGeom() GeomObject      { return m.Object }
func (m *MovingCollidable) GetPosition() Vector2     { return m.Object.GetCenter() }
func (m *MovingCollidable) SetPosition(pos Vector2)  { m.Object.SetCenter(pos) }
func (m *MovingCollidable) GetVelocity() Vector2     { return m.Vel }
func (m *MovingCollidable) SetVelocity(vel Vector2)  { m.Vel = vel }
func (m *MovingCollidable) GetWeight() float64       { return m.Weight }
func (m *MovingCollidable) IsStatic() bool           { return false }
func (m *MovingCollidable) ApplyForce(force Vector2) { m.Vel = m.Vel.Add(force.Scale(1.0 / m.Weight)) }

func (m *MovingCollidable) OnCollision(other Collidable, normal Vector2, overlap float64) {
	// Static Resolution (The Push)
	ratio := other.GetWeight() / (m.Weight + other.GetWeight())
	if other.IsStatic() {
		ratio = 1.0
	}
	m.Object.SetCenter(m.Object.GetCenter().Sub(normal.Scale(overlap * ratio)))

	// Momentum Resolution (The Bounce)
	relVel := other.GetVelocity().Sub(m.Vel)
	velAlongNormal := relVel.X*normal.X + relVel.Y*normal.Y

	if velAlongNormal < 0 {
		restitution := 0.5
		j := -(1.0 + restitution) * velAlongNormal
		invMassSelf := 1.0 / m.Weight
		invMassOther := 1.0 / other.GetWeight()
		if other.IsStatic() {
			invMassOther = 0
		}

		j /= (invMassSelf + invMassOther)
		m.ApplyForce(normal.Scale(-j))
	}
}

// StaticCollidable defines all the obstacles
type StaticCollidable struct {
	Object GeomObject
}

func (s *StaticCollidable) GetID() string                                                 { return "static_" + s.Object.GetCenter().String() }
func (s *StaticCollidable) GetGeom() GeomObject                                           { return s.Object }
func (s *StaticCollidable) GetPosition() Vector2                                          { return s.Object.GetCenter() }
func (s *StaticCollidable) SetPosition(pos Vector2)                                       { s.Object.SetCenter(pos) }
func (s *StaticCollidable) GetVelocity() Vector2                                          { return Vector2{0, 0} }
func (s *StaticCollidable) SetVelocity(vel Vector2)                                       {}
func (s *StaticCollidable) GetWeight() float64                                            { return 1e18 }
func (s *StaticCollidable) IsStatic() bool                                                { return true }
func (s *StaticCollidable) ApplyForce(force Vector2)                                      {}
func (s *StaticCollidable) OnCollision(other Collidable, normal Vector2, overlap float64) {}

// Grid for collision computation
type GridCoord struct {
	X, Y int
}

func getCoord(pos Vector2, cellSize float64) GridCoord {
	return GridCoord{
		X: int(pos.X / cellSize),
		Y: int(pos.Y / cellSize),
	}
}

// CheckAllCollisions just triggers interactions between nearby objects
func (g *Game) CheckAllCollisions(arena *Arena) {
	var allCollidables []Collidable
	for _, e := range g.Entities {
		allCollidables = append(allCollidables, e.(Collidable))
	}
	for _, obs := range arena.Obstacles {
		allCollidables = append(allCollidables, &StaticCollidable{Object: obs})
	}

	grid := make(map[GridCoord][]Collidable)
	for _, c := range allCollidables {
		coord := getCoord(c.GetPosition(), g.Config.CellSize)
		grid[coord] = append(grid[coord], c)
	}

	var wg sync.WaitGroup
	offsets := []GridCoord{
		{0, 0}, {-1, 0}, {1, 0}, {0, -1}, {0, 1},
		{-1, -1}, {-1, 1}, {1, -1}, {1, 1},
	}

	for i := 0; i < len(allCollidables); i++ {
		c1 := allCollidables[i]
		if c1.IsStatic() {
			continue
		}

		wg.Add(1)
		go func(obj1 Collidable) {
			defer wg.Done()
			baseCoord := getCoord(obj1.GetPosition(), g.Config.CellSize)

			for _, offset := range offsets {
				checkCoord := GridCoord{X: baseCoord.X + offset.X, Y: baseCoord.Y + offset.Y}

				if cellObjects, exists := grid[checkCoord]; exists {
					for _, obj2 := range cellObjects {
						if obj1.GetID() < obj2.GetID() {
							if hit, normal, overlap := obj1.GetGeom().Intersects(obj2.GetGeom()); hit {
								obj1.OnCollision(obj2, normal, overlap)
								obj2.OnCollision(obj1, normal.Scale(-1), overlap)
							}
						}
					}
				}
			}
		}(c1)
	}
	wg.Wait()
}
