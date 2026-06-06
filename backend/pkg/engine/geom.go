package engine

import (
	"fmt"
	"math"
)

type Vector2 struct {
	X, Y float64
}

func (v Vector2) String() string {
	return fmt.Sprintf("(%.2f, %.2f)", v.X, v.Y)
}

func (v Vector2) Length() float64 {
	return math.Sqrt(v.X*v.X + v.Y*v.Y)
}

func (v Vector2) Add(other Vector2) Vector2 {
	return Vector2{X: v.X + other.X, Y: v.Y + other.Y}
}

func (v Vector2) Sub(other Vector2) Vector2 {
	return Vector2{X: v.X - other.X, Y: v.Y - other.Y}
}

func (v Vector2) DistanceSquaredTo(other Vector2) float64 {
	dx := v.X - other.X
	dy := v.Y - other.Y
	return dx*dx + dy*dy
}

func (v Vector2) Scale(multiplier float64) Vector2 {
	return Vector2{X: v.X * multiplier, Y: v.Y * multiplier}
}

// ---------------------------------------------------------
// GEOMETRICAL OBJECTS
// ---------------------------------------------------------

type GeomObject interface {
	GetCenter() Vector2
	SetCenter(pos Vector2)

	Area() float64
	Perimeter() float64

	Contains(point Vector2) bool
	// Intersects returns (hit, normal, overlap)
	Intersects(other GeomObject) (bool, Vector2, float64)
}

// ------------------- CIRCLE -------------------

type Circle struct {
	Center Vector2
	Radius float64
}

var _ GeomObject = (*Circle)(nil)

func (c *Circle) GetCenter() Vector2    { return c.Center }
func (c *Circle) SetCenter(pos Vector2) { c.Center = pos }
func (c *Circle) Area() float64         { return math.Pi * c.Radius * c.Radius }
func (c *Circle) Perimeter() float64    { return 2 * math.Pi * c.Radius }
func (c *Circle) Contains(point Vector2) bool {
	return c.Center.DistanceSquaredTo(point) <= (c.Radius * c.Radius)
}

func (c *Circle) Intersects(other GeomObject) (bool, Vector2, float64) {
	switch o := other.(type) {
	case *Circle:
		dx := o.Center.X - c.Center.X
		dy := o.Center.Y - c.Center.Y
		distSq := dx*dx + dy*dy
		radiusSum := c.Radius + o.Radius

		if distSq > radiusSum*radiusSum {
			return false, Vector2{}, 0
		}

		dist := math.Sqrt(distSq)
		if dist == 0 {
			return true, Vector2{X: 1, Y: 0}, radiusSum
		}

		return true, Vector2{X: dx / dist, Y: dy / dist}, radiusSum - dist

	case *Square:
		hit, normal, overlap := intersectsCircleSquare(c, o)
		return hit, normal, overlap
	default:
		return genericIntersects(c, o)
	}
}

// ------------------- SQUARE -------------------

type Square struct {
	Center     Vector2
	SideLength float64
}

var _ GeomObject = (*Square)(nil)

func (c *Square) GetCenter() Vector2    { return c.Center }
func (c *Square) SetCenter(pos Vector2) { c.Center = pos }
func (s *Square) Area() float64         { return s.SideLength * s.SideLength }
func (s *Square) Perimeter() float64    { return 4 * s.SideLength }

func (s *Square) Contains(point Vector2) bool {
	halfSide := s.SideLength / 2
	return math.Abs(point.X-s.Center.X) <= halfSide &&
		math.Abs(point.Y-s.Center.Y) <= halfSide
}

func (s *Square) Intersects(other GeomObject) (bool, Vector2, float64) {
	switch o := other.(type) {
	case *Circle:
		hit, normal, overlap := intersectsCircleSquare(o, s)
		return hit, normal.Scale(-1), overlap
	case *Square:
		dx := o.Center.X - s.Center.X
		dy := o.Center.Y - s.Center.Y
		halfA := s.SideLength / 2
		halfB := o.SideLength / 2

		// Simplified AABB resolution
		overlapX := (halfA + halfB) - math.Abs(dx)
		overlapY := (halfA + halfB) - math.Abs(dy)

		if overlapX <= 0 || overlapY <= 0 {
			return false, Vector2{}, 0
		}

		if overlapX < overlapY {
			nx := 1.0
			if dx < 0 {
				nx = -1.0
			}
			return true, Vector2{X: nx, Y: 0}, overlapX
		} else {
			ny := 1.0
			if dy < 0 {
				ny = -1.0
			}
			return true, Vector2{X: 0, Y: ny}, overlapY
		}
	default:
		return genericIntersects(s, o)
	}
}

// ------------------- TRIANGLE -------------------

type Triangle struct {
	Center Vector2
	Size   float64 // Distance from center to vertices
}

var _ GeomObject = (*Triangle)(nil)

func (t *Triangle) GetCenter() Vector2    { return t.Center }
func (t *Triangle) SetCenter(pos Vector2) { t.Center = pos }
func (t *Triangle) Area() float64         { return (3 * math.Sqrt(3) / 4) * t.Size * t.Size }
func (t *Triangle) Perimeter() float64    { return 3 * t.Size * math.Sqrt(3) }
func (t *Triangle) Contains(point Vector2) bool {
	v1 := Vector2{X: t.Center.X, Y: t.Center.Y - t.Size}
	v2 := Vector2{X: t.Center.X - t.Size*math.Sqrt(3)/2, Y: t.Center.Y + t.Size/2}
	v3 := Vector2{X: t.Center.X + t.Size*math.Sqrt(3)/2, Y: t.Center.Y + t.Size/2}
	d1 := sign(point, v1, v2)
	d2 := sign(point, v2, v3)
	d3 := sign(point, v3, v1)
	return !((d1 < 0 || d2 < 0 || d3 < 0) && (d1 > 0 || d2 > 0 || d3 > 0))
}

