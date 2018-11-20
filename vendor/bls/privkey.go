package bls

import "bls/bn256"

type PrivateKey struct {
	scalar *bn256.Scalar
}

func (pk *PrivateKey) IsNull() bool {
	if pk.scalar == nil {
		return true
	}
	return pk.scalar.IsZero()
}

func (pk *PrivateKey) Copy() *PrivateKey {
	ret := new(PrivateKey)
	ret.Unmarshal(pk.Marshal())
	return ret
}

func (pk *PrivateKey) Public() *PublicKey {
	if pk.scalar == nil {
		pk.scalar = &bn256.Scalar{}
	}
	return &PublicKey{
		point: new(bn256.G2).ScalarBaseMult(pk.scalar),
	}
}

func (pk *PrivateKey) Marshal() []byte {
	if pk.scalar == nil {
		pk.scalar = &bn256.Scalar{}
	}
	return pk.scalar.Marshal()
}

func (pk *PrivateKey) Unmarshal(in []byte) error {
	if pk.scalar == nil {
		pk.scalar = &bn256.Scalar{}
	}
	return pk.scalar.Unmarshal(in)
}
