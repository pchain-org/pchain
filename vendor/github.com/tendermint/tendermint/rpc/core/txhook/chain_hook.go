package core

import (
	"bytes"
	"fmt"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
)

var (
	WFCCFuncName string = "WithdrawFromChildChain"
)

func init() {
	RegisterCommitCb(WFCCFuncName, wfccCommitCb)
}

func wfccCommitCb(brCommit BrCommit) error {

	fmt.Println("wfccCommitCb")

	chainID := brCommit.GetChainId()
	if chainID != "pchain" {
		// save only when there're WFCCFuncName tx inside the block.
		toSave := false
		block := brCommit.GetCurrentBlock()
		for _, tx := range block.Data.Txs {
			ethTx := new(types.Transaction)
			rlpStream := rlp.NewStream(bytes.NewBuffer(tx), 0)
			if err := ethTx.DecodeRLP(rlpStream); err != nil {
				fmt.Printf("wfccCommitCb: ethtx.DecodeRLP(rlpStream) error with: %s\n", err.Error())
				continue
			}

			etd := ethTx.ExtendTxData()
			if etd != nil && etd.FuncName == WFCCFuncName {
				toSave = true
				break
			}
		}

		if toSave {
			brCommit.SaveCurrentBlock2MainChain()
		}
	}
	return nil
}
