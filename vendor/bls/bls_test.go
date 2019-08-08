package bls

import (
	"encoding/hex"
	"math/big"
	"math/rand"
	"testing"
	"time"
)

func TestBigIntEndian(t *testing.T) {
	orderBytes := []byte{
		0x8f, 0xb5, 0x01, 0xe3, 0x4a, 0xa3, 0x87, 0xf9,
		0xaa, 0x6f, 0xec, 0xb8, 0x61, 0x84, 0xdc, 0x21,
		0x2e, 0x8d, 0x8e, 0x12, 0xf8, 0x2b, 0x39, 0x24,
		0x1a, 0x2e, 0xf4, 0x5b, 0x57, 0xac, 0x72, 0x61,
	}
	order, _ := new(big.Int).SetString("65000549695646603732796438742359905742570406053903786389881062969044166799969", 10)
	if order.Cmp(new(big.Int).SetBytes(orderBytes)) != 0 {
		t.Fatal("BigInt is little-endian")
	}
	uBytes := []byte{
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x5a, 0x76, 0xae, 0x9a, 0xec, 0x58, 0x83, 0x01,
	}
	u, _ := new(big.Int).SetString("6518589491078791937", 10)
	if u.Cmp(new(big.Int).SetBytes(uBytes)) != 0 {
		t.Fatal("Error when setting BigInt with leading zero")
	}
}

func randMsg() []byte {
	len := rand.Intn(65536) + 1
	msg := make([]byte, len)
	for i := 0; i < len; i++ {
		msg[i] = byte(rand.Int())
	}
	return msg
}
func TestSign(t *testing.T) {
	rand.Seed(time.Now().UnixNano())
	key1 := GenerateKey()
	key2 := GenerateKey()

	msg := randMsg()

	sig := Sign(msg, key1.Private())

	if !Verify(sig, msg, key1.Public()) {
		t.Fatal("Verifying failed with correct key")
	}
	if Verify(sig, msg, key2.Public()) {
		t.Fatalf("Verifying passed with incorrect key %s %s",
			hex.EncodeToString(key1.Marshal()),
			hex.EncodeToString(key2.Marshal()),
		)
	}
	msg[0] = ^msg[0]
	if Verify(sig, msg, key1.Public()) {
		t.Fatal("Verifying passed with incorrect message")
	}
}

func TestNFor1(t *testing.T) {
	rand.Seed(time.Now().UnixNano())
	key1 := GenerateKey()
	key2 := GenerateKey()
	key3 := GenerateKey()
	key4 := GenerateKey()

	msg := randMsg()

	sig1 := Sign(msg, key1.Private())
	sig2 := Sign(msg, key2.Private())
	sig3 := Sign(msg, key3.Private())

	sig := new(Signature).Aggregate(sig1, sig2, sig3)

	if !VerifyNFor1(sig, msg, key1.Public(), key2.Public(), key3.Public()) {
		t.Fatal("Verifying failed with correct key set")
	}
	if VerifyNFor1(sig, msg, key1.Public(), key2.Public(), key4.Public()) {
		t.Fatal("Verifying passed with incorrect key set")
	}
	msg[0] = ^msg[0]
	if VerifyNFor1(sig, msg, key1.Public(), key2.Public(), key3.Public()) {
		t.Fatal("Verifying passed with incorrect message")
	}
}

func Test1ForN(t *testing.T) {
	rand.Seed(time.Now().UnixNano())
	key1 := GenerateKey()
	key2 := GenerateKey()

	msg1 := randMsg()
	msg2 := randMsg()
	msg3 := randMsg()
	msg4 := randMsg()

	sig1 := Sign(msg1, key1.Private())
	sig2 := Sign(msg2, key1.Private())
	sig3 := Sign(msg3, key1.Private())

	sig := new(Signature).Aggregate(sig1, sig2, sig3)

	if !Verify1ForN(sig, key1.Public(), msg1, msg2, msg3) {
		t.Fatal("Verifying failed with correct message set")
	}
	if Verify1ForN(sig, key1.Public(), msg1, msg2, msg4) {
		t.Fatal("Verifying passed with incorrect message set")

	}
	if Verify1ForN(sig, key2.Public(), msg1, msg2, msg3) {
		t.Fatal("Verifying passed with incorrect key")
	}
}

func TestAggregate(t *testing.T) {
	rand.Seed(time.Now().UnixNano())
	key1 := GenerateKey()
	key2 := GenerateKey()
	key3 := GenerateKey()
	key4 := GenerateKey()

	msg1 := randMsg()
	msg2 := randMsg()
	msg3 := randMsg()
	msg4 := randMsg()

	sig1 := Sign(msg1, key1.Private())
	sig2 := Sign(msg2, key2.Private())
	sig3 := Sign(msg3, key3.Private())
	sig4 := Sign(msg4, key4.Private())

	if !VerifyGroupMessage(
		new(Signature).Aggregate(sig1, sig2, sig3),
		NewVerifiableMessage(msg1, key1.Public()),
		NewVerifiableMessage(msg2, key2.Public()),
		NewVerifiableMessage(msg3, key3.Public()),
	) {
		t.Fatal("Verifying failed with correct key-message set")
	}
	if VerifyGroupMessage(
		new(Signature).Aggregate(sig1, sig2, sig3),
		NewVerifiableMessage(msg1, key1.Public()),
		NewVerifiableMessage(msg2, key2.Public()),
		NewVerifiableMessage(msg3, key4.Public()),
	) {
		t.Fatal("Verifying passed with incorrect key")
	}
	if VerifyGroupMessage(
		new(Signature).Aggregate(sig1, sig2, sig3),
		NewVerifiableMessage(msg1, key1.Public()),
		NewVerifiableMessage(msg2, key2.Public()),
		NewVerifiableMessage(msg4, key3.Public()),
	) {
		t.Fatal("Verifying passed with incorrect message")
	}
	if VerifyGroupMessage(
		new(Signature).Aggregate(sig1, sig2, sig4),
		NewVerifiableMessage(msg1, key1.Public()),
		NewVerifiableMessage(msg2, key2.Public()),
		NewVerifiableMessage(msg3, key3.Public()),
	) {
		t.Fatal("Verifying passed with incorrect key-message set")
	}
}
