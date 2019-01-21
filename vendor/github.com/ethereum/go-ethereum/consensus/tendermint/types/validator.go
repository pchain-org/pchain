package types

import (
	"bytes"
	"fmt"
	"io"

	"github.com/ethereum/go-ethereum/common"
	ethTypes "github.com/ethereum/go-ethereum/core/types"
	. "github.com/tendermint/go-common"
	"github.com/tendermint/go-crypto"
	"github.com/tendermint/go-wire"
	"math/big"
)

// Volatile state for each Validator
// TODO: make non-volatile identity
type Validator struct {
	Address     []byte        `json:"address"`
	PubKey      crypto.PubKey `json:"pub_key"`
	VotingPower *big.Int      `json:"voting_power"`
}

func NewValidator(pubKey crypto.PubKey, votingPower *big.Int) *Validator {
	return &Validator{
		Address:     pubKey.Address(),
		PubKey:      pubKey,
		VotingPower: votingPower,
	}
}

// Creates a new copy of the validator so we can mutate accum.
// Panics if the validator is nil.
func (v *Validator) Copy() *Validator {
	vCopy := *v
	vCopy.VotingPower = new(big.Int).Set(v.VotingPower)
	return &vCopy
}

func (v *Validator) Equals(other *Validator) bool {

	return bytes.Equal(v.Address, other.Address) &&
		v.PubKey.Equals(other.PubKey) &&
		v.VotingPower.Cmp(other.VotingPower) == 0
}

func (v *Validator) String() string {
	if v == nil {
		return "nil-Validator"
	}
	return fmt.Sprintf("Validator{ADD:%X PK:%X VP:%v}",
		v.Address,
		v.PubKey,
		v.VotingPower)
}

func (v *Validator) Hash() []byte {
	return wire.BinaryRipemd160(v)
}

//-------------------------------------

var ValidatorCodec = validatorCodec{}

type validatorCodec struct{}

func (vc validatorCodec) Encode(o interface{}, w io.Writer, n *int, err *error) {
	wire.WriteBinary(o.(*Validator), w, n, err)
}

func (vc validatorCodec) Decode(r io.Reader, n *int, err *error) interface{} {
	return wire.ReadBinary(&Validator{}, r, 0, n, err)
}

func (vc validatorCodec) Compare(o1 interface{}, o2 interface{}) int {
	PanicSanity("ValidatorCodec.Compare not implemented")
	return 0
}

//-------------------------------------

type RefundValidatorAmount struct {
	Address common.Address
	Amount  *big.Int // Amount will be nil when Voteout is true
	Voteout bool     // Voteout means refund all the amount (self deposit + delegate)
}

// SwitchEpoch op
type SwitchEpochOp struct {
	NewValidators *ValidatorSet
}

func (op *SwitchEpochOp) Conflict(op1 ethTypes.PendingOp) bool {
	if _, ok := op1.(*SwitchEpochOp); ok {
		// Only one SwitchEpochOp is allowed in each block
		return true
	}
	return false
}

func (op *SwitchEpochOp) String() string {
	return fmt.Sprintf("SwitchEpochOp - New Validators: %v", op.NewValidators)
}
