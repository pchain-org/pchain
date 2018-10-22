package core

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"
	"testing"
)

// Tests cross chain tx storage and retrieval operations.
func TestCrossChainTx(t *testing.T) {
	db, _ := ethdb.NewMemDatabase()

	var chainId = "child_chain_test"
	var accountZ = common.HexToAddress("0x19018716")
	var accountP = common.HexToAddress("0x19018722")
	var tx1 = common.HexToHash("0x1234567890")
	var tx2 = common.HexToHash("0x0123456789")

	if s := ValidateCrossChainTx(db, MainChainToChildChain, accountZ, chainId, tx1); s != CrossChainTxNotFound {
		t.Fatalf("Non existent tx returned: %x", tx1)
	}

	if err := MarkCrossChainTx(db, MainChainToChildChain, accountZ, chainId, tx1, false); err != nil {
		t.Fatalf("Failed to mark tx into database: %x", tx1)
	}

	if err := MarkCrossChainTx(db, ChildChainToMainChain, accountP, chainId, tx2, false); err != nil {
		t.Fatalf("Failed to add tx into database: %x", tx2)
	}

	if s := ValidateCrossChainTx(db, MainChainToChildChain, accountZ, chainId, tx1); s != CrossChainTxReady {
		t.Fatalf("Failed to retrieve tx: %x", tx1)
	}

	if s := ValidateCrossChainTx(db, ChildChainToMainChain, accountZ, chainId, tx1); s != CrossChainTxNotFound {
		t.Fatalf("Non existent tx returned: %x", tx1)
	}

	if s := ValidateCrossChainTx(db, ChildChainToMainChain, accountP, chainId, tx2); s != CrossChainTxReady {
		t.Fatalf("Failed to retrieve tx: %x", tx2)
	}

	if s := ValidateCrossChainTx(db, MainChainToChildChain, accountP, chainId, tx2); s != CrossChainTxNotFound {
		t.Fatalf("Non existent tx returned: %x", tx2)
	}

	if err := MarkCrossChainTx(db, MainChainToChildChain, accountZ, chainId, tx2, true); err == nil {
		t.Fatalf("Mark non existent tx: %x as used", tx2)
	}

	if err := MarkCrossChainTx(db, MainChainToChildChain, accountP, chainId, tx1, true); err == nil {
		t.Fatalf("Mark non existent tx: %x as used", tx1)
	}

	if err := MarkCrossChainTx(db, MainChainToChildChain, accountZ, chainId, tx1, true); err != nil {
		t.Fatalf("Failed to makr tx: %x as used", tx1)
	}

	if s := ValidateCrossChainTx(db, MainChainToChildChain, accountZ, chainId, tx1); s != CrossChainTxAlreadyUsed {
		t.Fatalf("Non existent tx returned: %x", tx1)
	}

	if s := ValidateCrossChainTx(db, ChildChainToMainChain, accountP, chainId, tx2); s != CrossChainTxReady {
		t.Fatalf("Failed to retrieve tx: %x", tx2)
	}
}
