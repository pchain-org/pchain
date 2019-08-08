package bls

import (
	"errors"
	"math/big"
)

type PrivateKey struct {
	val *big.Int
}

func (pvtk *PrivateKey) Null() *PrivateKey {
	pvtk.val = new(big.Int).SetUint64(0)
	return pvtk
}

func (pvtk *PrivateKey) IsNull() bool {
	if pvtk.val == nil {
		return true
	}
	return pvtk.val.IsUint64() && pvtk.val.Uint64() == 0
}

func (pvtk *PrivateKey) Copy() *PrivateKey {
	if pvtk.val == nil {
		pvtk.val = new(big.Int).SetUint64(0)
	}
	ret := new(PrivateKey)
	ret.Unmarshal(pvtk.Marshal())
	return ret
}

func (pvtk *PrivateKey) Public() *PublicKey {
	return Private2Public(pvtk)
}

func (pvtk *PrivateKey) Marshal() []byte {
	if pvtk.val == nil {
		pvtk.val = new(big.Int).SetUint64(0)
	}
	ret := make([]byte, 32)
	copy(ret[32-len(pvtk.val.Bytes()):], pvtk.val.Bytes())
	return ret
}

func (pvtk *PrivateKey) Unmarshal(in []byte) error {
	if len(in) != 32 {
		return errors.New("incorrect data length")
	}
	pvtk.val = new(big.Int).SetBytes(in)
	return nil
}
