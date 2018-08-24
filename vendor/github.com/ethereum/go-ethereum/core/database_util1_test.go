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

	if ok := HasCrossChainTx(db, MainChainToChildChain, chainId, accountZ, tx1); ok {
		t.Fatalf("Non existent tx returned: %x", tx1)
	}

	if err := AddCrossChainTx(db, MainChainToChildChain, chainId, accountZ, tx1); err != nil {
		t.Fatalf("Failed to add tx into database: %x", tx1)
	}

	if err := AddCrossChainTx(db, ChildChainToMainChain, chainId, accountP, tx2); err != nil {
		t.Fatalf("Failed to add tx into database: %x", tx2)
	}

	if ok := HasCrossChainTx(db, MainChainToChildChain, chainId, accountZ, tx1); !ok {
		t.Fatalf("Failed to retrieve tx: %x", tx1)
	}

	if ok := HasCrossChainTx(db, ChildChainToMainChain, chainId, accountZ, tx1); ok {
		t.Fatalf("Non existent tx returned: %x", tx1)
	}

	if ok := HasCrossChainTx(db, ChildChainToMainChain, chainId, accountP, tx2); !ok {
		t.Fatalf("Failed to retrieve tx: %x", tx2)
	}

	if ok := HasCrossChainTx(db, MainChainToChildChain, chainId, accountP, tx2); ok {
		t.Fatalf("Non existent tx returned: %x", tx2)
	}

	if err := RemoveCrossChainTx(db, MainChainToChildChain, chainId, accountZ, tx2); err == nil {
		t.Fatalf("Remove non existent tx: %x", tx2)
	}

	if err := RemoveCrossChainTx(db, MainChainToChildChain, chainId, accountP, tx1); err == nil {
		t.Fatalf("Remove non existent tx: %x", tx1)
	}

	if err := RemoveCrossChainTx(db, MainChainToChildChain, chainId, accountZ, tx1); err != nil {
		t.Fatalf("Failed to remove tx: %x", tx1)
	}

	if ok := HasCrossChainTx(db, MainChainToChildChain, chainId, accountZ, tx1); ok {
		t.Fatalf("Non existent tx returned: %x", tx1)
	}

	if ok := HasCrossChainTx(db, ChildChainToMainChain, chainId, accountP, tx2); !ok {
		t.Fatalf("Failed to retrieve tx: %x", tx2)
	}
}
