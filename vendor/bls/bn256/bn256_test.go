package bn256

import (
	"io"
	"math/big"
	"testing"

	"bytes"
	"crypto/rand"

	"golang.org/x/crypto/bn256"
)

func bigFromBase10(s string) *big.Int {
	n, _ := new(big.Int).SetString(s, 10)
	return n
}

var biOrder = bigFromBase10("65000549695646603732796438742359905742570406053903786389881062969044166799969")

func randomK(r io.Reader) (k *big.Int, err error) {
	for {
		k, err = rand.Int(r, biOrder)
		if k.Sign() > 0 || err != nil {
			return
		}
	}

	return
}

func bi2sc(k *big.Int) *Scalar {
	buf := make([]byte, 32)
	copy(buf[32-len(k.Bytes()):], k.Bytes())
	ret := &Scalar{}
	ret.Unmarshal(buf)
	return ret
}

// RandomG1 returns x and g₁ˣ where x is a random, non-zero number read from r.
func RandomG1(r io.Reader) (*big.Int, *G1, error) {
	k, err := randomK(r)
	if err != nil {
		return nil, nil, err
	}

	return k, new(G1).ScalarBaseMult(bi2sc(k)), nil
}

// RandomG2 returns x and g₂ˣ where x is a random, non-zero number read from r.
func RandomG2(r io.Reader) (*big.Int, *G2, error) {
	k, err := randomK(r)
	if err != nil {
		return nil, nil, err
	}

	return k, new(G2).ScalarBaseMult(bi2sc(k)), nil
}

// RandomGT returns x and e(g₁, g₂)ˣ where x is a random, non-zero number read
// from r.
func RandomGT(r io.Reader) (*big.Int, *GT, error) {
	k, err := randomK(r)
	if err != nil {
		return nil, nil, err
	}

	return k, new(GT).ScalarBaseMult(bi2sc(k)), nil
}