func (t *Triangle) Intersects(other GeomObject) (bool, Vector2, float64) {
	return genericIntersects(t, other)
}

// ------------------- PENTAGON -------------------

type Pentagon struct {
	Center Vector2
	Size   float64
}

var _ GeomObject = (*Pentagon)(nil)

func (p *Pentagon) GetCenter() Vector2    { return p.Center }
func (p *Pentagon) SetCenter(pos Vector2) { p.Center = pos }
func (p *Pentagon) Area() float64         { return (5.0 / 2.0) * p.Size * p.Size * math.Sin(2*math.Pi/5.0) }
func (p *Pentagon) Perimeter() float64    { return 5 * (2 * p.Size * math.Sin(math.Pi/5.0)) }
func (p *Pentagon) Contains(point Vector2) bool {
	sides := 5
	step := (math.Pi * 2) / float64(sides)
	for i := range sides {
		angle1 := float64(i) * step
		angle2 := float64(i+1) * step
		v1 := p.Center
		v2 := Vector2{X: p.Center.X + math.Sin(angle1)*p.Size, Y: p.Center.Y - math.Cos(angle1)*p.Size}
		v3 := Vector2{X: p.Center.X + math.Sin(angle2)*p.Size, Y: p.Center.Y - math.Cos(angle2)*p.Size}
		d1 := sign(point, v1, v2)
		d2 := sign(point, v2, v3)
		d3 := sign(point, v3, v1)
		if !((d1 < 0 || d2 < 0 || d3 < 0) && (d1 > 0 || d2 > 0 || d3 > 0)) {
			return true
		}
	}
	return false
}

func (p *Pentagon) Intersects(other GeomObject) (bool, Vector2, float64) {
	return genericIntersects(p, other)
}

// ------------------- HELPERS -------------------

func sign(p1, p2, p3 Vector2) float64 { return (p1.X-p3.X)*(p2.Y-p3.Y) - (p2.X-p3.X)*(p1.Y-p3.Y) }

func intersectsCircleSquare(c *Circle, s *Square) (bool, Vector2, float64) {
	halfSide := s.SideLength / 2
	closestX := math.Max(s.Center.X-halfSide, math.Min(c.Center.X, s.Center.X+halfSide))
	closestY := math.Max(s.Center.Y-halfSide, math.Min(c.Center.Y, s.Center.Y+halfSide))

	dx := closestX - c.Center.X
	dy := closestY - c.Center.Y
	distSq := dx*dx + dy*dy

	if distSq > c.Radius*c.Radius {
		// If circle center is inside square
		if s.Contains(c.Center) {
			// Find the closest edge and push out
			distToLeft := c.Center.X - (s.Center.X - halfSide)
			distToRight := (s.Center.X + halfSide) - c.Center.X
			distToTop := c.Center.Y - (s.Center.Y - halfSide)
			distToBottom := (s.Center.Y + halfSide) - c.Center.Y

			minDist := math.Min(math.Min(distToLeft, distToRight), math.Min(distToTop, distToBottom))

			if minDist == distToLeft {
				return true, Vector2{X: -1, Y: 0}, c.Radius + distToLeft
			}
			if minDist == distToRight {
				return true, Vector2{X: 1, Y: 0}, c.Radius + distToRight
			}
			if minDist == distToTop {
				return true, Vector2{X: 0, Y: -1}, c.Radius + distToTop
			}
			return true, Vector2{X: 0, Y: 1}, c.Radius + distToBottom
		}
		return false, Vector2{}, 0
	}

	dist := math.Sqrt(distSq)
	if dist == 0 {
		return true, Vector2{X: 1, Y: 0}, c.Radius
	}

	return true, Vector2{X: dx / dist, Y: dy / dist}, c.Radius - dist
}

func genericIntersects(a, b GeomObject) (bool, Vector2, float64) {
	dx := b.GetCenter().X - a.GetCenter().X
	dy := b.GetCenter().Y - a.GetCenter().Y
	distSq := dx*dx + dy*dy

	rA := getBoundingRadius(a)
	rB := getBoundingRadius(b)
	radiusSum := rA + rB

	if distSq > radiusSum*radiusSum {
		return false, Vector2{}, 0
	}

	dist := math.Sqrt(distSq)
	if dist == 0 {
		return true, Vector2{X: 1, Y: 0}, radiusSum
	}

	return true, Vector2{X: dx / dist, Y: dy / dist}, radiusSum - dist
}

func getBoundingRadius(obj GeomObject) float64 {
	switch shape := obj.(type) {
	case *Circle:
		return shape.Radius
	case *Square:
		return shape.SideLength * 0.707
	case *Triangle:
		return shape.Size
	case *Pentagon:
		return shape.Size
	default:
		return 0
	}
}

// ---------------------------------------------------------
// Arena
// ---------------------------------------------------------

type Arena struct {
	Width, Height float64
	Obstacles     []GeomObject
}

func NewArena(width, height float64) *Arena {
	return &Arena{Width: width, Height: height, Obstacles: []GeomObject{}}
}
