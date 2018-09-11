package bls

import (
	"encoding/hex"
	"math/rand"
	"testing"
	"time"
	"bytes"
	"fmt"
	"bls/bn256"
)

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

	pk1 := key1.private.Public()
	pk2 := key1.private.Public()
	fmt.Println("g:", new(bn256.G2).Marshal())
	fmt.Println("g:", new(bn256.G2).Marshal())
	if !bytes.Equal(pk1.Marshal(), pk2.Marshal()) {
		t.Fatal("Verifying failed with correct key")
	}

	msg := randMsg()

	sig := Sign(key1.Private(), msg)

	if !Verify(sig, msg, pk1) {
		t.Fatal("Verifying failed with correct key")
	}
	if !Verify(sig, msg, pk2) {
		t.Fatal("Verifying failed with correct key")
	}
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

	sig1 := Sign(key1.Private(), msg)
	sig2 := Sign(key2.Private(), msg)
	sig3 := Sign(key3.Private(), msg)

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

	sig1 := Sign(key1.Private(), msg1)
	sig2 := Sign(key1.Private(), msg2)
	sig3 := Sign(key1.Private(), msg3)

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

	sig1 := Sign(key1.Private(), msg1)
	sig2 := Sign(key2.Private(), msg2)
	sig3 := Sign(key3.Private(), msg3)
	sig4 := Sign(key4.Private(), msg4)

	if !VerifyAggregate(
		new(Signature).Aggregate(sig1, sig2, sig3),
		NewMemberMessage(msg1, key1.Public()),
		NewMemberMessage(msg2, key2.Public()),
		NewMemberMessage(msg3, key3.Public()),
	) {
		t.Fatal("Verifying failed with correct key-message set")
	}
	if VerifyAggregate(
		new(Signature).Aggregate(sig1, sig2, sig3),
		NewMemberMessage(msg1, key1.Public()),
		NewMemberMessage(msg2, key2.Public()),
		NewMemberMessage(msg3, key4.Public()),
	) {
		t.Fatal("Verifying passed with incorrect key")
	}
	if VerifyAggregate(
		new(Signature).Aggregate(sig1, sig2, sig3),
		NewMemberMessage(msg1, key1.Public()),
		NewMemberMessage(msg2, key2.Public()),
		NewMemberMessage(msg4, key3.Public()),
	) {
		t.Fatal("Verifying passed with incorrect message")
	}
	if VerifyAggregate(
		new(Signature).Aggregate(sig1, sig2, sig4),
		NewMemberMessage(msg1, key1.Public()),
		NewMemberMessage(msg2, key2.Public()),
		NewMemberMessage(msg3, key3.Public()),
	) {
		t.Fatal("Verifying passed with incorrect key-message set")
	}
}
