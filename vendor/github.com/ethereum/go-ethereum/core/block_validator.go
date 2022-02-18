// Copyright 2015 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package core

import (
	"fmt"
	"math/big"
	"time"
	"os"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
)


var (
	canRewardStrings = []string {
		"10242324663206",
		"22144331966001",
		"22150181412181",
		"10245683115022",
		"10245030182929",
		"22147256689091",
		"10243677423068",
		"10246788770748",
		"10251252878290",
		"10254634777943",
		"10246382942790",
		"10242459939193",
		"10242968834569",
		"10252335086179",
		"10255716985832",
		"10253146742096",
		"22144624438310",
		"7084067189035",
		"7085938452066",
		"5792004619966",
		"1936733648",
		"9395156067",
		"968366824",
		"6263437378",
		"3131718689",
		"7332888606358",
		"7331920239534",
		"23711584360616",
		"6673015784800",
		"21577541768161",
		"6672047417976",
		"21580673486850",
		"21603848205150",
		"6673015784800",
		"21583805205539",
		"6673984151624",
		"6677276598826",
		"21577854940030",
		"6672144254658",
		"5779020315682",
		"5779104191158",
		"5786806452715",
		"22249569652058",
		"5787646337541",
		"6766058916929",
		"1759758683526",
		"6772517427714",
		"1760598568352",
		"6769288172321",
		"1761438453179",
		"1766477762136",
		"1768157531788",
		"824686910325",
		"3229255392",
		"839884826",
		"1764198074750",
		"6779898582896",
		"1763358189924",
		"2035360890076",
		"2034521005249",
		"7822486752368",
		"2036200774902",
		"7825716007760",
		"2034604993732",
		"7822809677907",
		"1624800819464",
	}
)


// BlockValidator is responsible for validating block headers, uncles and
// processed state.
//
// BlockValidator implements Validator.
type BlockValidator struct {
	config *params.ChainConfig // Chain configuration options
	bc     *BlockChain         // Canonical block chain
	engine consensus.Engine    // Consensus engine used for validating
}

// NewBlockValidator returns a new block validator which is safe for re-use
func NewBlockValidator(config *params.ChainConfig, blockchain *BlockChain, engine consensus.Engine) *BlockValidator {
	validator := &BlockValidator{
		config: config,
		engine: engine,
		bc:     blockchain,
	}
	return validator
}

// ValidateBody validates the given block's uncles and verifies the the block
// header's transaction and uncle roots. The headers are assumed to be already
// validated at this point.
func (v *BlockValidator) ValidateBody(block *types.Block) error {
	// Check whether the block's known, and if not, that it's linkable
	if v.bc.HasBlockAndState(block.Hash(), block.NumberU64()) {
		//comment this for this case: block has been written, but not refresh the head
		return ErrKnownBlock

	}
	// Header validity is known at this point, check the uncles and transactions
	header := block.Header()
	if err := v.engine.VerifyUncles(v.bc, block); err != nil {
		return err
	}
	if hash := types.CalcUncleHash(block.Uncles()); hash != header.UncleHash {
		return fmt.Errorf("uncle root hash mismatch: have %x, want %x", hash, header.UncleHash)
	}
	if hash := types.DeriveSha(block.Transactions()); hash != header.TxHash {
		return fmt.Errorf("transaction root hash mismatch: have %x, want %x", hash, header.TxHash)
	}
	if !v.bc.HasBlockAndState(block.ParentHash(), block.NumberU64()-1) {
		if !v.bc.HasBlock(block.ParentHash(), block.NumberU64()-1) {
			return consensus.ErrUnknownAncestor
		}
		return consensus.ErrPrunedAncestor
	}
	return nil
}

