package engine

import "math"

type Vector2 struct {
	X, Y float64
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
}

// ------------------- CIRCLE -------------------

type Circle struct {
	Center Vector2
	Radius float64
}

// Compile time interface compliance check *note
var _ GeomObject = (*Circle)(nil)

func (c *Circle) GetCenter() Vector2    { return c.Center }
func (c *Circle) SetCenter(pos Vector2) { c.Center = pos }

func (c *Circle) Area() float64 {
	return math.Pi * c.Radius * c.Radius
}

func (c *Circle) Perimeter() float64 {
	return 2 * math.Pi * c.Radius
}

func (c *Circle) Contains(point Vector2) bool {
	dx := point.X - c.Center.X
	dy := point.Y - c.Center.Y
	distanceSquared := dx*dx + dy*dy

	return distanceSquared <= (c.Radius * c.Radius)
}

// ------------------- SQUARE -------------------

type Square struct {
	Center     Vector2
	SideLength float64
}

var _ GeomObject = (*Square)(nil)

func (c *Square) GetCenter() Vector2    { return c.Center }
func (c *Square) SetCenter(pos Vector2) { c.Center = pos }

func (s *Square) Area() float64 {
	return s.SideLength * s.SideLength
}

func (s *Square) Perimeter() float64 {
	return 4 * s.SideLength
}

func (s *Square) Contains(point Vector2) bool {
	halfSide := s.SideLength / 2

	inX := point.X >= (s.Center.X-halfSide) && point.X <= (s.Center.X+halfSide)
	inY := point.Y >= (s.Center.Y-halfSide) && point.Y <= (s.Center.Y+halfSide)

	return inX && inY
}

// ------------------- TRIANGLE -------------------

type Triangle struct {
	Center Vector2
	Size   float64 // Distance from center to vertices
}

var _ GeomObject = (*Triangle)(nil)

func (t *Triangle) GetCenter() Vector2    { return t.Center }
func (t *Triangle) SetCenter(pos Vector2) { t.Center = pos }

func (t *Triangle) Area() float64 {
	// Side length s = Size * sqrt(3)
	// Area = (sqrt(3)/4) * s^2 = (3 * sqrt(3) / 4) * Size^2
	return (3 * math.Sqrt(3) / 4) * t.Size * t.Size
}

func (t *Triangle) Perimeter() float64 {
	return 3 * t.Size * math.Sqrt(3)
}

func (t *Triangle) Contains(point Vector2) bool {
	// Vertices for an equilateral triangle based on Center and Size
	v1 := Vector2{X: t.Center.X, Y: t.Center.Y - t.Size}                           // Top
	v2 := Vector2{X: t.Center.X - t.Size*math.Sqrt(3)/2, Y: t.Center.Y + t.Size/2} // Bottom Left
	v3 := Vector2{X: t.Center.X + t.Size*math.Sqrt(3)/2, Y: t.Center.Y + t.Size/2} // Bottom Right

	d1 := sign(point, v1, v2)
	d2 := sign(point, v2, v3)
	d3 := sign(point, v3, v1)

	hasNeg := (d1 < 0) || (d2 < 0) || (d3 < 0)
	hasPos := (d1 > 0) || (d2 > 0) || (d3 > 0)

	return !(hasNeg && hasPos)
}

func sign(p1, p2, p3 Vector2) float64 {
	return (p1.X-p3.X)*(p2.Y-p3.Y) - (p2.X-p3.X)*(p1.Y-p3.Y)
}

// ------------------- PENTAGON -------------------

type Pentagon struct {
	Center Vector2
	Size   float64 // distance from center to vertices
}

var _ GeomObject = (*Pentagon)(nil)

func (p *Pentagon) GetCenter() Vector2    { return p.Center }
func (p *Pentagon) SetCenter(pos Vector2) { p.Center = pos }

func (p *Pentagon) Area() float64 {
	// Area = (5/2) * Size^2 * sin(72 degrees)
	return (5.0 / 2.0) * p.Size * p.Size * math.Sin(2*math.Pi/5.0)
}

func (p *Pentagon) Perimeter() float64 {
	// side = 2 * Size * sin(36 degrees)
	return 5 * (2 * p.Size * math.Sin(math.Pi/5.0))
}

func (p *Pentagon) Contains(point Vector2) bool {
	// A pentagon is 5 triangles meeting at the center
	sides := 5
	step := (math.Pi * 2) / float64(sides)

	for i := 0; i < sides; i++ {
		angle1 := float64(i) * step
		angle2 := float64(i+1) * step

		v1 := p.Center
		v2 := Vector2{
			X: p.Center.X + math.Sin(angle1)*p.Size,
			Y: p.Center.Y - math.Cos(angle1)*p.Size,
		}
		v3 := Vector2{
			X: p.Center.X + math.Sin(angle2)*p.Size,
			Y: p.Center.Y - math.Cos(angle2)*p.Size,
		}

		d1 := sign(point, v1, v2)
		d2 := sign(point, v2, v3)
		d3 := sign(point, v3, v1)

		if !((d1 < 0 || d2 < 0 || d3 < 0) && (d1 > 0 || d2 > 0 || d3 > 0)) {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------
// Arena
// ---------------------------------------------------------

type Arena struct {
	Width, Height float64
	Obstacles     []GeomObject
}

// NewArena creates a basic map
func NewArena(width, height float64) *Arena {
	return &Arena{
		Width:     width,
		Height:    height,
		Obstacles: []GeomObject{},
	}
}
