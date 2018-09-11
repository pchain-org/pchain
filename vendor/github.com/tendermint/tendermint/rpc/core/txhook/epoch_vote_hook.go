package core

import (
	"errors"
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	ethTypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/tendermint/go-crypto"
	"github.com/tendermint/tendermint/epoch"
	"golang.org/x/crypto/sha3"
	"math/big"
)

const (
	VNEFuncName = "VoteNextEpoch"
	REVFuncName = "RevealVote"
)

func init() {
	RegisterCheckTxCb(VNEFuncName, voteForEpochCheckTxCb)
	RegisterCheckTxCb(REVFuncName, revealTheVoteCheckTxCb)

	RegisterDeliverTxCb(VNEFuncName, voteForEpochDeliverTxCb)
	RegisterDeliverTxCb(REVFuncName, revealTheVoteDeliverTxCb)
}

func voteForEpochCheckTxCb(height int, tx *ethTypes.Transaction, ep *epoch.Epoch) error {
	plog.Debugln("voteForEpochCheckTxCb CheckTx Callback")
	// Check Epoch in Hash Vote stage
	if ep.NextEpoch == nil {
		return errors.New("next Epoch is nil, You can't vote the next epoch")
	}

	// Vote is valid between height 75% - 85%
	if !ep.CheckInHashVoteStage(height) {
		return errors.New(fmt.Sprintf("you can't send the hash vote during this time, current height %v", height))
	}
	return nil
}

// voteForEpochDeliverTxCb save the vote from other validator to DB
func voteForEpochDeliverTxCb(tx *ethTypes.Transaction, ep *epoch.Epoch) error {

	plog.Debugln("voteForEpochDeliverTxCb DeliverTx Callback")

	plog.Debugln("voteForEpochDeliverTxCb, params are: %s\n", tx.ExtendTxData().Params.String())

	txhash := tx.Hash()

	// Load Payload from tx
	params := tx.ExtendTxData().Params
	fromVar, _ := params.Get("from")
	from := common.BytesToAddress(fromVar.([]byte))
	voteHashVar, _ := params.Get("voteHash")
	voteHash := common.BytesToHash(voteHashVar.([]byte))

	plog.Debugln("voteForEpochDeliverTxCb, after decode, params are: (epoch, from, voteHash) = (%v, %x, %x)",
		ep.NextEpoch.Number, from, voteHash)

	voteSet := ep.NextEpoch.GetEpochValidatorVoteSet()
	vote, exist := voteSet.GetVoteByAddress(from)

	if exist {
		// Overwrite the Previous Hash Vote
		vote.VoteHash = voteHash
		vote.TxHash = txhash
	} else {
		// Create a new Hash Vote
		vote = &epoch.EpochValidatorVote{
			Address:  from,
			VoteHash: voteHash,
			TxHash:   txhash,
		}
		voteSet.StoreVote(vote)
	}

	return nil
}

func revealTheVoteCheckTxCb(height int, tx *ethTypes.Transaction, ep *epoch.Epoch) error {
	plog.Debugln("revealTheVoteCheckTxCb CheckTx Callback")
	// Check Epoch in Reveal Vote stage
	if ep.NextEpoch == nil {
		return errors.New("next Epoch is nil, You can't vote the next epoch")
	}

	// Vote is valid between height 85% - 95%
	if !ep.CheckInRevealVoteStage(height) {
		return errors.New(fmt.Sprintf("you can't send the reveal vote during this time, current height %v", height))
	}

	// Load Payload from tx
	params := tx.ExtendTxData().Params
	fromVar, _ := params.Get("from")
	from := common.BytesToAddress(fromVar.([]byte))
	pubkeyVar, _ := params.Get("pubkey")
	pubkey := string(pubkeyVar.([]byte))
	amountVar, _ := params.Get("amount")
	amount := new(big.Int).SetBytes(amountVar.([]byte))
	saltVar, _ := params.Get("salt")
	salt := string(saltVar.([]byte))

	voteSet := ep.NextEpoch.GetEpochValidatorVoteSet()
	vote, exist := voteSet.GetVoteByAddress(from)

	// Check Vote exist
	if !exist {
		return errors.New(fmt.Sprintf("Can not found the vote for Address %x", from))
	}

	if len(vote.VoteHash) == 0 {
		return errors.New(fmt.Sprintf("Address %x doesn't has vote hash", from))
	}

	// Check Vote Hash
	byte_data := [][]byte{
		from.Bytes(),
		common.FromHex(pubkey),
		amount.Bytes(),
		[]byte(salt),
	}
	voteHash := sha3.Sum256(concatCopyPreAllocate(byte_data))
	if vote.VoteHash != voteHash {
		return errors.New("your vote doesn't match your vote hash, please check your vote")
	}

	return nil
}

// revealTheVoteDeliverTxCb update the real vote data from other validator to DB
func revealTheVoteDeliverTxCb(tx *ethTypes.Transaction, ep *epoch.Epoch) error {

	plog.Debugln("revealTheVoteDeliverTxCb DeliverTx Callback")

	plog.Debugln("revealTheVoteDeliverTxCb, params are: %s\n", tx.ExtendTxData().Params.String())

	txhash := tx.Hash()

	// Load Payload from tx
	params := tx.ExtendTxData().Params
	fromVar, _ := params.Get("from")
	from := common.BytesToAddress(fromVar.([]byte))
	pubkeyVar, _ := params.Get("pubkey")
	pubkey := string(pubkeyVar.([]byte))
	amountVar, _ := params.Get("amount")
	amount := new(big.Int).SetBytes(amountVar.([]byte))
	saltVar, _ := params.Get("salt")
	salt := string(saltVar.([]byte))

	plog.Debugln("revealTheVoteDeliverTxCb, after decode, params are: (epoch, from, pubkey, amount, salt) = (%v, %s, %s, %v, %s)",
		ep.NextEpoch.Number, from, pubkey, amount, salt)

	voteSet := ep.NextEpoch.GetEpochValidatorVoteSet()
	vote, exist := voteSet.GetVoteByAddress(from)

	if exist {
		// Update the Hash Vote with Real Data
		vote.PubKey = crypto.BLSPubKey(common.FromHex(pubkey))
		vote.Amount = amount
		vote.Salt = salt
		vote.TxHash = txhash
	}

	return nil
}

func concatCopyPreAllocate(slices [][]byte) []byte {
	var totalLen int
	for _, s := range slices {
		totalLen += len(s)
	}
	tmp := make([]byte, totalLen)
	var i int
	for _, s := range slices {
		i += copy(tmp[i:], s)
	}
	return tmp
}
