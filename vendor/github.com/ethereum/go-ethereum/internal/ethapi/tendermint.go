package ethapi

import (
	"github.com/ethereum/go-ethereum/rpc"
	"golang.org/x/net/context"
	"fmt"
	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	st "github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core"
	"strings"
	"github.com/syndtr/goleveldb/leveldb/errors"
	"math/big"
	"github.com/tendermint/tendermint/rpc/core/types"
)

var (
	SvmFuncName string = "SendValidatorMessage"
)

const (
	SVM_JOIN = "join"
	SVM_WITHDRAW = "withdraw"
	SVM_ACCEPT = "accept"
)

type PublicTendermintAPI struct {
	am *accounts.Manager
	b  Backend
	Client Client
}

// NewPublicEthereumAPI creates a new Etheruem protocol API.
func NewPublicTendermintAPI(b Backend) *PublicTendermintAPI {
	return &PublicTendermintAPI{
		am: b.AccountManager(),
		b:  b,
		Client: b.Client(),
	}
}

// GasPrice returns a suggestion for a gas price.
func (s *PublicTendermintAPI) GetBlock(ctx context.Context, blockNumber rpc.BlockNumber) (interface{}, error) {

	var result core_types.TMResult

	//fmt.Printf("GetBlock() called with startBlock: %v\n", blockNumber)
	params := map[string]interface{}{
		"height":  blockNumber,
	}

	_, err := s.Client.Call("block", params, &result)
	if err != nil {
		fmt.Println(err)
		return "", err
	}

	//fmt.Printf("tdm_getBlock: %v\n", result)
	return result.(*core_types.ResultBlock).Block, nil
}

// GasPrice returns a suggestion for a gas price.
func (s *PublicTendermintAPI) GetCurrentEpochNumber(ctx context.Context) (int64, error) {

	var result core_types.TMResult

	_, err := s.Client.Call("current_epochnumber", nil, &result)
	if err != nil {
		fmt.Println(err)
		return -1, err
	}

	//fmt.Printf("tdm_getBlock: %v\n", result)
	return int64(result.(*core_types.ResultUint64).Value), nil
}

func (s *PublicTendermintAPI) GetEpoch(ctx context.Context, number uint64) (interface{}, error) {

	var result core_types.TMResult

	//fmt.Printf("GetEpoch() called with address: %v\n", address)
	params := map[string]interface{}{
		"number":  number,
	}
	_, err := s.Client.Call("epoch", params, &result)
	if err != nil {
		fmt.Println(err)
		return -1, err
	}

	//fmt.Printf("tdm_getBlock: %v\n", result)
	return result.(*core_types.ResultEpoch), nil
}

// GasPrice returns a suggestion for a gas price.
func (s *PublicTendermintAPI) GetValidator(ctx context.Context, address string, epoch uint64) (interface{}, error) {

	var result core_types.TMResult

	//fmt.Printf("GetValidator() called with address: %v\n", address)
	params := map[string]interface{}{
		"address":  strings.ToLower(address),
		"epoch": epoch,
	}

	_, err := s.Client.Call("validator_epoch", params, &result)
	if err != nil {
		fmt.Println(err)
		return "", err
	}

	//fmt.Printf("tdm_getValidator: %v\n", result)
	return result.(*core_types.ResultValidatorEpoch), nil
}
func (s *PublicTendermintAPI) GetUnconfirmedValidatorsOperation(ctx context.Context) (interface{}, error){

	var result core_types.TMResult

	//fmt.Printf("GetUnconfirmedValidatorsOperation() called\n")

	_, err := s.Client.Call("unconfirmed_vo", nil, &result)
	if err != nil {
		fmt.Println(err)
		return "", err
	}

	//fmt.Printf("tdm_getValidator: %v\n", result)
	return result.(*core_types.ResultValidatorsOperation), nil
}

func (s *PublicTendermintAPI) GetConfirmedValidatorsOperation(ctx context.Context, epoch uint64) (interface{}, error){
	var result core_types.TMResult

	//fmt.Printf("GetValidator() called with address: %v\n", address)
	params := map[string]interface{}{
		"epoch": epoch,
	}

	_, err := s.Client.Call("confirmed_vo", params, &result)
	if err != nil {
		fmt.Println(err)
		return "", err
	}

	//fmt.Printf("tdm_getValidator: %v\n", result)
	return result.(*core_types.ResultValidatorsOperation), nil
}

