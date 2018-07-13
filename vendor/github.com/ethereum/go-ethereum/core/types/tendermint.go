package types

import (
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	//"github.com/ethereum/go-ethereum/rlp"
)

var (
	// TendermintDigest represents a hash of "Tendermint practical byzantine fault tolerance"
	// to identify whether the block is from Tendermint consensus engine
	TendermintDigest = common.HexToHash("54656e6465726d696e742070726163746963616c2062797a616e74696e65206661756c7420746f6c6572616e6365")

	TendermintExtraVanity = 32 // Fixed number of extra-data bytes reserved for validator vanity
	TendermintExtraSeal   = 65 // Fixed number of extra-data bytes reserved for validator seal

	// ErrInvalidIstanbulHeaderExtra is returned if the length of extra-data is less than 32 bytes
	ErrInvalidTendermintHeaderExtra = errors.New("invalid istanbul header extra-data")

	MagicExtra = []byte("pchain_tmp_extra")
)

// TendermintFilteredHeader returns a filtered header which some information (like seal, committed seals)
// are clean to fulfill the Tendermint hash rules. It returns nil if the extra-data cannot be
// decoded/encoded by rlp.
func TendermintFilteredHeader(h *Header, keepSeal bool) *Header {
	newHeader := CopyHeader(h)

	/*
	tdmExtra, err := ExtractTendermintExtra(newHeader)

	if err != nil {
		return nil
	}

	payload, err := rlp.EncodeToBytes(&tdmExtra)
	if err != nil {
		return nil
	}
	*/
	payload := MagicExtra
	newHeader.Extra = payload
	fmt.Printf("TendermintFilteredHeader， newHeader.Extra is %x\n", newHeader.Extra)
	return newHeader
}

