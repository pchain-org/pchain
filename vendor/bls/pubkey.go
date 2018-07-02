package bls

import "bls/bn256"

type PublicKey struct {
	point *bn256.G2
}

func (pk *PublicKey) IsNull() bool {
	if pk.point == nil {
		return true
	}
	return pk.point.IsZero()
}

func (pk *PublicKey) Copy() *PublicKey {
	ret := new(PublicKey)
	ret.Unmarshal(pk.Marshal())
	return ret
}

func (pk *PublicKey) Aggregate(pks ...*PublicKey) *PublicKey {
	pk.point = &bn256.G2{}
	for _, _pk := range pks {
		pk.point.Add(pk.point, _pk.point)
	}
	return pk
}

func (pk *PublicKey) AggregateArray(pks []*PublicKey) *PublicKey {
	pk.point = &bn256.G2{}
	for _, _pk := range pks {
		pk.point.Add(pk.point, _pk.point)
	}
	return pk
}
func (pk *PublicKey) Marshal() []byte {
	if pk.point == nil {
		pk.point = &bn256.G2{}
	}
	return pk.point.Marshal()
}

func (pk *PublicKey) Unmarshal(in []byte) error {
	if pk.point == nil {
		pk.point = &bn256.G2{}
	}
	return pk.point.Unmarshal(in)
}