func (s *PublicTendermintAPI) SendValidatorMessage(ctx context.Context, from common.Address, epoch uint64, power uint64,
						action string, hash string) (common.Hash, error) {
	fmt.Println("in func (s *PublicTendermintAPI) SendValidatorMessage()")

	action = strings.ToLower(action)
	if action != SVM_JOIN && action != SVM_WITHDRAW && action != SVM_ACCEPT {
		return common.Hash{}, errors.New("action should be {join|withdraw|accept}")
	}

	//targetStr := target
	//targetAddr := common.Address{}
	if action == SVM_ACCEPT {
		if hash == "" {
			return common.Hash{}, errors.New("parameter hash can not be empty or zero-length")
		}
		//targetAddr = common.HexToAddress(target)
		//targetStr = fmt.Sprintf("%X", targetAddr.Bytes())
	}

	fromStr := fmt.Sprintf("%X", from.Bytes())
	/*
	data := fmt.Sprintf("%s-%X-%X-%s-%s", fromStr, epoch, power, action, targetStr)
	fmt.Printf("in func (s *PublicTendermintAPI) SendValidatorMessage(), data to sign is: %v\n", data)

	signature, err := s.Sign(ctx, from, data)
	if err != nil {
		return common.Hash{}, err
	}
	*/
	/*params := map[string]interface{}{
		"from": fromStr,
		"epoch":  epoch,
		"power":  power,
		"action":   action,
		"target":   target,
	}
	*/
	params := types.MakeKeyValueSet()
	params.Set("from", fromStr)
	params.Set("epoch", epoch)
	params.Set("power", power)
	params.Set("action", action)
	params.Set("hash", hash)

	fmt.Printf("params are : %s\n", params.String())

	etd := &types.ExtendTxData {
		FuncName:    SvmFuncName,
		Params:      params,
	}

	args := SendTxArgs {
		From:         from,
		To:           nil,
		Gas:          nil,
		GasPrice:     nil,
		Value:        nil,
		Data:         nil,
		Nonce:        nil,
		ExtendTxData: etd,
	}

	return s.b.GetInnerAPIBridge().SendTransaction(ctx, args)
}

// SendTransaction creates a transaction for the given argument, sign it and submit it to the
// transaction pool.
/*
func (s *PublicTendermintAPI) Sign(ctx context.Context, from common.Address, dataStr string) ([]byte, error) {

	fmt.Printf("(s *PublicTendermintAPI) SignAndVerify(), from: %x, data: %v\n", from, dataStr)

	//fmt.Printf("(s *PublicTransactionPoolAPI) SendTransaction(), s.b.GetSendTxMutex() is %v\n", s.b.GetSendTxMutex())

	s.b.GetSendTxMutex().Lock()
	defer s.b.GetSendTxMutex().Unlock()

	data := []byte(dataStr)

	// Look up the wallet containing the requested signer
	account := accounts.Account{Address: from}
	//fmt.Println("(s *PublicBlockChainAPI) SendTransaction() 1")
	wallet, err := s.b.AccountManager().Find(account)
	if err != nil {
		return []byte{}, err
	}

	sig, err := wallet.SignHash(account, signHash(data))
	if err != nil {
		return []byte{}, err
	}
	sig[64] += 27

	fmt.Printf("SignAndVerify(), sig is: %X\n", sig)

	return sig, nil
}
*/

func init() {

	core.RegisterValidateCb(SvmFuncName, svm_ValidateCb)
	core.RegisterApplyCb(SvmFuncName, svm_ApplyCb)
}


func svm_ValidateCb(tx *types.Transaction, state *st.StateDB, cch core.CrossChainHelper) error{

	fmt.Println("svm_ValidateCb")

	etd := tx.ExtendTxData()
	fmt.Printf("params are : %s\n", etd.Params.String())

	//tx from ethereum, the params have not been converted to []byte
	fromInt, _ := etd.Params.Get("from")
	from := common.HexToAddress(fromInt.(string))
	actionInt, _ := etd.Params.Get("action")
	action := actionInt.(string)
	powerInt, _ := etd.Params.Get("power")
	power := powerInt.(uint64)
	biPower := big.NewInt(int64(power))
	fmt.Printf("balance for(%s) is : (%v, %v), biPower is %v\n",
		from.Hex(), state.GetBalance(from), state.GetLockedBalance(from), biPower.String())
	if action == SVM_JOIN {
		if state.GetBalance(from).Cmp(biPower) < 0 {
			return errors.New("balance is insufficient for voting power")
		}
	}
	if action == SVM_WITHDRAW {
		if state.GetLockedBalance(from).Cmp(biPower) < 0 {
			return errors.New("locked balance is smaller than withdrawing amount")
		}
	}
	fmt.Printf("balance for(%s) is : (%v, %v), biPower is %v\n",
		from.Hex(), state.GetBalance(from), state.GetLockedBalance(from), biPower.String())

	return nil
}


/*must not handle SVM_WITHDRAW here, need wait for 2 more epoch*/
func svm_ApplyCb(tx *types.Transaction, state *st.StateDB, cch core.CrossChainHelper) error{

	fmt.Println("svm_ApplyCb")

	etd := tx.ExtendTxData()
	//tx from ethereum, the params have been converted to []byte
	fromInt, _ := etd.Params.Get("from")
	from := common.BytesToAddress(common.FromHex(string(fromInt.([]byte))))
	actionInt, _ := etd.Params.Get("action")
	action := string(actionInt.([]byte))
	powerInt, _ := etd.Params.Get("power")
	biPower := new(big.Int).SetBytes(powerInt.([]byte))

	fmt.Printf("balance for(%s) is : (%v, %v), biPower is %v\n",
		from.Hex(), state.GetBalance(from), state.GetLockedBalance(from), biPower.String())
	if action == SVM_JOIN {
		if state.GetBalance(from).Cmp(biPower) < 0 {
			return errors.New("balance is insufficient for voting power")
		} else {

			state.SubBalance(from, biPower)
			state.AddLockedBalance(from, biPower)
		}
	}

	fmt.Printf("balance for(%s) is : (%v, %v), biPower is %v\n",
		from.Hex(), state.GetBalance(from), state.GetLockedBalance(from), biPower.String())

	return nil
}
