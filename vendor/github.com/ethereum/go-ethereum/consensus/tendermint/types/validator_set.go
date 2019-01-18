package types

import (
	"bytes"
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"math/big"
	"sort"
	"strings"

	"github.com/ethereum/go-ethereum/log"
	cmn "github.com/tendermint/go-common"
	crypto "github.com/tendermint/go-crypto"
	"github.com/tendermint/go-merkle"
)

// ValidatorSet represent a set of *Validator at a given height.
// The validators can be fetched by address or index.
// The index is in order of .Address, so the indices are fixed
// for all rounds of a given blockchain height.
// On the other hand, the .AccumPower of each validator and
// the designated .GetProposer() of a set changes every round,
// upon calling .IncrementAccum().
// NOTE: Not goroutine-safe.
// NOTE: All get/set to validators should copy the value for safety.
// TODO: consider validator Accum overflow
// TODO: move valset into an iavl tree where key is 'blockbonded|pubkey'
type ValidatorSet struct {
	// NOTE: persisted via reflect, must be exported.
	Validators []*Validator `json:"validators"`
	// cached (unexported)
	totalVotingPower *big.Int
}

func NewValidatorSet(vals []*Validator) *ValidatorSet {
	validators := make([]*Validator, len(vals))
	for i, val := range vals {
		validators[i] = val.Copy()
	}
	sort.Sort(ValidatorsByAddress(validators))
	vs := &ValidatorSet{
		Validators: validators,
	}

	return vs
}

func (valSet *ValidatorSet) Copy() *ValidatorSet {
	validators := make([]*Validator, len(valSet.Validators))
	for i, val := range valSet.Validators {
		// NOTE: must copy, since IncrementAccum updates in place.
		validators[i] = val.Copy()
	}
	return &ValidatorSet{
		Validators:       validators,
		totalVotingPower: valSet.totalVotingPower,
	}
}

func (valSet *ValidatorSet) AggrPubKey(bitMap *cmn.BitArray) crypto.PubKey {
	if bitMap == nil {
		return nil
	}
	if (int)(bitMap.Size()) != len(valSet.Validators) {
		return nil
	}
	validators := valSet.Validators
	var pks []*crypto.PubKey
	for i := (uint64)(0); i < bitMap.Size(); i++ {
		if bitMap.GetIndex(i) {
			pks = append(pks, &(validators[i].PubKey))
		}
	}
	return crypto.BLSPubKeyAggregate(pks)
}

func (valSet *ValidatorSet) TalliedVotingPower(bitMap *cmn.BitArray) (*big.Int, error) {
	if bitMap == nil {
		return big.NewInt(0), fmt.Errorf("Invalid bitmap(nil)")
	}
	validators := valSet.Validators
	if validators == nil {
		return big.NewInt(0), fmt.Errorf("Invalid validators(nil)")
	}
	if valSet.Size() != (int)(bitMap.Size()) {
		return big.NewInt(0), fmt.Errorf("Size is not equal, validators size:%v, bitmap size:%v", valSet.Size(), bitMap.Size())
	}
	powerSum := big.NewInt(0)
	for i := (uint64)(0); i < bitMap.Size(); i++ {
		if bitMap.GetIndex(i) {
			powerSum.Add(powerSum, common.Big1)
		}
	}
	return powerSum, nil
}

func (valSet *ValidatorSet) Equals(other *ValidatorSet) bool {

	if len(valSet.Validators) != len(other.Validators) {
		return false
	}

	for _, v := range other.Validators {

		_, val := valSet.GetByAddress(v.Address)
		if val == nil || !val.Equals(v) {
			return false
		}
	}

	return true
}

// HasAddress returns true if address given is in the validator set, false -
// otherwise.
func (valSet *ValidatorSet) HasAddress(address []byte) bool {
	idx := sort.Search(len(valSet.Validators), func(i int) bool {
		return bytes.Compare(address, valSet.Validators[i].Address) <= 0
	})
	return idx < len(valSet.Validators) && bytes.Equal(valSet.Validators[idx].Address, address)
}

func (valSet *ValidatorSet) GetByAddress(address []byte) (index int, val *Validator) {
	idx := sort.Search(len(valSet.Validators), func(i int) bool {
		return bytes.Compare(address, valSet.Validators[i].Address) <= 0
	})
	if idx != len(valSet.Validators) && bytes.Compare(valSet.Validators[idx].Address, address) == 0 {
		return idx, valSet.Validators[idx].Copy()
	} else {
		return 0, nil
	}
}

func (valSet *ValidatorSet) GetByIndex(index int) (address []byte, val *Validator) {
	val = valSet.Validators[index]
	return val.Address, val.Copy()
}

func (valSet *ValidatorSet) Size() int {
	return len(valSet.Validators)
}

func (valSet *ValidatorSet) TotalVotingPower() *big.Int {
	return big.NewInt(int64(valSet.Size()))
}

func (valSet *ValidatorSet) Hash() []byte {
	if len(valSet.Validators) == 0 {
		return nil
	}
	hashables := make([]merkle.Hashable, len(valSet.Validators))
	for i, val := range valSet.Validators {
		hashables[i] = val
	}
	return merkle.SimpleHashFromHashables(hashables)
}

