package ethapi

import (
	"github.com/ethereum/go-ethereum/rpc"
	"golang.org/x/net/context"
	"fmt"
	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"strings"
	"github.com/syndtr/goleveldb/leveldb/errors"
	"github.com/tendermint/tendermint/rpc/core/types"
)

var (
	svm_ValidateCbName string = "svm_ValidateCb"
	svm_ApplyCbName string = "svm_ApplyCb"
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
func (s *PublicTendermintAPI) GetValidator(ctx context.Context, address string) (interface{}, error) {

	var result core_types.TMResult

	//fmt.Printf("GetValidator() called with address: %v\n", address)
	params := map[string]interface{}{
		"address":  address,
	}

	_, err := s.Client.Call("validator_epoch", params, &result)
	if err != nil {
		fmt.Println(err)
		return "", err
	}

	//fmt.Printf("tdm_getValidator: %v\n", result)
	return result.(*core_types.ResultValidatorEpoch), nil
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
	params := types.MakeKeyValueSet(5)
	params.Set("from", fromStr)
	params.Set("epoch", epoch)
	params.Set("power", power)
	params.Set("action", action)
	params.Set("hash", hash)

	etd := &types.ExtendTxData {
		FuncName:    "SendValidatorMessage",
		Params:      params,
		ValidateCb:  svm_ValidateCbName,
		ApplyCb:     svm_ApplyCbName,
	}

	args := SendTxArgs {
		From:         from,
		To:           nil,
		Gas:          nil,
		GasPrice:     nil,
		Value:        nil,
		Data:         nil,
		Nonce:        nil,
		Type:         nil,
		ExtendTxData: etd,
	}

	return ApiBridge.SendTransaction(ctx, args)
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

	types.RegisterValidateCb(svm_ValidateCbName, func() error{ fmt.Println("svm_ValidateCb"); return nil})
	types.RegisterApplyCb(svm_ApplyCbName, func() error { fmt.Println("svm_ApplyCb"); return nil})
}
