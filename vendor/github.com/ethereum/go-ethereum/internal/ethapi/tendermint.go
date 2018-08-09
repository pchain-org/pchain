package ethapi

import (
	"bytes"
	"fmt"
	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	st "github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/syndtr/goleveldb/leveldb/errors"
	"github.com/tendermint/go-crypto"
	"github.com/tendermint/tendermint/rpc/core/types"
	"golang.org/x/net/context"
	"math/big"
	"strings"
)

var (
	SvmFuncName string = "SendValidatorMessage"
)

const (
	VNEFuncName = "VoteNextEpoch"
	REVFuncName = "RevealVote"

	// Reveal Vote Parameters
	REV_ARGS_FROM    = "from"
	REV_ARGS_PUBKEY  = "pubkey"
	REV_ARGS_DEPOSIT = "amount"
	REV_ARGS_SALT    = "salt"

	SVM_JOIN     = "join"
	SVM_WITHDRAW = "withdraw"
	SVM_ACCEPT   = "accept"
)

type PublicTendermintAPI struct {
	am     *accounts.Manager
	b      Backend
	Client Client
}

// NewPublicEthereumAPI creates a new Etheruem protocol API.
func NewPublicTendermintAPI(b Backend) *PublicTendermintAPI {
	return &PublicTendermintAPI{
		am:     b.AccountManager(),
		b:      b,
		Client: b.Client(),
	}
}

// GasPrice returns a suggestion for a gas price.
func (s *PublicTendermintAPI) GetBlock(ctx context.Context, blockNumber rpc.BlockNumber) (interface{}, error) {

	var result core_types.TMResult

	//fmt.Printf("GetBlock() called with startBlock: %v\n", blockNumber)
	params := map[string]interface{}{
		"height": blockNumber,
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
		"number": number,
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
		"address": strings.ToLower(address),
		"epoch":   epoch,
	}

	_, err := s.Client.Call("validator_epoch", params, &result)
	if err != nil {
		fmt.Println(err)
		return "", err
	}

	//fmt.Printf("tdm_getValidator: %v\n", result)
	return result.(*core_types.ResultValidatorEpoch), nil
}

func (s *PublicTendermintAPI) VoteNextEpoch(ctx context.Context, from common.Address, voteHash common.Hash) (common.Hash, error) {

	params := types.MakeKeyValueSet()
	params.Set("from", from)
	params.Set("voteHash", voteHash)

	etd := &types.ExtendTxData{
		FuncName: VNEFuncName,
		Params:   params,
	}

	args := SendTxArgs{
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

	return s.b.GetInnerAPIBridge().SendTransaction(ctx, args)
}

func (s *PublicTendermintAPI) RevealVote(ctx context.Context, from common.Address, pubkey string, amount *big.Int, salt string) (common.Hash, error) {

	params := types.MakeKeyValueSet()
	params.Set(REV_ARGS_FROM, from)
	params.Set(REV_ARGS_PUBKEY, pubkey)
	params.Set(REV_ARGS_DEPOSIT, amount)
	params.Set(REV_ARGS_SALT, salt)

	etd := &types.ExtendTxData{
		FuncName: REVFuncName,
		Params:   params,
	}

	args := SendTxArgs{
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

	return s.b.GetInnerAPIBridge().SendTransaction(ctx, args)
}

func (s *PublicTendermintAPI) GetEpochVote(ctx context.Context) (interface{}, error) {

	var result core_types.TMResult

	_, err := s.Client.Call("epoch_votes", nil, &result)
	if err != nil {
		fmt.Println(err)
		return "", err
	}

	return result.(*core_types.ResultEpochVotes), nil
}

// Deprecated
func (s *PublicTendermintAPI) GetUnconfirmedValidatorsOperation(ctx context.Context) (interface{}, error) {

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

// Deprecated
func (s *PublicTendermintAPI) GetConfirmedValidatorsOperation(ctx context.Context, epoch uint64) (interface{}, error) {
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

// Deprecated
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

	etd := &types.ExtendTxData{
		FuncName: SvmFuncName,
		Params:   params,
	}

	args := SendTxArgs{
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
	core.RegisterValidateCb(REVFuncName, revealVote_ValidateCb)
	core.RegisterApplyCb(REVFuncName, revealVote_ApplyCb)
	//core.RegisterValidateCb(SvmFuncName, svm_ValidateCb)
	//core.RegisterApplyCb(SvmFuncName, svm_ApplyCb)
}

// Deprecated
func svm_ValidateCb(tx *types.Transaction, state *st.StateDB, cch core.CrossChainHelper) error {

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
// Deprecated
func svm_ApplyCb(tx *types.Transaction, state *st.StateDB, cch core.CrossChainHelper) error {

	fmt.Println("svm_ApplyCb")

	etd := tx.ExtendTxData()
	//tx from ethereum, the params have been converted to []byte
	fromInt, _ := etd.Params.Get("from")
	from := common.BytesToAddress(common.FromHex(string(fromInt.([]byte))))
	actionInt, _ := etd.Params.Get("action")
	action := string(actionInt.([]byte))
	powerInt, _ := etd.Params.Get("power")
	biPower := common.BytesToBig(powerInt.([]byte))

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

func revealVote_ValidateCb(tx *types.Transaction, state *st.StateDB, cch core.CrossChainHelper) error {
	etd := tx.ExtendTxData()

	fromVar, _ := etd.Params.Get(REV_ARGS_FROM)
	from := fromVar.(common.Address)
	pubkeyVar, _ := etd.Params.Get(REV_ARGS_PUBKEY)
	pubkey := pubkeyVar.(string)
	depositAmountVar, _ := etd.Params.Get(REV_ARGS_DEPOSIT)
	depositAmount := depositAmountVar.(*big.Int)

	// Check PubKey match the Address
	pubkeySlice := ethcrypto.FromECDSAPub(ethcrypto.ToECDSAPub(common.FromHex(pubkey)))
	if pubkeySlice == nil {
		return errors.New("your Public Key is not valid, please provide a valid Public Key")
	}

	validatorPubkey := crypto.EtherumPubKey(pubkeySlice)
	if !bytes.Equal(validatorPubkey.Address(), from.Bytes()) {
		return errors.New("your Public Key is not match with your Address, please provide a valid Public Key and Address")
	}

	// Check Balance (Available + Lock)
	total := new(big.Int).Add(state.GetBalance(from), state.GetLockedBalance(from))

	if total.Cmp(depositAmount) == -1 {
		return core.ErrBalance
	}
	return nil
}

func revealVote_ApplyCb(tx *types.Transaction, state *st.StateDB, cch core.CrossChainHelper) error {
	etd := tx.ExtendTxData()

	fromVar, _ := etd.Params.Get(REV_ARGS_FROM)
	from := common.BytesToAddress(fromVar.([]byte))
	depositAmountVar, _ := etd.Params.Get(REV_ARGS_DEPOSIT)
	depositAmount := new(big.Int).SetBytes(depositAmountVar.([]byte))

	// if lock balance less than deposit amount, then add enough amount to locked balance
	if state.GetLockedBalance(from).Cmp(depositAmount) == -1 {
		difference := new(big.Int).Sub(depositAmount, state.GetLockedBalance(from))
		if state.GetBalance(from).Cmp(difference) == -1 {
			return core.ErrBalance
		} else {
			state.SubBalance(from, difference)
			state.AddLockedBalance(from, difference)
		}
	}
	return nil
}
