package bone

import "math"

type Vector2 struct {
	X float64
	Y float64
}

func (v *Vector2) Add(other Vector2) Vector2 {
	return Vector2{X: v.X + other.X, Y: v.Y + other.Y}
}

func (v *Vector2) Sub(other Vector2) Vector2 {
	return Vector2{X: v.X - other.X, Y: v.Y - other.Y}
}

func (v *Vector2) Mul(scalar float64) Vector2 {
	return Vector2{X: v.X * scalar, Y: v.Y * scalar}
}

func (v *Vector2) Magnitude() float64 {
	return math.Sqrt(v.X*v.X + v.Y*v.Y)
}

func (v *Vector2) Normalized() Vector2 {
	mag := v.Magnitude()
	return Vector2{X: v.X / mag, Y: v.Y / mag}
}

func (v *Vector2) Dot(other Vector2) float64 {
	return v.X*other.X + v.Y*other.Y
}

type Vector2i struct {
	X int
	Y int
}

func (v *Vector2i) Add(other Vector2i) Vector2i {
	return Vector2i{X: v.X + other.X, Y: v.Y + other.Y}
}

func (v *Vector2i) Sub(other Vector2i) Vector2i {
	return Vector2i{X: v.X - other.X, Y: v.Y - other.Y}
}

func (v *Vector2i) Mul(scalar int) Vector2i {
	return Vector2i{X: v.X * scalar, Y: v.Y * scalar}
}

func (v *Vector2i) Magnitude() float64 {
	return math.Sqrt(float64(v.X*v.X + v.Y*v.Y))
}

func (v *Vector2i) Normalized() Vector2 {
	mag := v.Magnitude()
	return Vector2{X: float64(v.X) / mag, Y: float64(v.Y) / mag}
}

func (v *Vector2i) Dot(other Vector2i) int {
	return v.X*other.X + v.Y*other.Y
}

func PowInt(x int, y int) int {
	return int(math.Pow(float64(x), float64(y)))
}
