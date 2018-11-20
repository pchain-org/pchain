package bn256

import "errors"

type Scalar [4]uint64

func (sc *Scalar) Zero() *Scalar {
	sc[0], sc[1], sc[2], sc[3] = 0, 0, 0, 0
	return sc
}

func (sc *Scalar) IsZero() bool {
	return sc[0] == 0 && sc[1] == 0 && sc[2] == 0 && sc[3] == 0
}

func (sc *Scalar) Bit(pos int) uint64 {
	return (sc[uint(pos)>>6] >> (uint(pos) & 0x3F)) & 0x1
}

func (sc *Scalar) Marshal() []byte {
	out := make([]byte, 32)
	for w := uint(0); w < 4; w++ {
		for b := uint(0); b < 8; b++ {
			out[8*w+b] = byte(sc[3-w] >> (56 - 8*b))
		}
	}
	return out
}

func (sc *Scalar) Unmarshal(in []byte) error {
	if len(in) != 32 {
		return errors.New("bn256.Scalar: not enough data")
	}
	sc[0], sc[1], sc[2], sc[3] = 0, 0, 0, 0
	for w := uint(0); w < 4; w++ {
		for b := uint(0); b < 8; b++ {
			sc[3-w] |= uint64(in[8*w+b]) << (56 - 8*b)
		}
	}
	return nil
}
