package types

import (
	"math/big"
	cmn "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
)
const (
	Length160 = 20
	Length256 = 32
)

type Hash160 [Length160]byte
type Hash256 [Length256]byte

var (
	EMPTY_HASH160 = Hash160{}
	EMPTY_HASH256 = Hash256{}
)

/////////// Hash160
func BytesToHash160(b []byte) Hash160 {
	var a Hash160
	a.SetBytes(b)
	return a
}
func StringToHash160(s string) Hash160 { return BytesToHash160([]byte(s)) }
func BigToHash160(b *big.Int) Hash160  { return BytesToHash160(b.Bytes()) }
func HexToHash160(s string) Hash160    { return BytesToHash160(cmn.FromHex(s)) }

// IsHexAddress verifies whether a string can represent a valid hex-encoded
// Ethereum address or not.
func IsHexHash160(s string) bool {
	if len(s) == 2+2*Length160 && cmn.IsHex(s) {
		return true
	}
	if len(s) == 2*Length160 && cmn.IsHex("0x"+s) {
		return true
	}
	return false
}

// Get the string representation of the underlying address
func (a Hash160) Str() string   { return string(a[:]) }
func (a Hash160) Bytes() []byte { return a[:] }
func (a Hash160) Big() *big.Int { return cmn.Bytes2Big(a[:]) }
func (a Hash160) Hash() Hash160    { return BytesToHash160(a[:]) }
func (a Hash160) Hex() string   { return hexutil.Encode(a[:]) }

// Sets the address to the value of b. If b is larger than len(a) it will panic
func (a *Hash160) SetBytes(b []byte) {
	if len(b) > len(a) {
		b = b[len(b)-Length160:]
	}
	copy(a[Length160-len(b):], b)
}

// Set string `s` to a. If s is larger than len(a) it will panic
func (a *Hash160) SetString(s string) { a.SetBytes([]byte(s)) }

// Sets a to other
func (a *Hash160) Set(other Hash160) {
	for i, v := range other {
		a[i] = v
	}
}

// Serialize given address to JSON
func (a Hash160) MarshalJSON() ([]byte, error) {
	return hexutil.Bytes(a[:]).MarshalJSON()
}

// Parse address from raw json data
func (a *Hash160) UnmarshalJSON(input []byte) error {
	return hexutil.UnmarshalJSON("Hash160", input, a[:])
}


/////////// Hash256
func BytesToHash256(b []byte) Hash256 {
	var a Hash256
	a.SetBytes(b)
	return a
}
func StringToHash256(s string) Hash256 { return BytesToHash256([]byte(s)) }
func BigToHash256(b *big.Int) Hash256  { return BytesToHash256(b.Bytes()) }
func HexToHash256(s string) Hash256    { return BytesToHash256(cmn.FromHex(s)) }

// IsHexAddress verifies whether a string can represent a valid hex-encoded
// Ethereum address or not.
func IsHexHash256(s string) bool {
	if len(s) == 2+2*Length256 && cmn.IsHex(s) {
		return true
	}
	if len(s) == 2*Length256 && cmn.IsHex("0x"+s) {
		return true
	}
	return false
}

// Get the string representation of the underlying address
func (a Hash256) Str() string   { return string(a[:]) }
func (a Hash256) Bytes() []byte { return a[:] }
func (a Hash256) Big() *big.Int { return cmn.Bytes2Big(a[:]) }
func (a Hash256) Hash() Hash256    { return BytesToHash256(a[:]) }
func (a Hash256) Hex() string   { return hexutil.Encode(a[:]) }

// Sets the address to the value of b. If b is larger than len(a) it will panic
func (a *Hash256) SetBytes(b []byte) {
	if len(b) > len(a) {
		b = b[len(b)-Length256:]
	}
	copy(a[Length256-len(b):], b)
}

// Set string `s` to a. If s is larger than len(a) it will panic
func (a *Hash256) SetString(s string) { a.SetBytes([]byte(s)) }

// Sets a to other
func (a *Hash256) Set(other Hash256) {
	for i, v := range other {
		a[i] = v
	}
}

// Serialize given address to JSON
func (a Hash256) MarshalJSON() ([]byte, error) {
	return hexutil.Bytes(a[:]).MarshalJSON()
}

// Parse address from raw json data
func (a *Hash256) UnmarshalJSON(input []byte) error {
	return hexutil.UnmarshalJSON("Hash256", input, a[:])
}