func TestG1(t *testing.T) {
	k, Ga, err := RandomG1(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	ma := Ga.Marshal()

	Gb := new(bn256.G1).ScalarBaseMult(k)
	mb := Gb.Marshal()

	if !bytes.Equal(ma, mb) {
		t.Fatal("bytes are different")
	}
}

func TestG1Marshal(t *testing.T) {
	_, Ga, err := RandomG1(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	ma := Ga.Marshal()

	Gb := new(G1)
	err = Gb.Unmarshal(ma)
	if err != nil {
		t.Fatal(err)
	}
	mb := Gb.Marshal()

	if !bytes.Equal(ma, mb) {
		t.Fatal("bytes are different")
	}
}

func TestG2(t *testing.T) {
	k, Ga, err := RandomG2(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	ma := Ga.Marshal()

	Gb := new(bn256.G2).ScalarBaseMult(k)
	mb := Gb.Marshal()

	if !bytes.Equal(ma, mb) {
		t.Fatal("bytes are different")
	}
}

func TestG2Marshal(t *testing.T) {
	_, Ga, err := RandomG2(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	ma := Ga.Marshal()

	Gb := new(G2)
	err = Gb.Unmarshal(ma)
	if err != nil {
		t.Fatal(err)
	}
	mb := Gb.Marshal()

	if !bytes.Equal(ma, mb) {
		t.Fatal("bytes are different")
	}
}

func TestGT(t *testing.T) {
	k, Ga, err := RandomGT(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	ma := Ga.Marshal()

	Gb, ok := new(bn256.GT).Unmarshal((&GT{gfP12Gen}).Marshal())
	if !ok {
		t.Fatal("unmarshal not ok")
	}
	Gb.ScalarMult(Gb, k)
	mb := Gb.Marshal()

	if !bytes.Equal(ma, mb) {
		t.Fatal("bytes are different")
	}
}

func TestGTMarshal(t *testing.T) {
	_, Ga, err := RandomGT(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	ma := Ga.Marshal()

	Gb := new(GT)
	err = Gb.Unmarshal(ma)
	if err != nil {
		t.Fatal(err)
	}
	mb := Gb.Marshal()

	if !bytes.Equal(ma, mb) {
		t.Fatal("bytes are different")
	}
}

func TestBilinearity(t *testing.T) {
	for i := 0; i < 2; i++ {
		a, p1, _ := RandomG1(rand.Reader)
		b, p2, _ := RandomG2(rand.Reader)
		e1 := Pair(p1, p2)

		e2 := Pair(&G1{curveGen}, &G2{twistGen})
		e2.ScalarMult(e2, bi2sc(a))
		e2.ScalarMult(e2, bi2sc(b))

		if *e1.p != *e2.p {
			t.Fatalf("bad pairing result: %s", e1)
		}
	}
}

func TestTripartiteDiffieHellman(t *testing.T) {
	a, _ := rand.Int(rand.Reader, biOrder)
	b, _ := rand.Int(rand.Reader, biOrder)
	c, _ := rand.Int(rand.Reader, biOrder)

	pa, pb, pc := new(G1), new(G1), new(G1)
	qa, qb, qc := new(G2), new(G2), new(G2)

	pa.Unmarshal(new(G1).ScalarBaseMult(bi2sc(a)).Marshal())
	qa.Unmarshal(new(G2).ScalarBaseMult(bi2sc(a)).Marshal())
	pb.Unmarshal(new(G1).ScalarBaseMult(bi2sc(b)).Marshal())
	qb.Unmarshal(new(G2).ScalarBaseMult(bi2sc(b)).Marshal())
	pc.Unmarshal(new(G1).ScalarBaseMult(bi2sc(c)).Marshal())
	qc.Unmarshal(new(G2).ScalarBaseMult(bi2sc(c)).Marshal())

	k1 := Pair(pb, qc)
	k1.ScalarMult(k1, bi2sc(a))
	k1Bytes := k1.Marshal()

	k2 := Pair(pc, qa)
	k2.ScalarMult(k2, bi2sc(b))
	k2Bytes := k2.Marshal()

	k3 := Pair(pa, qb)
	k3.ScalarMult(k3, bi2sc(c))
	k3Bytes := k3.Marshal()

	if !bytes.Equal(k1Bytes, k2Bytes) || !bytes.Equal(k2Bytes, k3Bytes) {
		t.Errorf("keys didn't agree")
	}
}

func BenchmarkG1(b *testing.B) {
	x, _ := rand.Int(rand.Reader, biOrder)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		new(G1).ScalarBaseMult(bi2sc(x))
	}
}

func BenchmarkG2(b *testing.B) {
	x, _ := rand.Int(rand.Reader, biOrder)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		new(G2).ScalarBaseMult(bi2sc(x))
	}
}

func BenchmarkGT(b *testing.B) {
	x, _ := rand.Int(rand.Reader, biOrder)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		new(GT).ScalarBaseMult(bi2sc(x))
	}
}

func BenchmarkPairing(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Pair(&G1{curveGen}, &G2{twistGen})
	}
}

func addmod(a, b *big.Int) *big.Int {
	return new(big.Int).Mod(
		new(big.Int).Add(a, b),
		new(big.Int).SetBytes(order.Marshal()),
	)
}

func mulmod(a, b *big.Int) *big.Int {
	return new(big.Int).Mod(
		new(big.Int).Mul(a, b),
		new(big.Int).SetBytes(order.Marshal()),
	)
}

func testmulpair(t *testing.T) {
	h, a, _ := RandomG1(rand.Reader)
	x, b, _ := RandomG2(rand.Reader)

	hx := mulmod(h, x)
	e1 := Pair(a, b)
	e2 := Pair(new(G1).ScalarBaseMult(bi2sc(hx)), new(G2).Base())
	e3 := new(GT).ScalarBaseMult(bi2sc(hx))

	if !bytes.Equal(e1.Marshal(), e2.Marshal()) {
		t.Fatal("bytes are different")
	}
	if !bytes.Equal(e1.Marshal(), e3.Marshal()) {
		t.Fatal("bytes are different")
	}
}

func testaddpair(t *testing.T) {
	x1, a1, _ := RandomG1(rand.Reader)
	x2, a2, _ := RandomG1(rand.Reader)

	x := addmod(x1, x2)

	e1 := new(GT).Add(
		Pair(a1, new(G2).Base()),
		Pair(a2, new(G2).Base()),
	)
	e2 := new(GT).ScalarBaseMult(bi2sc(x))

	if !bytes.Equal(e1.Marshal(), e2.Marshal()) {
		t.Fatal("bytes are different")
	}
}

func testaddmulpair(t *testing.T) {
	h1, a1, _ := RandomG1(rand.Reader)
	h2, a2, _ := RandomG1(rand.Reader)
	x1, b1, _ := RandomG2(rand.Reader)
	x2, b2, _ := RandomG2(rand.Reader)

	hx := addmod(mulmod(h1, x1), mulmod(h2, x2))
	e1 := new(GT).ScalarBaseMult(bi2sc(hx))

	e2 := Pair(
		new(G1).Add(
			new(G1).ScalarMult(a1, bi2sc(x1)),
			new(G1).ScalarMult(a2, bi2sc(x2)),
		),
		new(G2).Base(),
	)

	e3 := new(GT).Add(
		Pair(a1, b1),
		Pair(a2, b2),
	)

	if !bytes.Equal(e1.Marshal(), e2.Marshal()) {
		t.Fatal("bytes are different")
	}
	if !bytes.Equal(e1.Marshal(), e3.Marshal()) {
		t.Fatal("bytes are different")
	}
}

func TestPair(t *testing.T) {
	testmulpair(t)
	testaddpair(t)
	testaddmulpair(t)
}

func TestGTAdd(t *testing.T) {
	k1, e1, _ := RandomGT(rand.Reader)
	k2, e2, _ := RandomGT(rand.Reader)

	k := addmod(k1, k2)

	Ga := new(GT).Add(e1, e2)
	Gb := new(GT).ScalarBaseMult(bi2sc(k))

	Gc := Pair(
		new(G1).Add(
			new(G1).ScalarBaseMult(bi2sc(k1)),
			new(G1).ScalarBaseMult(bi2sc(k2)),
		),
		new(G2).Base(),
	)
	/*
		Gb1 := Pair(new(G1).ScalarBaseMult(bi2sc(k1)), new(G2).ScalarBaseMult(one))
		Gb2 := Pair(new(G1).ScalarBaseMult(bi2sc(k2)), new(G2).ScalarBaseMult(one))
		Gb := new(GT).Add(Gb1, Gb2)
	*/
	if !bytes.Equal(Ga.Marshal(), Gb.Marshal()) {
		t.Fatal("bytes are different")
	}
	if !bytes.Equal(Ga.Marshal(), Gc.Marshal()) {
		t.Fatal("bytes are different")
	}
}
