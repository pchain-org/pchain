package epoch

import (
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/tendermint/go-crypto"
	"github.com/tendermint/go-db"
	"github.com/tendermint/go-wire"
	"math/big"
	"sync"
)

var voteRWMutex sync.RWMutex

// Epoch Validator Vote Set
// Store in the Level DB will be Key + EpochValidatorVoteSet
// Key   = string EpochValidatorVoteKey
// Value = []byte EpochValidatorVoteSet
// eg. Key: EpochValidatorVote_1, EpochValidatorVote_2
func calcEpochValidatorVoteKey(epochNumber uint64) []byte {
	return []byte(fmt.Sprintf("EpochValidatorVote_%v", epochNumber))
}

type EpochValidatorVoteSet struct {
	// Store the Votes
	Votes []*EpochValidatorVote
	// For fast searching, key = Address Hex (not export)
	votesByAddress map[common.Address]*EpochValidatorVote
}

type EpochValidatorVote struct {
	Address  common.Address
	PubKey   crypto.PubKey
	Amount   *big.Int
	Salt     string
	VoteHash common.Hash // VoteHash = Sha3(Epoch Number + PubKey + Amount + Salt)
	TxHash   common.Hash
}

func NewEpochValidatorVoteSet() *EpochValidatorVoteSet {
	return &EpochValidatorVoteSet{
		Votes:          make([]*EpochValidatorVote, 0),
		votesByAddress: make(map[common.Address]*EpochValidatorVote),
	}
}

// GetVoteByAddress get the Vote from VoteSet by Address Hex Key
func (voteSet *EpochValidatorVoteSet) GetVoteByAddress(address common.Address) (vote *EpochValidatorVote, exist bool) {
	voteRWMutex.RLock()
	defer voteRWMutex.RUnlock()

	vote, exist = voteSet.votesByAddress[address]
	return
}

// StoreVote insert or update the Vote into VoteSet by Address Hex Key
func (voteSet *EpochValidatorVoteSet) StoreVote(vote *EpochValidatorVote) {
	voteRWMutex.Lock()
	defer voteRWMutex.Unlock()

	oldVote, exist := voteSet.votesByAddress[vote.Address]
	if exist {
		// Exist, remove it
		index := -1
		for i:=0; i<len(voteSet.Votes); i++ {
			if voteSet.Votes[i] == oldVote {
				index = i
				break
			}
		}
		voteSet.Votes = append(voteSet.Votes[:index], voteSet.Votes[index+1:]...)
	}
	voteSet.votesByAddress[vote.Address] = vote
	voteSet.Votes = append(voteSet.Votes, vote)
}

func SaveEpochVoteSet(epochDB db.DB, epochNumber uint64, voteSet *EpochValidatorVoteSet) {
	voteRWMutex.Lock()
	defer voteRWMutex.Unlock()

	epochDB.SetSync(calcEpochValidatorVoteKey(epochNumber), wire.BinaryBytes(*voteSet))
}

func LoadEpochVoteSet(epochDB db.DB, epochNumber uint64) *EpochValidatorVoteSet {
	voteRWMutex.RLock()
	defer voteRWMutex.RUnlock()

	data := epochDB.Get(calcEpochValidatorVoteKey(epochNumber))
	if len(data) == 0 {
		return nil
	} else {
		var voteSet EpochValidatorVoteSet
		err := wire.ReadBinaryBytes(data, &voteSet)
		if err != nil {
			log.Error("Load Epoch Vote Set failed", "error", err)
			return nil
		}
		// Fulfill the Vote Map
		voteSet.votesByAddress = make(map[common.Address]*EpochValidatorVote)
		for _, v := range voteSet.Votes {
			voteSet.votesByAddress[v.Address] = v
		}
		return &voteSet
	}
}

func (voteSet *EpochValidatorVoteSet) Copy() *EpochValidatorVoteSet {
	if voteSet == nil {
		return nil
	}

	votes_copy := make([]*EpochValidatorVote, 0, len(voteSet.Votes))
	votesByAddress_copy := make(map[common.Address]*EpochValidatorVote, len(voteSet.Votes))
	for _, vote := range voteSet.Votes {
		v := vote.Copy()
		votes_copy = append(votes_copy, v)
		votesByAddress_copy[vote.Address] = v
	}

	return &EpochValidatorVoteSet{
		Votes:          votes_copy,
		votesByAddress: votesByAddress_copy,
	}
}

func (voteSet *EpochValidatorVoteSet) IsEmpty() bool {
	return voteSet == nil || len(voteSet.Votes) == 0
}

func (vote *EpochValidatorVote) Copy() *EpochValidatorVote {
	vCopy := *vote
	return &vCopy
}
