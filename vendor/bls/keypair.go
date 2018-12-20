package bls

import (
	"crypto/rand"
	"errors"
	"math/big"

	"golang.org/x/crypto/sha3"
)

var maxUint256 = new(big.Int).Exp(big.NewInt(2), big.NewInt(256), nil)

func randBytes32() []byte {
	x, _ := rand.Int(rand.Reader, maxUint256)
	buf := make([]byte, 32)
	copy(buf[32-len(x.Bytes()):], x.Bytes())
	return buf
}

type KeyPair struct {
	private *PrivateKey
	public  *PublicKey
}

func GenerateKey() *KeyPair {
	kp := new(KeyPair)
	kp.private = new(PrivateKey)
	kp.private.Unmarshal(randBytes32())
	kp.public = Private2Public(kp.private)
	return kp
}

func KeyFromSeed(seed []byte) *KeyPair {
	kp := new(KeyPair)
	kp.private = new(PrivateKey)
	h := sha3.Sum256(seed)
	kp.private.Unmarshal(h[:])
	kp.public = Private2Public(kp.private)
	return kp
}

func (kp *KeyPair) fixPrivate() {
	kp.private = new(PrivateKey).Null()
}

func (kp *KeyPair) Private() *PrivateKey {
	if kp.private == nil {
		kp.fixPrivate()
	}
	ret := new(PrivateKey)
	ret.Unmarshal(kp.private.Marshal())
	return ret
}

func (kp *KeyPair) MarshalPrivate() []byte {
	if kp.private == nil {
		kp.fixPrivate()
	}
	return kp.private.Marshal()
}

func (kp *KeyPair) UnmarshalPrivate(in []byte) error {
	if kp.private == nil {
		kp.private = new(PrivateKey)
	}
	if err := kp.private.Unmarshal(in); err != nil {
		return err
	}
	kp.public = Private2Public(kp.private)
	return nil
}

func (kp *KeyPair) fixPublic() {
	kp.public = new(PublicKey).Null()
	if kp.private == nil {
		kp.fixPrivate()
	}
	if !kp.private.IsNull() {
		kp.public = Private2Public(kp.private)
	}
}

func (kp *KeyPair) Public() *PublicKey {
	if kp.public == nil {
		kp.fixPublic()
	}
	ret := new(PublicKey)
	ret.Unmarshal(kp.public.Marshal())
	return ret
}

func (kp *KeyPair) MarshalPublic() []byte {
	if kp.public == nil {
		kp.fixPublic()
	}
	return kp.public.Marshal()
}

func (kp *KeyPair) UnmarshalPublic(in []byte) error {
	kp.private = new(PrivateKey).Null()
	if kp.public == nil {
		kp.public = new(PublicKey)
	}
	return kp.public.Unmarshal(in)
}

func (kp *KeyPair) Marshal() []byte {
	if kp.private == nil {
		kp.fixPrivate()
	}
	if kp.private.IsNull() {
		return kp.MarshalPublic()
	}
	return kp.MarshalPrivate()
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
