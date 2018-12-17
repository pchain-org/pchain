package bls

import (
	"bytes"

	"bls/bn256"
)

func Sign(msg []byte, pvtk *PrivateKey) *Signature {
	hpt := new(HashPoint).Sum(msg)
	return new(Signature).Sign(hpt, pvtk)
}

var g2 = new(bn256.G2).Base()

func Verify(sig *Signature, msg []byte, pubk *PublicKey) bool {
	hpt := new(HashPoint).Sum(msg)
	e1 := bn256.Pair(&hpt.val, &pubk.val)
	e2 := bn256.Pair(&sig.val, g2)
	return bytes.Equal(e1.Marshal(), e2.Marshal())
}

func VerifyNFor1(sig *Signature, msg []byte, pubks ...*PublicKey) bool {
	hpt := new(HashPoint).Sum(msg)
	apubk := new(PublicKey).Aggregate(pubks...)
	e1 := bn256.Pair(&hpt.val, &apubk.val)
	e2 := bn256.Pair(&sig.val, g2)
	return bytes.Equal(e1.Marshal(), e2.Marshal())
}

func Verify1ForN(sig *Signature, pubk *PublicKey, msgs ...[]byte) bool {
	ahpt := new(HashPoint).SumAggregate(msgs...)
	e1 := bn256.Pair(&ahpt.val, &pubk.val)
	e2 := bn256.Pair(&sig.val, g2)
	return bytes.Equal(e1.Marshal(), e2.Marshal())
}

type VerifiableMessage struct {
	msghash HashPoint
	member  PublicKey
}

func NewVerifiableMessage(msg []byte, pubk *PublicKey) *VerifiableMessage {
	ret := new(VerifiableMessage)
	ret.msghash.Sum(msg)
	ret.member.Unmarshal(pubk.Marshal())
	return ret
}

func VerifyGroupMessage(sig *Signature, msgs ...*VerifiableMessage) bool {
	e1 := new(bn256.GT).Unit()
	for _, msg := range msgs {
		e := bn256.Pair(&msg.msghash.val, &msg.member.val)
		e1.Add(e1, e)
	}
	e2 := bn256.Pair(&sig.val, g2)
	return bytes.Equal(e1.Marshal(), e2.Marshal())
}
