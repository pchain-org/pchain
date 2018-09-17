package crypto

import (
	"bytes"

	secp256k1 "github.com/btcsuite/btcd/btcec"
	"github.com/tendermint/ed25519"
	"github.com/tendermint/ed25519/extra25519"
	. "github.com/tendermint/go-common"
	"github.com/tendermint/go-data"
	"github.com/tendermint/go-wire"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	"bls"
)

// PrivKey is part of PrivAccount and state.PrivValidator.
type PrivKey interface {
	Bytes() []byte
	Sign(msg []byte) Signature
	PubKey() PubKey
	Equals(PrivKey) bool
}

// Types of implementations
const (
	TypeEd25519   = byte(0x01)
	TypeSecp256k1 = byte(0x02)
	TypeEthereum   = byte(0x03)
	TypeBls       = byte(0x04)
	NameEd25519   = "ed25519"
	NameSecp256k1 = "secp256k1"
	NameEthereum    = "ethereum"
	NameBls        = "bls"
)

var privKeyMapper data.Mapper

// register both private key types with go-data (and thus go-wire)
func init() {
	privKeyMapper = data.NewMapper(PrivKeyS{}).
		RegisterImplementation(PrivKeyEd25519{}, NameEd25519, TypeEd25519).
		RegisterImplementation(PrivKeySecp256k1{}, NameSecp256k1, TypeSecp256k1).
		RegisterImplementation(EthereumPrivKey{}, NameEthereum, TypeEthereum).
		RegisterImplementation(BLSPrivKey{}, NameBls, TypeBls)

}

// PrivKeyS add json serialization to PrivKey
type PrivKeyS struct {
	PrivKey
}

func WrapPrivKey(pk PrivKey) PrivKeyS {
	for ppk, ok := pk.(PrivKeyS); ok; ppk, ok = pk.(PrivKeyS) {
		pk = ppk.PrivKey
	}
	return PrivKeyS{pk}
}

func (p PrivKeyS) MarshalJSON() ([]byte, error) {
	return privKeyMapper.ToJSON(p.PrivKey)
}

func (p *PrivKeyS) UnmarshalJSON(data []byte) (err error) {
	parsed, err := privKeyMapper.FromJSON(data)
	if err == nil && parsed != nil {
		p.PrivKey = parsed.(PrivKey)
	}
	return
}

func (p PrivKeyS) Empty() bool {
	return p.PrivKey == nil
}

func PrivKeyFromBytes(privKeyBytes []byte) (privKey PrivKey, err error) {
	err = wire.ReadBinaryBytes(privKeyBytes, &privKey)
	return
}

//-------------------------------------

// Implements PrivKey
type PrivKeyEd25519 [64]byte

func (privKey PrivKeyEd25519) Bytes() []byte {
	return wire.BinaryBytes(struct{ PrivKey }{privKey})
}

func (privKey PrivKeyEd25519) Sign(msg []byte) Signature {
	privKeyBytes := [64]byte(privKey)
	signatureBytes := ed25519.Sign(&privKeyBytes, msg)
	return SignatureEd25519(*signatureBytes)
}

func (privKey PrivKeyEd25519) PubKey() PubKey {
	privKeyBytes := [64]byte(privKey)
	return PubKeyEd25519(*ed25519.MakePublicKey(&privKeyBytes))
}

func (privKey PrivKeyEd25519) Equals(other PrivKey) bool {
	if otherEd, ok := other.(PrivKeyEd25519); ok {
		return bytes.Equal(privKey[:], otherEd[:])
	} else {
		return false
	}
}

func (p PrivKeyEd25519) MarshalJSON() ([]byte, error) {
	return data.Encoder.Marshal(p[:])
}

func (p *PrivKeyEd25519) UnmarshalJSON(enc []byte) error {
	var ref []byte
	err := data.Encoder.Unmarshal(&ref, enc)
	copy(p[:], ref)
	return err
}

func (privKey PrivKeyEd25519) ToCurve25519() *[32]byte {
	keyCurve25519 := new([32]byte)
	privKeyBytes := [64]byte(privKey)
	extra25519.PrivateKeyToCurve25519(keyCurve25519, &privKeyBytes)
	return keyCurve25519
}

func (privKey PrivKeyEd25519) String() string {
	return Fmt("PrivKeyEd25519{*****}")
}

// Deterministically generates new priv-key bytes from key.
func (privKey PrivKeyEd25519) Generate(index int) PrivKeyEd25519 {
	newBytes := wire.BinarySha256(struct {
		PrivKey [64]byte
		Index   int
	}{privKey, index})
	var newKey [64]byte
	copy(newKey[:], newBytes)
	return PrivKeyEd25519(newKey)
}

func GenPrivKeyEd25519() PrivKeyEd25519 {
	privKeyBytes := new([64]byte)
	copy(privKeyBytes[:32], CRandBytes(32))
	ed25519.MakePublicKey(privKeyBytes)
	return PrivKeyEd25519(*privKeyBytes)
}

// NOTE: secret should be the output of a KDF like bcrypt,
// if it's derived from user input.
func GenPrivKeyEd25519FromSecret(secret []byte) PrivKeyEd25519 {
	privKey32 := Sha256(secret) // Not Ripemd160 because we want 32 bytes.
	privKeyBytes := new([64]byte)
	copy(privKeyBytes[:32], privKey32)
	ed25519.MakePublicKey(privKeyBytes)
	return PrivKeyEd25519(*privKeyBytes)
}

//-------------------------------------

// Implements PrivKey
type PrivKeySecp256k1 [32]byte

