package ethapi

import (
	"fmt"
	"errors"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	st "github.com/ethereum/go-ethereum/core/state"
	"math/big"
)

//this module records all apis called from tendermint side

var UnlockAssertFuncName = "UnlockAssert"

func init() {

	//UnlockAssertFuncName
	core.RegisterApplyCb(UnlockAssertFuncName, UnlockAssert_ApplyCb)
}

func UnlockAssert_ApplyCb(tx *types.Transaction, state *st.StateDB, cch core.CrossChainHelper) error{

	fmt.Println("UnlockAssert_ApplyCb")

	etd := tx.ExtendTxData()
	//tx from ethereum, the params have been converted to []byte
	//senderInt, _ := etd.Params.Get("sender")
	//sender := common.BytesToAddress(common.FromHex(string(senderInt.([]byte))))
	accountInt, _ := etd.Params.Get("account")
	account := common.BytesToAddress(common.FromHex(string(accountInt.([]byte))))
	amountInt, _ := etd.Params.Get("amount")
	biAmount := new(big.Int).SetBytes(amountInt.([]byte))

	fmt.Printf("balance for(%s) is : (%v, %v), biPower is %v\n",
		account.Hex(), state.GetBalance(account), state.GetLockedBalance(account), biAmount.String())

	if state.GetLockedBalance(account).Cmp(biAmount) < 0 {
		return errors.New("locked balance is smaller than withdrawing amount")
	} else {
		state.AddBalance(account, biAmount)
		state.SubLockedBalance(account, biAmount)
	}

	fmt.Printf("balance for(%s) is : (%v, %v), biPower is %v\n",
		account.Hex(), state.GetBalance(account), state.GetLockedBalance(account), biAmount.String())

	return nil
}
