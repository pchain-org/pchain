package bls

import (
	"math/big"

	"bls/bn256"
	"golang.org/x/crypto/sha3"
)

type HashPoint struct {
	val bn256.G1
}

func (hpt *HashPoint) Sum(msg []byte) *HashPoint {
	h := sha3.Sum256(msg)
	k := new(big.Int).SetBytes(h[:])
	hpt.val.ScalarBaseMult(k)
	return hpt
}

func (hpt *HashPoint) Aggregate(hpts ...*HashPoint) *HashPoint {
	hpt.val.Zero()
	for _, hptI := range hpts {
		hpt.val.Add(&hpt.val, &hptI.val)
	}
	return hpt
}

func (hpt *HashPoint) SumAggregate(msgs ...[]byte) *HashPoint {
	hps := make([]*HashPoint, len(msgs))
	for i, msg := range msgs {
		hps[i] = new(HashPoint).Sum(msg)
	}
	hpt.Aggregate(hps...)
	return hpt
}

func (hp *HashPoint) Marshal() []byte {
	return hp.val.Marshal()
}

func (hp *HashPoint) Unmarshal(in []byte) error {
	return hp.val.Unmarshal(in)
}
