package bls

import (
	"bls/bn256"
	"bytes"
)

func Sign(privkey *PrivateKey, msg []byte) *Signature {
	hp := new(HashPoint).Sum(msg)
	return &Signature{
		point: new(bn256.G1).ScalarMult(hp.point, privkey.scalar),
	}
}

func Verify(sig *Signature, msg []byte, pubkey *PublicKey) bool {
	hp := new(HashPoint).Sum(msg)
	e1 := bn256.Pair(hp.point, pubkey.point)
	e2 := bn256.Pair(sig.point, new(bn256.G2).Base())
	return bytes.Equal(e1.Marshal(), e2.Marshal())
}

func VerifyNFor1(sig *Signature, msg []byte, pks ...*PublicKey) bool {
	hp := new(HashPoint).Sum(msg)
	aggpk := new(PublicKey).Aggregate(pks...)
	e1 := bn256.Pair(hp.point, aggpk.point)
	e2 := bn256.Pair(sig.point, new(bn256.G2).Base())
	return bytes.Equal(e1.Marshal(), e2.Marshal())
}

func Verify1ForN(sig *Signature, pk *PublicKey, msgs ...[]byte) bool {
	agghp := new(HashPoint).SumAggregate(msgs...)
	e1 := bn256.Pair(agghp.point, pk.point)
	e2 := bn256.Pair(sig.point, new(bn256.G2).Base())
	return bytes.Equal(e1.Marshal(), e2.Marshal())
}

type MemberMessage struct {
	pubkey *PublicKey
	hash   *HashPoint
}

func NewMemberMessage(msg []byte, pubkey *PublicKey) *MemberMessage {
	return &MemberMessage{
		pubkey: pubkey.Copy(),
		hash:   new(HashPoint).Sum(msg),
	}
}

func VerifyAggregate(sig *Signature, mmsgs ...*MemberMessage) bool {
	e1 := new(bn256.GT).Unit()
	for _, mmsg := range mmsgs {
		e := bn256.Pair(mmsg.hash.point, mmsg.pubkey.point)
		e1.Add(e1, e)
	}
	e2 := bn256.Pair(sig.point, new(bn256.G2).Base())
	return bytes.Equal(e1.Marshal(), e2.Marshal())
}