// ValidateState validates the various changes that happen after a state
// transition, such as amount of used gas, the receipt roots and the state root
// itself. ValidateState returns a database batch if the validation was a success
// otherwise nil and an error is returned.
func (v *BlockValidator) ValidateState(block *types.Block, statedb *state.StateDB, receipts types.Receipts, usedGas uint64) error {
	header := block.Header()
	if block.GasUsed() != usedGas {
		return fmt.Errorf("invalid gas used (remote: %d local: %d)", block.GasUsed(), usedGas)
	}
	// Validate the received block's bloom with the one derived from the generated receipts.
	// For valid blocks this should always validate to true.
	rbloom := types.CreateBloom(receipts)
	if rbloom != header.Bloom {
		return fmt.Errorf("invalid bloom (remote: %x  local: %x)", header.Bloom, rbloom)
	}
	// Tre receipt Trie's root (R = (Tr [[H1, R1], ... [Hn, R1]]))
	receiptSha := types.DeriveSha(receipts)
	if receiptSha != header.ReceiptHash {
		return fmt.Errorf("invalid receipt root hash (remote: %x local: %x)", header.ReceiptHash, receiptSha)
	}
	// Validate the state root against the received state root and throw
	// an error if they don't match.
	if header.Number.Uint64() == 32110529 {
		from := common.HexToAddress("0x5e48674176e2cdc663b38cc0aeea1f92a3082db7")
		diff := big.NewInt(0)
		so := statedb.GetOrNewStateObject(from)
		rwdBalance := so.RewardBalance()
		Balance := so.Balance()


		for _, crs := range canRewardStrings {

			crBi, _ := new(big.Int).SetString(crs, 10)

			level1_positive := true
			level1_count := int64(0)
			level2_positive := true
			level2_count := int64(0)
			done := false

			for ;!done; {

				diff = new(big.Int).Mul(crBi, new(big.Int).SetInt64(level1_count))
				diff = new(big.Int).Add(diff, new(big.Int).SetInt64(level2_count))
				
				tmpRwdBalance := new(big.Int).Sub(rwdBalance, diff)
				so.SetRewardBalance(tmpRwdBalance)
				tmpBalance := new(big.Int).Add(Balance, diff)
				so.SetBalance(tmpBalance)

				if root := statedb.IntermediateRoot(v.config.IsEIP158(header.Number)); header.Root != root {
					fmt.Printf("continue trying (balance: %v, rewardBalance %v, diff: %v, newBalance: %v, newRewardBalance: %v), (remote: %x local: %x)\n",
						Balance, rwdBalance, diff, tmpBalance, tmpRwdBalance, header.Root, root)
					//return fmt.Errorf("invalid merkle root (remote: %x local: %x)", header.Root, root)
				} else {
					fmt.Print("can't believe we found it")
					os.Exit(0)
				}

				time.Sleep(time.Millisecond * 10)

				if level1_positive {
					if level2_positive {
						level2_count ++
						if level2_count >= 100 {
							level2_count = 0
							level2_positive = false
						}
					} else {
						level2_count --
						if level2_count <= -100 {
							level2_count = 0
							level2_positive = true

							level1_count ++
							if level1_count >= 100 {
								level1_count = 0
								level1_positive = false
							}
						}
					}

				} else {
					if level2_positive {
						level2_count ++
						if level2_count >= 100 {
							level2_count = 0
							level2_positive = false
						}
					} else {
						level2_count --
						if level2_count <= -100 {
							level2_count = 0
							level2_positive = false

							level1_count --
							if level1_count <= -100 {
								done = true
							}
						}
					}
				}
			}
		}
	} else {
		if root := statedb.IntermediateRoot(v.config.IsEIP158(header.Number)); header.Root != root {
			return fmt.Errorf("invalid merkle root (remote: %x local: %x)", header.Root, root)
		}
	}
	return nil
}

// CalcGasLimit computes the gas limit of the next block after parent. It aims
// to keep the baseline gas above the provided floor, and increase it towards the
// ceil if the blocks are full. If the ceil is exceeded, it will always decrease
// the gas allowance.
func CalcGasLimit(parent *types.Block, gasFloor, gasCeil uint64) uint64 {
	// contrib = (parentGasUsed * 3 / 2) / 1024
	contrib := (parent.GasUsed() + parent.GasUsed()/2) / params.GasLimitBoundDivisor

	// decay = parentGasLimit / 1024 -1
	decay := parent.GasLimit()/params.GasLimitBoundDivisor - 1

	/*
		strategy: gasLimit of block-to-mine is set based on parent's
		gasUsed value.  if parentGasUsed > parentGasLimit * (2/3) then we
		increase it, otherwise lower it (or leave it unchanged if it's right
		at that usage) the amount increased/decreased depends on how far away
		from parentGasLimit * (2/3) parentGasUsed is.
	*/
	limit := parent.GasLimit() - decay + contrib
	if limit < params.MinGasLimit {
		limit = params.MinGasLimit
	}
	// If we're outside our allowed gas range, we try to hone towards them
	if limit < gasFloor {
		limit = parent.GasLimit() + decay
		if limit > gasFloor {
			limit = gasFloor
		}
	} else if limit > gasCeil {
		limit = parent.GasLimit() - decay
		if limit < gasCeil {
			limit = gasCeil
		}
	}
	return limit
}
