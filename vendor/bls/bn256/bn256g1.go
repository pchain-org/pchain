package bn256

import (
	"errors"
	"math/big"
)

// G1 is an abstract cyclic group. The zero value is suitable for use as the
// output of an operation, but cannot be used as an input.
type G1 struct {
	p *curvePoint
}

func (g *G1) String() string {
	return "bn256.G1" + g.p.String()
}

// Base set e to g where g is the generator of the group and then returns e.
func (e *G1) Base() *G1 {
	if e.p == nil {
		e.p = &curvePoint{}
	}
	e.p.Set(curveGen)
	return e
}

// Zero set e to the null element and then returns e.
func (e *G1) Zero() *G1 {
	if e.p == nil {
		e.p = &curvePoint{}
	}
	e.p.SetInfinity()
	return e
}

// IsZero returns whether e is zero or not.
func (e *G1) IsZero() bool {
	if e.p == nil {
		return true
	}
	return e.p.IsInfinity()
}

// ScalarBaseMult sets e to g*k where g is the generator of the group and then
// returns e.
func (e *G1) ScalarBaseMult(k *big.Int) *G1 {
	if e.p == nil {
		e.p = &curvePoint{}
	}
	e.p.Mul(curveGen, k)
	return e
}

// ScalarMult sets e to a*k and then returns e.
func (e *G1) ScalarMult(a *G1, k *big.Int) *G1 {
	if e.p == nil {
		e.p = &curvePoint{}
	}
	e.p.Mul(a.p, k)
	return e
}

// Add sets e to a+b and then returns e.
func (e *G1) Add(a, b *G1) *G1 {
	if e.p == nil {
		e.p = &curvePoint{}
	}
	e.p.Add(a.p, b.p)
	return e
}

// Neg sets e to -a and then returns e.
func (e *G1) Neg(a *G1) *G1 {
	if e.p == nil {
		e.p = &curvePoint{}
	}
	e.p.Neg(a.p)
	return e
}

// Set sets e to a and then returns e.
func (e *G1) Set(a *G1) *G1 {
	if e.p == nil {
		e.p = &curvePoint{}
	}
	e.p.Set(a.p)
	return e
}

// Marshal converts e to a byte slice.
func (e *G1) Marshal() []byte {
	// Each value is a 256-bit number.
	const numBytes = 256 / 8

	if e.p == nil {
		e.p = &curvePoint{}
	}

	e.p.MakeAffine()
	ret := make([]byte, numBytes*2)
	if e.p.IsInfinity() {
		return ret
	}

	temp := &gfP{}
	montDecode(temp, &e.p.x)
	temp.Marshal(ret)
	montDecode(temp, &e.p.y)
	temp.Marshal(ret[numBytes:])

	return ret
}

// Unmarshal sets e to the result of converting the output of Marshal back into
// a group element and then returns e.
func (e *G1) Unmarshal(m []byte) error {
	// Each value is a 256-bit number.
	const numBytes = 256 / 8

	if len(m) != 2*numBytes {
		return errors.New("bn256.G1: incorrect data length")
	}

	if e.p == nil {
		e.p = &curvePoint{}
	}

	e.p.x.Unmarshal(m)
	e.p.y.Unmarshal(m[numBytes:])
	montEncode(&e.p.x, &e.p.x)
	montEncode(&e.p.y, &e.p.y)

	if e.p.x.IsZero() && e.p.y.IsZero() {
		// This is the point at infinity.
		e.p.SetInfinity()
	} else {
		e.p.z.SetOne()
		e.p.t.SetOne()

		if !e.p.IsOnCurve() {
			return errors.New("bn256.G1: malformed point")
		}
	}

	return nil
}
