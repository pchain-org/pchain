package types

import (
	"errors"
	//"io"

	"github.com/ethereum/go-ethereum/common"
	//"github.com/ethereum/go-ethereum/rlp"
)

var (
	// IstanbulDigest represents a hash of "Istanbul practical byzantine fault tolerance"
	// to identify whether the block is from Istanbul consensus engine
	TendermintDigest = common.HexToHash("0x63746963616c2062797a616e74696e65206661756c7420746f6c6572616e6365")

	// ErrInvalidIstanbulHeaderExtra is returned if the length of extra-data is less than 32 bytes
	ErrInvalidTendermintHeaderExtra = errors.New("invalid istanbul header extra-data")
)

var (
	EPOCHDATA int = 0
	COMMITEDSEAL int = 1
)

type TendermintExtra struct {
	Type          int
	EpochData     []byte
	CommittedSeal [][]byte
}
