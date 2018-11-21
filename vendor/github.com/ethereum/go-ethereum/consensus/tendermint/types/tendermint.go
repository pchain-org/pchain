package types

import (
	"fmt"
	ethTypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/tendermint/go-merkle"
	"github.com/tendermint/go-wire"
	"time"
)

type TendermintExtra struct {
	ChainID         string    `json:"chain_id"`
	Height          uint64    `json:"height"`
	Time            time.Time `json:"time"`
	NeedToSave      bool      `json:"need_to_save"`
	NeedToBroadcast bool      `json:"need_to_broadcast"`
	EpochNumber     uint64    `json:"epoch_number"`
	SeenCommitHash  []byte    `json:"last_commit_hash"` // commit from validators from the last block
	ValidatorsHash  []byte    `json:"validators_hash"`  // validators for the current block
	SeenCommit      *Commit   `json:"seen_commit"`
	EpochBytes      []byte    `json:"epoch_bytes"`
}

/*
// EncodeRLP serializes ist into the Ethereum RLP format.
func (te *TendermintExtra) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, []interface{}{
		te.ChainID, te.Height, te.Time, te.LastBlockID,
		te.SeenCommitHash, te.ValidatorsHash,
		te.SeenCommit,
	})
}

// DecodeRLP implements rlp.Decoder, and load the istanbul fields from a RLP stream.
func (te *TendermintExtra) DecodeRLP(s *rlp.Stream) error {
	var tdmExtra TendermintExtra
	if err := s.Decode(&tdmExtra); err != nil {
		return err
	}
	te.ChainID, te.Height, te.Time, te.LastBlockID,
		te.SeenCommitHash, te.ValidatorsHash,
		te.SeenCommit = tdmExtra.ChainID, tdmExtra.Height, tdmExtra.Time, tdmExtra.LastBlockID,
		tdmExtra.SeenCommitHash, tdmExtra.ValidatorsHash,
		tdmExtra.SeenCommit
	return nil
}
*/

//be careful, here not deep copy because just reference to SeenCommit
func (te *TendermintExtra) Copy() *TendermintExtra {
	//fmt.Printf("State.Copy(), s.LastValidators are %v\n",s.LastValidators)
	//debug.PrintStack()

	return &TendermintExtra{
		ChainID:         te.ChainID,
		Height:          te.Height,
		Time:            te.Time,
		NeedToSave:      te.NeedToSave,
		NeedToBroadcast: te.NeedToBroadcast,
		EpochNumber:     te.EpochNumber,
		SeenCommitHash:  te.SeenCommitHash,
		ValidatorsHash:  te.ValidatorsHash,
		SeenCommit:      te.SeenCommit,
		EpochBytes:      te.EpochBytes,
	}
}

// NOTE: hash is nil if required fields are missing.
func (te *TendermintExtra) Hash() []byte {
	if len(te.ValidatorsHash) == 0 {
		return nil
	}
	return merkle.SimpleHashFromMap(map[string]interface{}{
		"ChainID":         te.ChainID,
		"Height":          te.Height,
		"Time":            te.Time,
		"NeedToSave":      te.NeedToSave,
		"NeedToBroadcast": te.NeedToBroadcast,
		"EpochNumber":     te.EpochNumber,
		"Validators":      te.ValidatorsHash,
		"EpochBytes":      te.EpochBytes,
	})
}

// ExtractTendermintExtra extracts all values of the TendermintExtra from the header. It returns an
// error if the length of the given extra-data is less than 32 bytes or the extra-data can not
// be decoded.
func ExtractTendermintExtra(h *ethTypes.Header) (*TendermintExtra, error) {

	if len(h.Extra) == 0 {
		return &TendermintExtra{}, nil
	}

	var tdmExtra = TendermintExtra{}
	err := wire.ReadBinaryBytes(h.Extra[:], &tdmExtra)
	//err := rlp.DecodeBytes(h.Extra[:], &tdmExtra)
	if err != nil {
		return nil, err
	}
	return &tdmExtra, nil
}

func (te *TendermintExtra) String() string {
	str := fmt.Sprintf(`TendermintExtra: {
ChainID:     %s
EpochNumber: %v
Height:      %v
Time:        %v

EpochBytes: length %v
}
`, te.ChainID, te.EpochNumber, te.Height, te.Time, len(te.EpochBytes))
	return str
}
