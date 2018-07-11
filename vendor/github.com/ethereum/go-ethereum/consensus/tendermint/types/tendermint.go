package types



import (
	"errors"
	"io"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	ethTypes "github.com/ethereum/go-ethereum/core/types"
)

var (
	// IstanbulDigest represents a hash of "Istanbul practical byzantine fault tolerance"
	// to identify whether the block is from Istanbul consensus engine
	TendermintDigest = common.HexToHash("0x63746963616c2062797a616e74696e65206661756c7420746f6c6572616e6365")

	TendermintExtraVanity = 32 // Fixed number of extra-data bytes reserved for validator vanity
	TendermintExtraSeal   = 65 // Fixed number of extra-data bytes reserved for validator seal

	// ErrInvalidIstanbulHeaderExtra is returned if the length of extra-data is less than 32 bytes
	ErrInvalidTendermintHeaderExtra = errors.New("invalid istanbul header extra-data")
)

type TendermintExtra struct {
	Header *Header    `json:"header"`
	LastCommit *Commit `json:"last_commit"`
}

// EncodeRLP serializes ist into the Ethereum RLP format.
func (te *TendermintExtra) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, []interface{}{
		te.Header,
		te.LastCommit,
	})
}

// DecodeRLP implements rlp.Decoder, and load the istanbul fields from a RLP stream.
func (te *TendermintExtra) DecodeRLP(s *rlp.Stream) error {
	var tdmExtra = TendermintExtra{}
	if err := s.Decode(&tdmExtra); err != nil {
		return err
	}
	te.Header, te.LastCommit = tdmExtra.Header, tdmExtra.LastCommit
	return nil
}

// ExtractTendermintExtra extracts all values of the TendermintExtra from the header. It returns an
// error if the length of the given extra-data is less than 32 bytes or the extra-data can not
// be decoded.
func ExtractTendermintExtra(h *ethTypes.Header) (*TendermintExtra, error) {
	if len(h.Extra) < TendermintExtraVanity {
		return nil, ErrInvalidTendermintHeaderExtra
	}

	var tdmExtra *TendermintExtra
	err := rlp.DecodeBytes(h.Extra[:], &tdmExtra)
	if err != nil {
		return nil, err
	}
	return tdmExtra, nil
}

// TendermintFilteredHeader returns a filtered header which some information (like seal, committed seals)
// are clean to fulfill the Tendermint hash rules. It returns nil if the extra-data cannot be
// decoded/encoded by rlp.
func TendermintFilteredHeader(h *ethTypes.Header, keepSeal bool) *ethTypes.Header {
	newHeader := ethTypes.CopyHeader(h)
	tdmExtra, err := ExtractTendermintExtra(newHeader)
	if err != nil {
		return nil
	}

	tdmExtra.Header = &Header{}
	tdmExtra.LastCommit = &Commit{}

	payload, err := rlp.EncodeToBytes(&tdmExtra)
	if err != nil {
		return nil
	}

	newHeader.Extra = append(newHeader.Extra[:], payload...)

	return newHeader
}
