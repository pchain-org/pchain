package core

import (
	"encoding/binary"
	"unsafe"
)

func tx_b2s(bs []uint8) string {

	ba := []byte{}
	for _, b := range bs {
		ba = append(ba, byte(b))
	}
	return string(ba)
}

func tx_b2i(bs []uint8) uint64 {

	ba := []byte{0,0,0,0,0,0,0,0}
	for i, b := range bs {
		ba[i] = byte(b)
	}

	if tx_getEndian() {
		return binary.BigEndian.Uint64(ba)
	} else {
		return binary.LittleEndian.Uint64(ba)
	}
}

const INT_SIZE int = int(unsafe.Sizeof(0))

//true = big endian, false = little endian
func tx_getEndian() (ret bool) {
	var i int = 0x1
	bs := (*[INT_SIZE]byte)(unsafe.Pointer(&i))
	if bs[0] == 0 {
		return true
	} else {
		return false
	}
}
