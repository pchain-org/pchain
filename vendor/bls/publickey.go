package bls

import (
	"math/big"

	"bls/bn256"
)

type PublicKey struct {
	val bn256.G2
}

func Private2Public(pvtk *PrivateKey) *PublicKey {
	if pvtk.val == nil {
		pvtk.val = new(big.Int).SetUint64(0)
	}
	pubk := new(PublicKey)
	pubk.val.ScalarBaseMult(pvtk.val)
	return pubk
}

func (pubk *PublicKey) Null() *PublicKey {
	pubk.val.Zero()
	return pubk
}

func (pubk *PublicKey) IsNull() bool {
	return pubk.val.IsZero()
}

func (pubk *PublicKey) Copy() *PublicKey {
	ret := new(PublicKey)
	ret.Unmarshal(pubk.Marshal())
	return ret
}

func (pubk *PublicKey) Aggregate(pubks ...*PublicKey) *PublicKey {
	pubk.val.Zero()
	for _, pubkI := range pubks {
		pubk.val.Add(&pubk.val, &pubkI.val)
	}
	return pubk
}

func (pubk *PublicKey) Marshal() []byte {
	return pubk.val.Marshal()
}

func (pubk *PublicKey) Unmarshal(in []byte) error {
	return pubk.val.Unmarshal(in)
}
