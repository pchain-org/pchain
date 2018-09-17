package bls

import (
	"bls/bn256"
	"bls/sha3"
)

type HashPoint struct {
	point *bn256.G1
}

func (hp *HashPoint) Sum(msg []byte) *HashPoint {
	sc := new(bn256.Scalar)
	h := sha3.Sum256(msg)
	sc.Unmarshal(h[:])
	hp.point = new(bn256.G1).ScalarBaseMult(sc)
	return hp
}

func (hp *HashPoint) Aggregate(hps ...*HashPoint) *HashPoint {
	hp.point = &bn256.G1{}
	for _, _hp := range hps {
		hp.point.Add(hp.point, _hp.point)
	}
	return hp
}

func (hp *HashPoint) SumAggregate(msgs ...[]byte) *HashPoint {
	hps := make([]*HashPoint, len(msgs))
	for i, msg := range msgs {
		hps[i] = new(HashPoint).Sum(msg)
	}
	hp.Aggregate(hps...)
	return hp
}

func (hp *HashPoint) Marshal() []byte {
	if hp.point == nil {
		hp.point = &bn256.G1{}
	}
	return hp.point.Marshal()
}

func (hp *HashPoint) Unmarshal(in []byte) error {
	if hp.point == nil {
		hp.point = &bn256.G1{}
	}
	return hp.point.Unmarshal(in)
}
