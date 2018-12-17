package bn256

import (
	"errors"
	"math/big"
)

// GT is an abstract cyclic group. The zero value is suitable for use as the
// output of an operation, but cannot be used as an input.
type GT struct {
	p *gfP12
}

// Pair calculates an Optimal Ate pairing.
func Pair(g1 *G1, g2 *G2) *GT {
	return &GT{optimalAte(g2.p, g1.p)}
}

// Miller applies Miller's algorithm, which is a bilinear function from the
// source groups to F_p^12. Miller(g1, g2).Finalize() is equivalent to Pair(g1,
// g2).
func Miller(g1 *G1, g2 *G2) *GT {
	return &GT{miller(g2.p, g1.p)}
}

func (g *GT) String() string {
	return "bn256.GT" + g.p.String()
}

// Base set e to g where g is the generator of the group and then returns e.
func (e *GT) Base() *GT {
	if e.p == nil {
		e.p = &gfP12{}
	}
	e.p.Set(gfP12Gen)
	return e
}

// Zero set e to the null element and then returns e.
func (e *GT) Zero() *GT {
	if e.p == nil {
		e.p = &gfP12{}
	}
	e.p.SetZero()
	return e
}

// IsZero returns whether e is zero or not.
func (e *GT) IsZero() bool {
	if e.p == nil {
		return true
	}
	return e.p.IsZero()
}

// Unit set e to multiplicative identity
func (e *GT) Unit() *GT {
	if e.p == nil {
		e.p = &gfP12{}
	}
	e.p.SetOne()
	return e
}

// ScalarBaseMult sets e to g*k where g is the generator of the group and then
// returns out.
func (e *GT) ScalarBaseMult(k *big.Int) *GT {
	if e.p == nil {
		e.p = &gfP12{}
	}
	e.p.latticeExp(gfP12Gen, k)
	return e
}

// ScalarMult sets e to a*k and then returns e. (If e is not guaranteed to be an element of the group because it is the
// output of Miller(), use ScalarMultSimple.)
func (e *GT) ScalarMult(a *GT, k *big.Int) *GT {
	if e.p == nil {
		e.p = &gfP12{}
	}
	e.p.latticeExp(a.p, k)
	return e
}

// ScalarMultSimple sets e to a*k and then returns e.
func (e *GT) ScalarMultSimple(a *GT, k *big.Int) *GT {
	if e.p == nil {
		e.p = &gfP12{}
	}
	e.p.Exp(a.p, k)
	return e
}

// Add sets e to a+b and then returns e.
func (e *GT) Add(a, b *GT) *GT {
	if e.p == nil {
		e.p = &gfP12{}
	}
	e.p.Mul(a.p, b.p)
	return e
}

// Neg sets e to -a and then returns e.
func (e *GT) Neg(a *GT) *GT {
	if e.p == nil {
		e.p = &gfP12{}
	}
	e.p.Conjugate(a.p)
	return e
}

// Set sets e to a and then returns e.
func (e *GT) Set(a *GT) *GT {
	if e.p == nil {
		e.p = &gfP12{}
	}
	e.p.Set(a.p)
	return e
}

// Finalize is a linear function from F_p^12 to GT.
func (e *GT) Finalize() *GT {
	ret := finalExponentiation(e.p)
	e.p.Set(ret)
	return e
}

// Marshal converts e into a byte slice.
func (e *GT) Marshal() []byte {
	// Each value is a 256-bit number.
	const numBytes = 256 / 8

	if e.p == nil {
		e.p = &gfP12{}
	}

	ret := make([]byte, numBytes*12)
	temp := &gfP{}

	montDecode(temp, &e.p.x.x.x)
	temp.Marshal(ret)
	montDecode(temp, &e.p.x.x.y)
	temp.Marshal(ret[numBytes:])
	montDecode(temp, &e.p.x.y.x)
	temp.Marshal(ret[2*numBytes:])
	montDecode(temp, &e.p.x.y.y)
	temp.Marshal(ret[3*numBytes:])
	montDecode(temp, &e.p.x.z.x)
	temp.Marshal(ret[4*numBytes:])
	montDecode(temp, &e.p.x.z.y)
	temp.Marshal(ret[5*numBytes:])
	montDecode(temp, &e.p.y.x.x)
	temp.Marshal(ret[6*numBytes:])
	montDecode(temp, &e.p.y.x.y)
	temp.Marshal(ret[7*numBytes:])
	montDecode(temp, &e.p.y.y.x)
	temp.Marshal(ret[8*numBytes:])
	montDecode(temp, &e.p.y.y.y)
	temp.Marshal(ret[9*numBytes:])
	montDecode(temp, &e.p.y.z.x)
	temp.Marshal(ret[10*numBytes:])
	montDecode(temp, &e.p.y.z.y)
	temp.Marshal(ret[11*numBytes:])

	return ret
}

// Unmarshal sets e to the result of converting the output of Marshal back into
// a group element and then returns e.
func (e *GT) Unmarshal(m []byte) error {
	// Each value is a 256-bit number.
	const numBytes = 256 / 8

	if len(m) != 12*numBytes {
		return errors.New("bn256.GT: incorrect data length")
	}

	if e.p == nil {
		e.p = &gfP12{}
	}

	e.p.x.x.x.Unmarshal(m)
	e.p.x.x.y.Unmarshal(m[numBytes:])
	e.p.x.y.x.Unmarshal(m[2*numBytes:])
	e.p.x.y.y.Unmarshal(m[3*numBytes:])
	e.p.x.z.x.Unmarshal(m[4*numBytes:])
	e.p.x.z.y.Unmarshal(m[5*numBytes:])
	e.p.y.x.x.Unmarshal(m[6*numBytes:])
	e.p.y.x.y.Unmarshal(m[7*numBytes:])
	e.p.y.y.x.Unmarshal(m[8*numBytes:])
	e.p.y.y.y.Unmarshal(m[9*numBytes:])
	e.p.y.z.x.Unmarshal(m[10*numBytes:])
	e.p.y.z.y.Unmarshal(m[11*numBytes:])
	montEncode(&e.p.x.x.x, &e.p.x.x.x)
	montEncode(&e.p.x.x.y, &e.p.x.x.y)
	montEncode(&e.p.x.y.x, &e.p.x.y.x)
	montEncode(&e.p.x.y.y, &e.p.x.y.y)
	montEncode(&e.p.x.z.x, &e.p.x.z.x)
	montEncode(&e.p.x.z.y, &e.p.x.z.y)
	montEncode(&e.p.y.x.x, &e.p.y.x.x)
	montEncode(&e.p.y.x.y, &e.p.y.x.y)
	montEncode(&e.p.y.y.x, &e.p.y.y.x)
	montEncode(&e.p.y.y.y, &e.p.y.y.y)
	montEncode(&e.p.y.z.x, &e.p.y.z.x)
	montEncode(&e.p.y.z.y, &e.p.y.z.y)

	return nil
}