func (valSet *ValidatorSet) Add(val *Validator) (added bool) {
	val = val.Copy()
	idx := sort.Search(len(valSet.Validators), func(i int) bool {
		return bytes.Compare(val.Address, valSet.Validators[i].Address) <= 0
	})
	if idx == len(valSet.Validators) {
		valSet.Validators = append(valSet.Validators, val)
		// Invalidate cache
		valSet.totalVotingPower = nil
		return true
	} else if bytes.Compare(valSet.Validators[idx].Address, val.Address) == 0 {
		return false
	} else {
		newValidators := make([]*Validator, len(valSet.Validators)+1)
		copy(newValidators[:idx], valSet.Validators[:idx])
		newValidators[idx] = val
		copy(newValidators[idx+1:], valSet.Validators[idx:])
		valSet.Validators = newValidators
		// Invalidate cache
		valSet.totalVotingPower = nil
		return true
	}
}

func (valSet *ValidatorSet) Update(val *Validator) (updated bool) {
	index, sameVal := valSet.GetByAddress(val.Address)
	if sameVal == nil {
		return false
	} else {
		valSet.Validators[index] = val.Copy()
		// Invalidate cache
		valSet.totalVotingPower = nil
		return true
	}
}

func (valSet *ValidatorSet) Remove(address []byte) (val *Validator, removed bool) {
	idx := sort.Search(len(valSet.Validators), func(i int) bool {
		return bytes.Compare(address, valSet.Validators[i].Address) <= 0
	})
	if idx == len(valSet.Validators) || bytes.Compare(valSet.Validators[idx].Address, address) != 0 {
		return nil, false
	} else {
		removedVal := valSet.Validators[idx]
		newValidators := valSet.Validators[:idx]
		if idx+1 < len(valSet.Validators) {
			newValidators = append(newValidators, valSet.Validators[idx+1:]...)
		}
		valSet.Validators = newValidators
		// Invalidate cache
		valSet.totalVotingPower = nil
		return removedVal, true
	}
}

func (valSet *ValidatorSet) Iterate(fn func(index int, val *Validator) bool) {
	for i, val := range valSet.Validators {
		stop := fn(i, val.Copy())
		if stop {
			break
		}
	}
}

// Verify that +2/3 of the set had signed the given signBytes
func (valSet *ValidatorSet) VerifyCommit(chainID string, height uint64, commit *Commit) error {

	log.Debugf("(valSet *ValidatorSet) VerifyCommit(), avoid valSet and commit.Precommits size check for validatorset change\n")
	if commit == nil {
		return fmt.Errorf("Invalid commit(nil)")
	}
	if (uint64)(valSet.Size()) != commit.BitArray.Size() {
		return fmt.Errorf("Invalid commit -- wrong set size: %v vs %v", valSet.Size(), commit.BitArray.Size())
	}
	if height != commit.Height {
		return fmt.Errorf("Invalid commit -- wrong height: %v vs %v", height, commit.Height)
	}

	pubKey := valSet.AggrPubKey(commit.BitArray)
	vote := &Vote{

		BlockID: commit.BlockID,
		Height:  commit.Height,
		Round:   (uint64)(commit.Round),
		Type:    commit.Type(),
	}
	if !pubKey.VerifyBytes(SignBytes(chainID, vote), commit.SignAggr) {
		return fmt.Errorf("Invalid commit -- wrong Signature:%v or BitArray:%v", commit.SignAggr, commit.BitArray)
	}

	talliedVotingPower, err := valSet.TalliedVotingPower(commit.BitArray)
	if err != nil {
		return err
	}

	/*
		quorum := big.NewInt(0)
		quorum.Mul(valSet.TotalVotingPower(), big.NewInt(2))
		quorum.Div(quorum, big.NewInt(3))
		quorum.Add(quorum, big.NewInt(1))
	*/
	quorum := Loose23MajorThreshold(valSet.TotalVotingPower(), commit.Round)

	if talliedVotingPower.Cmp(quorum) >= 0 {
		return nil
	} else {
		return fmt.Errorf("Invalid commit -- insufficient voting power: got %v, needed %v",
			talliedVotingPower, (quorum))
	}
}

// Verify that +2/3 of this set had signed the given signBytes.
// Unlike VerifyCommit(), this function can verify commits with differeent sets.
func (valSet *ValidatorSet) VerifyCommitAny(chainID string, blockID BlockID, height int, commit *Commit) error {
	panic("Not yet implemented")
	/*
			Start like:

		FOR_LOOP:
			for _, val := range vals {
				if len(precommits) == 0 {
					break FOR_LOOP
				}
				next := precommits[0]
				switch bytes.Compare(val.Address(), next.ValidatorAddress) {
				case -1:
					continue FOR_LOOP
				case 0:
					signBytes := tm.SignBytes(next)
					...
				case 1:
					... // error?
				}
			}
	*/
}

func (valSet *ValidatorSet) String() string {
	return valSet.StringIndented("")
}

func (valSet *ValidatorSet) StringIndented(indent string) string {
	if valSet == nil {
		return "nil-ValidatorSet"
	}
	valStrings := []string{}
	valSet.Iterate(func(index int, val *Validator) bool {
		valStrings = append(valStrings, val.String())
		return false
	})
	return fmt.Sprintf(`ValidatorSet{
%s  Validators:
%s    %v
%s}`,
		indent,
		indent, strings.Join(valStrings, "\n"+indent+"    "),
		indent)

}

//-------------------------------------
// Implements sort for sorting validators by address.

type ValidatorsByAddress []*Validator

func (vs ValidatorsByAddress) Len() int {
	return len(vs)
}

func (vs ValidatorsByAddress) Less(i, j int) bool {
	return bytes.Compare(vs[i].Address, vs[j].Address) == -1
}

func (vs ValidatorsByAddress) Swap(i, j int) {
	it := vs[i]
	vs[i] = vs[j]
	vs[j] = it
}
