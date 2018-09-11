package bls

import "bls/bn256"

type Signature struct {
	point *bn256.G1
}

func NewSignature() *Signature {
	return &Signature{
		point: &bn256.G1{},
	}
}

func (sig *Signature) Aggregate(sigs ...*Signature) *Signature {
	sig.point = &bn256.G1{}
	for _, _sig := range sigs {
		sig.point.Add(sig.point, _sig.point)
	}
	return sig
}

func (sig *Signature) AggregateArray(sigs []*Signature) *Signature {
	sig.point = &bn256.G1{}
	for _, _sig := range sigs {
		sig.point.Add(sig.point, _sig.point)
	}
	return sig
}

func (sig *Signature) Marshal() []byte {
	if sig.point == nil {
		sig.point = &bn256.G1{}
	}
	return sig.point.Marshal()
}

func (sig *Signature) Unmarshal(in []byte) error {
	if sig.point == nil {
		sig.point = &bn256.G1{}
	}
	return sig.point.Unmarshal(in)
}
