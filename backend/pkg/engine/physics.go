package engine

import (
	"math"
	"sync"
)

type GridCoord struct {
	X, Y int
}

// getCoord converts a world position into a grid cell ID
func getCoord(pos Vector2, cellSize float64) GridCoord {
	return GridCoord{
		X: int(pos.X / cellSize),
		Y: int(pos.Y / cellSize),
	}
}

type CollisionPair struct {
	A, B Entity
}

// This handles static arena walls and entity collisions
func (g *Game) CheckAllCollisions(arena *Arena) {
	grid := make(map[GridCoord][]Entity)

	for _, e := range g.Entities {
		resolveEntityVsArena(e, arena)

		coord := getCoord(e.GetPosition(), g.Config.CellSize)
		grid[coord] = append(grid[coord], e)
	}

	var wg sync.WaitGroup
	var pairsMu sync.Mutex
	allPairs := make([]CollisionPair, 0)

	offsets := []GridCoord{
		{0, 0}, {-1, 0}, {1, 0}, {0, -1}, {0, 1},
		{-1, -1}, {-1, 1}, {1, -1}, {1, 1},
	}

	for i := 0; i < len(g.Entities); i++ {
		wg.Add(1)

		// Spawn a routine for EVERY entity
		go func(e1 Entity) {
			defer wg.Done()
			localPairs := make([]CollisionPair, 0)

			baseCoord := getCoord(e1.GetPosition(), g.Config.CellSize)

			for _, offset := range offsets {
				checkCoord := GridCoord{X: baseCoord.X + offset.X, Y: baseCoord.Y + offset.Y}

				// Maps are safe for concurrent READS in Go
				if cellEntities, exists := grid[checkCoord]; exists {
					for _, e2 := range cellEntities {
						// no collision pairs repeats
						if e1.GetID() < e2.GetID() {
							if checkCollision(e1, e2) {
								localPairs = append(localPairs, CollisionPair{A: e1, B: e2})
							}
						}
					}
				}
			}

			if len(localPairs) > 0 {
				pairsMu.Lock()
				allPairs = append(allPairs, localPairs...)
				pairsMu.Unlock()
			}
		}(g.Entities[i])
	}

	// waiting for the routines to complete
	wg.Wait()

	for _, pair := range allPairs {
		resolveCollision(pair.A, pair.B)
	}
}

// ---------------------------------------------------------
// COLLISION MATH HELPERS
// ---------------------------------------------------------

func clamp(val, min, max float64) float64 {
	if val < min {
		return min
	}
	if val > max {
		return max
	}
	return val
}

func resolveEntityVsArena(entity Entity, arena *Arena) {
	circ, isCirc := entity.GetObject().(*Circle)
	if !isCirc {
		return
	}

	for _, obs := range arena.Obstacles {
		switch shape := obs.(type) {
		case *Square:
			halfSide := shape.SideLength / 2
			minX, maxX := shape.Center.X-halfSide, shape.Center.X+halfSide
			minY, maxY := shape.Center.Y-halfSide, shape.Center.Y+halfSide

			closestX := clamp(circ.Center.X, minX, maxX)
			closestY := clamp(circ.Center.Y, minY, maxY)

			dx := circ.Center.X - closestX
			dy := circ.Center.Y - closestY
			distanceSquared := dx*dx + dy*dy

			if distanceSquared < circ.Radius*circ.Radius {
				distance := math.Sqrt(distanceSquared)
				if distance == 0 {
					entity.SetPosition(Vector2{X: maxX + circ.Radius, Y: circ.Center.Y})
					continue
				}

				overlap := circ.Radius - distance
				nx := dx / distance
				ny := dy / distance

				pos := entity.GetPosition()
				entity.SetPosition(Vector2{
					X: pos.X + (nx * overlap),
					Y: pos.Y + (ny * overlap),
				})
			}
		}
	}
}

func checkCollision(a, b Entity) bool {
	// check if either center is contained within the other object's geometry
	return a.GetObject().Contains(b.GetPosition()) || b.GetObject().Contains(a.GetPosition())
}

func resolveCollision(a, b Entity) {
	posA := a.GetPosition()
	posB := b.GetPosition()

	// Get dimensions for physical response
	dimA := getShapeDimension(a.GetObject())
	dimB := getShapeDimension(b.GetObject())

	dx := posB.X - posA.X
	dy := posB.Y - posA.Y
	distance := math.Sqrt(dx*dx + dy*dy)

	if distance == 0 {
		a.SetPosition(Vector2{X: posA.X + 0.1, Y: posA.Y})
		b.SetPosition(Vector2{X: posB.X - 0.1, Y: posB.Y})
		return
	}

	overlap := (dimA + dimB) - distance
	if overlap < 0 {
		overlap = 0
	}

	nx := dx / distance
	ny := dy / distance

	// Static resolution
	push := Vector2{X: nx * (overlap / 2), Y: ny * (overlap / 2)}
	a.SetPosition(Vector2{X: posA.X - push.X, Y: posA.Y - push.Y})
	b.SetPosition(Vector2{X: posB.X + push.X, Y: posB.Y + push.Y})

	// Relative velocity
	relVel := b.GetVelocity().Sub(a.GetVelocity())

	// Velocity along the normal
	velAlongNormal := relVel.X*nx + relVel.Y*ny

	// Do not resolve if velocities are separating
	if velAlongNormal < 0 {
		j := -(1.5) * velAlongNormal // 0.5 bounciness
		j /= (1 / a.GetWeight()) + (1 / b.GetWeight())

		impulse := Vector2{X: nx * j, Y: ny * j}
		a.ApplyForce(impulse.Scale(-1 / a.GetWeight()))
		b.ApplyForce(impulse.Scale(1 / b.GetWeight()))
	}

	a.OnCollision(b)
	b.OnCollision(a)
}

func getShapeDimension(obj GeomObject) float64 {
	switch shape := obj.(type) {
	case *Circle:
		return shape.Radius
	case *Square:
		return shape.SideLength / 2
	case *Triangle:
		return shape.Size / 2
	case *Pentagon:
		return shape.Size / 2
	default:
		return 0
	}
}
