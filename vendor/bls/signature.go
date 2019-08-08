package bls

import (
	"bls/bn256"
)

type Signature struct {
	val bn256.G1
}

func (sig *Signature) Sign(hpt *HashPoint, pvtk *PrivateKey) *Signature {
	sig.val.ScalarMult(&hpt.val, pvtk.val)
	return sig
}

func (sig *Signature) Aggregate(sigs ...*Signature) *Signature {
	sig.val.Zero()
	for _, sigI := range sigs {
		sig.val.Add(&sig.val, &sigI.val)
	}
	return sig
}

func (sig *Signature) Marshal() []byte {
	return sig.val.Marshal()
}

func (sig *Signature) Unmarshal(in []byte) error {
	return sig.val.Unmarshal(in)
}
