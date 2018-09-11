package bls

import (
	"crypto/rand"
	"errors"
	"math/big"

	"bls/bn256"
	"bls/sha3"
)

var maxUint64 = new(big.Int).Exp(big.NewInt(2), big.NewInt(64), nil)

func randUint64() uint64 {
	x, _ := rand.Int(rand.Reader, maxUint64)
	return x.Uint64()
}

type KeyPair struct {
	private *PrivateKey
	public  *PublicKey
}

func GenerateKey() *KeyPair {
	key := new(KeyPair)
	n0, n1, n2, n3 := randUint64(), randUint64(), randUint64(), randUint64()
	key.private = &PrivateKey{
		scalar: &bn256.Scalar{n0, n1, n2, n3},
	}
	key.public = key.private.Public()
	return key
}

func KeyFromSeed(seed []byte) *KeyPair {
	key := new(KeyPair)
	h := sha3.Sum256(seed)
	key.private = new(PrivateKey)
	key.private.Unmarshal(h[:])
	key.public = key.private.Public()
	return key
}

func (kp *KeyPair) Private() *PrivateKey {
	if kp.private == nil {
		kp.private = new(PrivateKey)
	}
	ret := new(PrivateKey)
	ret.Unmarshal(kp.private.Marshal())
	return ret
}

func (kp *KeyPair) Public() *PublicKey {
	if kp.public == nil {
		kp.public = new(PublicKey)
	}
	ret := new(PublicKey)
	ret.Unmarshal(kp.public.Marshal())
	return ret
}

func (kp *KeyPair) MarshalPrivate() []byte {
	if kp.private == nil {
		kp.private = new(PrivateKey)
	}
	return kp.private.Marshal()
}

func (kp *KeyPair) MarshalPublic() []byte {
	if kp.public == nil {
		kp.public = new(PublicKey)
	}
	return kp.public.Marshal()
}

func (kp *KeyPair) Marshal() []byte {
	if kp.private == nil {
		kp.private = new(PrivateKey)
	}
	if kp.private.IsNull() {
		return kp.MarshalPublic()
	}
	return kp.MarshalPrivate()
}

func (kp *KeyPair) UnmarshalPrivate(in []byte) error {
	if kp.private == nil {
		kp.private = new(PrivateKey)
	}
	if err := kp.private.Unmarshal(in); err != nil {
		return err
	}
	kp.public = kp.private.Public()
	return nil
}

func (kp *KeyPair) UnmarshalPublic(in []byte) error {
	if kp.public == nil {
		kp.public = new(PublicKey)
	}
	kp.private = new(PrivateKey)
	return kp.public.Unmarshal(in)
}

func (kp *KeyPair) Unmarshal(in []byte) error {
	switch len(in) {
	case 32:
		kp.UnmarshalPrivate(in)
	case 128:
		kp.UnmarshalPublic(in)
	default:
		return errors.New("invalid length of key pair")
	}
	return nil
}
