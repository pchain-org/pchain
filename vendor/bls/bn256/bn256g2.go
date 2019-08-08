package bn256

import (
	"errors"
	"math/big"
)

// G2 is an abstract cyclic group. The zero value is suitable for use as the
// output of an operation, but cannot be used as an input.
type G2 struct {
	p *twistPoint
}

func (e *G2) String() string {
	return "bn256.G2" + e.p.String()
}

// Base set e to g where g is the generator of the group and then returns e.
func (e *G2) Base() *G2 {
	if e.p == nil {
		e.p = &twistPoint{}
	}
	e.p.Set(twistGen)
	return e
}

// Zero set e to the null element and then returns e.
func (e *G2) Zero() *G2 {
	if e.p == nil {
		e.p = &twistPoint{}
	}
	e.p.SetInfinity()
	return e
}

// IsZero returns whether e is zero or not.
func (e *G2) IsZero() bool {
	if e.p == nil {
		return true
	}
	return e.p.IsInfinity()
}

// ScalarBaseMult sets e to g*k where g is the generator of the group and then
// returns out.
func (e *G2) ScalarBaseMult(k *big.Int) *G2 {
	if e.p == nil {
		e.p = &twistPoint{}
	}
	e.p.Mul(twistGen, k)
	return e
}

// ScalarMult sets e to a*k and then returns e.
func (e *G2) ScalarMult(a *G2, k *big.Int) *G2 {
	if e.p == nil {
		e.p = &twistPoint{}
	}
	e.p.Mul(a.p, k)
	return e
}

// Add sets e to a+b and then returns e.
func (e *G2) Add(a, b *G2) *G2 {
	if e.p == nil {
		e.p = &twistPoint{}
	}
	e.p.Add(a.p, b.p)
	return e
}

// Neg sets e to -a and then returns e.
func (e *G2) Neg(a *G2) *G2 {
	if e.p == nil {
		e.p = &twistPoint{}
	}
	e.p.Neg(a.p)
	return e
}

// Set sets e to a and then returns e.
func (e *G2) Set(a *G2) *G2 {
	if e.p == nil {
		e.p = &twistPoint{}
	}
	e.p.Set(a.p)
	return e
}

// Marshal converts e into a byte slice.
func (e *G2) Marshal() []byte {
	// Each value is a 256-bit number.
	const numBytes = 256 / 8

	if e.p == nil {
		e.p = &twistPoint{}
	}

	e.p.MakeAffine()
	ret := make([]byte, numBytes*4)
	if e.p.IsInfinity() {
		return ret
	}

	temp := &gfP{}
	montDecode(temp, &e.p.x.x)
	temp.Marshal(ret)
	montDecode(temp, &e.p.x.y)
	temp.Marshal(ret[numBytes:])
	montDecode(temp, &e.p.y.x)
	temp.Marshal(ret[2*numBytes:])
	montDecode(temp, &e.p.y.y)
	temp.Marshal(ret[3*numBytes:])

	return ret
}

// Unmarshal sets e to the result of converting the output of Marshal back into
// a group element and then returns e.
func (e *G2) Unmarshal(m []byte) error {
	// Each value is a 256-bit number.
	const numBytes = 256 / 8

	if len(m) != 4*numBytes {
		return errors.New("bn256.G2: incorrect data length")
	}

	if e.p == nil {
		e.p = &twistPoint{}
	}

	e.p.x.x.Unmarshal(m)
	e.p.x.y.Unmarshal(m[numBytes:])
	e.p.y.x.Unmarshal(m[2*numBytes:])
	e.p.y.y.Unmarshal(m[3*numBytes:])
	montEncode(&e.p.x.x, &e.p.x.x)
	montEncode(&e.p.x.y, &e.p.x.y)
	montEncode(&e.p.y.x, &e.p.y.x)
	montEncode(&e.p.y.y, &e.p.y.y)

	if e.p.x.IsZero() && e.p.y.IsZero() {
		// This is the point at infinity.
		e.p.SetInfinity()
	} else {
		e.p.z.SetOne()
		e.p.t.SetOne()

		if !e.p.IsOnCurve() {
			return errors.New("bn256.G2: malformed point")
		}
	}

	return nil
}