func (privKey PrivKeySecp256k1) Bytes() []byte {
	return wire.BinaryBytes(struct{ PrivKey }{privKey})
}

func (privKey PrivKeySecp256k1) Sign(msg []byte) Signature {
	priv__, _ := secp256k1.PrivKeyFromBytes(secp256k1.S256(), privKey[:])
	sig__, err := priv__.Sign(Sha256(msg))
	if err != nil {
		PanicSanity(err)
	}
	return SignatureSecp256k1(sig__.Serialize())
}

func (privKey PrivKeySecp256k1) PubKey() PubKey {
	_, pub__ := secp256k1.PrivKeyFromBytes(secp256k1.S256(), privKey[:])
	var pub PubKeySecp256k1
	copy(pub[:], pub__.SerializeCompressed())
	return pub
}

func (privKey PrivKeySecp256k1) Equals(other PrivKey) bool {
	if otherSecp, ok := other.(PrivKeySecp256k1); ok {
		return bytes.Equal(privKey[:], otherSecp[:])
	} else {
		return false
	}
}

func (p PrivKeySecp256k1) MarshalJSON() ([]byte, error) {
	return data.Encoder.Marshal(p[:])
}

func (p *PrivKeySecp256k1) UnmarshalJSON(enc []byte) error {
	var ref []byte
	err := data.Encoder.Unmarshal(&ref, enc)
	copy(p[:], ref)
	return err
}

func (privKey PrivKeySecp256k1) String() string {
	return Fmt("PrivKeySecp256k1{*****}")
}


type EthereumPrivKey []byte

func (privKey EthereumPrivKey) Bytes() []byte {
	return wire.BinaryBytes(struct{ PrivKey }{privKey})
}

func (privKey EthereumPrivKey) Sign(msg []byte) Signature {
	priv,err := ethcrypto.ToECDSA(privKey)
	msg = ethcrypto.Keccak256(msg)
	sig, err := ethcrypto.Sign(msg, priv)
	if err != nil {
		return nil
	}
	return EthereumSignature(sig)
}

func (privKey EthereumPrivKey) PubKey() PubKey {
	priv,err := ethcrypto.ToECDSA(privKey)
	pubKey := ethcrypto.FromECDSAPub(&priv.PublicKey)
	if err != nil {
		return nil
	}
	return EthereumPubKey(pubKey)
}

func (privKey EthereumPrivKey) Equals(other PrivKey) bool {
	if otherEd, ok := other.(EthereumPrivKey); ok {
		return bytes.Equal(privKey[:], otherEd[:])
	} else {
		return false
	}
}


func (privKey EthereumPrivKey) MarshalJSON() ([]byte, error) {
	return data.Encoder.Marshal(privKey[:])
}


func (privKey *EthereumPrivKey) UnmarshalJSON(enc []byte) error {
	var ref []byte
	err := data.Encoder.Unmarshal(&ref, enc)
	copy((*privKey)[:], ref)
	return err
}

/*
// Deterministically generates new priv-key bytes from key.
func (key PrivKeySecp256k1) Generate(index int) PrivKeySecp256k1 {
	newBytes := wire.BinarySha256(struct {
		PrivKey [64]byte
		Index   int
	}{key, index})
	var newKey [64]byte
	copy(newKey[:], newBytes)
	return PrivKeySecp256k1(newKey)
}
*/

func GenPrivKeySecp256k1() PrivKeySecp256k1 {
	privKeyBytes := [32]byte{}
	copy(privKeyBytes[:], CRandBytes(32))
	priv, _ := secp256k1.PrivKeyFromBytes(secp256k1.S256(), privKeyBytes[:])
	copy(privKeyBytes[:], priv.Serialize())
	return PrivKeySecp256k1(privKeyBytes)
}

// NOTE: secret should be the output of a KDF like bcrypt,
// if it's derived from user input.
func GenPrivKeySecp256k1FromSecret(secret []byte) PrivKeySecp256k1 {
	privKey32 := Sha256(secret) // Not Ripemd160 because we want 32 bytes.
	priv, _ := secp256k1.PrivKeyFromBytes(secp256k1.S256(), privKey32)
	privKeyBytes := [32]byte{}
	copy(privKeyBytes[:], priv.Serialize())
	return PrivKeySecp256k1(privKeyBytes)
}


//-------------------------------------
type BLSPrivKey []byte
func (privKey BLSPrivKey) Bytes() []byte {
	return privKey
}

func (privKey BLSPrivKey) getElement() *bls.PrivateKey {
	sk := &bls.PrivateKey{}
	err := sk.Unmarshal(privKey)
	if err != nil {
		return nil
	} else {
		return sk
	}
}

func (privKey BLSPrivKey) PubKey() PubKey {
	pub := privKey.getElement().Public()
	return BLSPubKey(pub.Marshal())
}

func (privKey BLSPrivKey) Sign(msg []byte) Signature {
	sk := privKey.getElement()
	sign := bls.Sign(sk, msg)
	return BLSSignature(sign.Marshal())
}

func (privKey BLSPrivKey) Equals(other PrivKey) bool {
	if otherSk,ok := other.(BLSPrivKey); ok {
		return bytes.Equal(privKey, otherSk)
	} else {
		return false
	}
}

func (privKey BLSPrivKey) MarshalJSON() ([]byte, error) {
	return data.Encoder.Marshal(privKey)
}

func (privKey *BLSPrivKey) UnmarshalJSON(enc []byte) error {
	var ref []byte
	err := data.Encoder.Unmarshal(&ref, enc)
	copy(*privKey, ref)
	return err
}
