package ethapi

import (
	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common"
	"fmt"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	st "github.com/ethereum/go-ethereum/core/state"
	"golang.org/x/net/context"
	"github.com/pkg/errors"
	"strings"
	"math/big"
)

var (
	CCCFuncName string = "CreateChildChain"
	DIMCFuncName string = "DepositInMainChain"
	DICCFuncName string = "DepositInChildChain"
	WFCCFuncName string = "WithdrawFromChildChain"
	WFMCFuncName string = "WithdrawFromMainChain"
	SB2MCFuncName string = "SaveBlockToMainChain"
)


type PublicChainAPI struct {
	am *accounts.Manager
	b  Backend
	//Client Client
}

// NewPublicChainAPI creates a new Etheruem protocol API.
func NewPublicChainAPI(b Backend) *PublicChainAPI {
	return &PublicChainAPI{
		am: b.AccountManager(),
		b:  b,
		//Client: b.Client(),
	}
}

func (s *PublicChainAPI) CreateChildChain(ctx context.Context, from common.Address,
						chainId string) (common.Hash, error) {

	if chainId == "" || strings.Contains(chainId, ";") {
		return common.Hash{}, errors.New("chainId is nil or empty, or contains ';', should be meaningful")
	}

	fromStr := fmt.Sprintf("%X", from.Bytes())

	params := types.MakeKeyValueSet()
	params.Set("from", fromStr)
	params.Set("chainId", chainId)

	fmt.Printf("params are : %s\n", params.String())

	etd := &types.ExtendTxData {
		FuncName:    CCCFuncName,
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

func (s *PublicChainAPI) DepositInMainChain(ctx context.Context, from common.Address,
						chainId string, amount *big.Int) (common.Hash, error) {

	if chainId == "" || strings.Contains(chainId, ";") {
		return common.Hash{}, errors.New("chainId is nil or empty, or contains ';', should be meaningful")
	}

	if chainId == "pchain" {
		return common.Hash{}, errors.New("chainId should not be \"pchain\"")
	}

	if s.b.ChainConfig().PChainId != "pchain" {
		return common.Hash{}, errors.New("this api can only be called in main chain - pchain")
	}

	fromStr := fmt.Sprintf("%X", from.Bytes())

	params := types.MakeKeyValueSet()
	params.Set("from", fromStr)
	params.Set("chainId", chainId)
	params.Set("amount", amount)

	fmt.Printf("params are : %s\n", params.String())

	etd := &types.ExtendTxData {
		FuncName:    DIMCFuncName,
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

func (s *PublicChainAPI) DepositInChildChain(ctx context.Context, from common.Address,
						txHash common.Hash) (common.Hash, error) {

	chainId := s.b.ChainConfig().PChainId
	if chainId == "pchain" {
		return common.Hash{}, errors.New("this api can only be called in child chain")
	}

	cch := s.b.GetCrossChainHelper()
	mainTx := cch.GetTxFromMainChain(txHash)
	if mainTx == nil {
		return common.Hash{}, errors.New(fmt.Sprintf("tx %x does not exist in main chain", txHash))
	}

	mainEtd := mainTx.ExtendTxData()
	if mainEtd == nil || mainEtd.FuncName != DIMCFuncName {
		return common.Hash{}, errors.New(fmt.Sprintf("not expected tx %s", mainEtd))
	}

	mainFromInt, _ := mainEtd.Params.Get("from")
	mainFrom := common.BytesToAddress(common.FromHex(string(mainFromInt.([]byte))))
	mainChainIdInt, _ := mainEtd.Params.Get("chainId")
	mainChainId := string(mainChainIdInt.([]byte))

	if mainFromInt != mainFrom || mainChainId != chainId {
		return common.Hash{}, errors.New("params are not consistent with tx in main chain")
	}

	fromStr := fmt.Sprintf("%X", from.Bytes())

	params := types.MakeKeyValueSet()
	params.Set("from", fromStr)
	params.Set("txHash", txHash)

	fmt.Printf("params are : %s\n", params.String())

	etd := &types.ExtendTxData {
		FuncName:    DICCFuncName,
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

func (s *PublicChainAPI) WithdrawFromChildChain(ctx context.Context, from common.Address,
						amount *big.Int) (common.Hash, error) {

	chainId := s.b.ChainConfig().PChainId
	if chainId == "pchain" {
		return common.Hash{}, errors.New("this api can only be called in child chain")
	}

	fromStr := fmt.Sprintf("%X", from.Bytes())

	params := types.MakeKeyValueSet()
	params.Set("from", fromStr)
	params.Set("amount", amount)

	fmt.Printf("params are : %s\n", params.String())

	etd := &types.ExtendTxData {
		FuncName:    WFCCFuncName,
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

func (s *PublicChainAPI) WithdrawFromMainChain(ctx context.Context, from common.Address,
                                                chainId string, txHash common.Hash) (common.Hash, error) {

	if chainId == "pchain" {
		return common.Hash{}, errors.New("this api can only be called in main chain")
	}

	cch := s.b.GetCrossChainHelper()
	childTx := cch.GetTxFromChildChain(txHash, chainId)
	if childTx == nil {
		return common.Hash{}, errors.New(fmt.Sprintf("tx %x does not exist in child chain %s", txHash, chainId))
	}

	childEtd := childTx.ExtendTxData()
	if childEtd == nil || childEtd.FuncName != WFCCFuncName {
		return common.Hash{}, errors.New(fmt.Sprintf("not expected tx %s", childEtd))
	}

	childFromInt, _ := childEtd.Params.Get("from")
	childFrom := common.BytesToAddress(common.FromHex(string(childFromInt.([]byte))))

	if childFrom != from {
		return common.Hash{}, errors.New("params are not consistent with tx in main chain")
	}

	fromStr := fmt.Sprintf("%X", from.Bytes())

	params := types.MakeKeyValueSet()
	params.Set("from", fromStr)
	params.Set("chainId", chainId)
	params.Set("txHash", txHash)

	fmt.Printf("params are : %s\n", params.String())

	etd := &types.ExtendTxData {
		FuncName:    WFMCFuncName,
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

//here the parameter - 'block' is normal block in pchain with extra data in extra header
func (s *PublicChainAPI) SaveBlockToMainChain(ctx context.Context, from common.Address,
						block string) (common.Hash, error) {

	localChainId := s.b.ChainConfig().PChainId
	if localChainId != "pchain" {
		return common.Hash{}, errors.New("this api can only be called in main chain")
	}

	fromStr := fmt.Sprintf("%X", from.Bytes())

	params := types.MakeKeyValueSet()
	params.Set("from", fromStr)
	params.Set("block", block)

	fmt.Printf("params are : %s\n", params.String())

	etd := &types.ExtendTxData {
		FuncName:    SB2MCFuncName,
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

func init() {
	//CreateChildChain
	core.RegisterValidateCb(CCCFuncName, ccc_ValidateCb)
	core.RegisterApplyCb(CCCFuncName, ccc_ApplyCb)

	//DepositInMainChain
	core.RegisterValidateCb(DIMCFuncName, dimc_ValidateCb)
	core.RegisterApplyCb(DIMCFuncName, dimc_ApplyCb)

	//DepositInChildChain
	core.RegisterValidateCb(DICCFuncName, dicc_ValidateCb)
	core.RegisterApplyCb(DICCFuncName, dicc_ApplyCb)

	//WithdrawFromChildChain
	core.RegisterValidateCb(WFCCFuncName, wfcc_ValidateCb)
	core.RegisterApplyCb(WFCCFuncName, wfcc_ApplyCb)

	//WithdrawFromMainChain
	core.RegisterValidateCb(WFMCFuncName, wfmc_ValidateCb)
	core.RegisterApplyCb(WFMCFuncName, wfmc_ApplyCb)

	//SB2MCFuncName
	core.RegisterValidateCb(SB2MCFuncName, sb2mc_ValidateCb)
	core.RegisterApplyCb(SB2MCFuncName, sb2mc_ApplyCb)
}

func ccc_ValidateCb(tx *types.Transaction, state *st.StateDB, cch core.CrossChainHelper) error{

	fmt.Println("ccc_ValidateCb")

	etd := tx.ExtendTxData()
	fmt.Printf("params are : %s\n", etd.Params.String())

	//tx from ethereum, the params have not been converted to []byte
	fromInt, _ := etd.Params.Get("from")
	from := common.HexToAddress(fromInt.(string))
	chainIdInt, _ := etd.Params.Get("chainId")
	chainId := chainIdInt.(string)

	if chainId == "pchain" {
		return errors.New("chainId should not be \"pchain\"")
	}

	err := cch.CanCreateChildChain(from, chainId)
	if err != nil {
		return err
	}

	fmt.Printf("from is %X, childId is %s\n", from.Hex(), chainId)

	return nil
}

func ccc_ApplyCb(tx *types.Transaction, state *st.StateDB, cch core.CrossChainHelper) error{

	fmt.Println("ccc_ApplyCb")

	etd := tx.ExtendTxData()
	fmt.Printf("params are : %s\n", etd.Params.String())

	//tx from ethereum, the params have not been converted to []byte
	fromInt, _ := etd.Params.Get("from")
	from := common.BytesToAddress(common.FromHex(string(fromInt.([]byte))))
	chainIdInt, _ := etd.Params.Get("chainId")
	chainId := string(chainIdInt.([]byte))

	fmt.Printf("from is %X, childId is %s\n", from.Hex(), chainId)

	err := cch.CreateChildChain(from, chainId)
	if err != nil {return err}

	cch.GetTypeMutex().Post(core.CreateChildChainEvent{From:from, ChainId:chainId})

	return nil
}

func dimc_ValidateCb(tx *types.Transaction, state *st.StateDB, cch core.CrossChainHelper) error{

	fmt.Println("dimc_ValidateCb")

	etd := tx.ExtendTxData()
	fmt.Printf("params are : %s\n", etd.Params.String())

	//tx from ethereum, the params have not been converted to []byte
	fromInt, _ := etd.Params.Get("from")
	from := common.HexToAddress(fromInt.(string))
	chainIdInt, _ := etd.Params.Get("chainId")
	chainId := chainIdInt.(string)
	amountInt, _ := etd.Params.Get("amount")
	amount := amountInt.(*big.Int)

	if state.GetBalance(from).Cmp(amount) < 0 {
		return errors.New(fmt.Sprintf("%x has no enough balance for deposit", from))
	}

	fmt.Printf("from is %X, childId is %s\n", from.Hex(), chainId)

	return nil
}

func dimc_ApplyCb(tx *types.Transaction, state *st.StateDB, cch core.CrossChainHelper) error{

	fmt.Println("dimc_ApplyCb")

	etd := tx.ExtendTxData()
	fmt.Printf("params are : %s\n", etd.Params.String())

	//tx from ethereum, the params have not been converted to []byte
	fromInt, _ := etd.Params.Get("from")
	from := common.BytesToAddress(common.FromHex(string(fromInt.([]byte))))
	chainIdInt, _ := etd.Params.Get("chainId")
	chainId := string(chainIdInt.([]byte))
	amountInt, _ := etd.Params.Get("amount")
	amount := amountInt.(*big.Int)

	chainInfo := core.GetChainInfo(cch.GetChainInfoDB(), chainId)
	state.SubBalance(from, amount)
	state.AddChainBalance(chainInfo.Owner, amount)

	return nil
}

func dicc_ValidateCb(tx *types.Transaction, state *st.StateDB, cch core.CrossChainHelper) error{

	fmt.Println("dicc_ValidateCb")

	etd := tx.ExtendTxData()
	fmt.Printf("params are : %s\n", etd.Params.String())

	//tx from ethereum, the params have not been converted to []byte
	fromInt, _ := etd.Params.Get("from")
	from := common.HexToAddress(fromInt.(string))
	txHashInt, _ := etd.Params.Get("txHash")
	txHash := txHashInt.(common.Hash)

	fmt.Printf("from is %X, txHash is %x\n", from.Hex(), txHash)

	//has checked in DepositInChildChain()

	return nil
}

func dicc_ApplyCb(tx *types.Transaction, state *st.StateDB, cch core.CrossChainHelper) error{

	fmt.Println("dicc_ApplyCb")

	etd := tx.ExtendTxData()
	fmt.Printf("params are : %s\n", etd.Params.String())

	//tx from ethereum, the params have not been converted to []byte
	fromInt, _ := etd.Params.Get("from")
	from := common.HexToAddress(fromInt.(string))
	txHashInt, _ := etd.Params.Get("txHash")
	txHash := txHashInt.(common.Hash)

	fmt.Printf("from is %X, txHash is %s\n", from.Hex(), txHash)

	mainTx := cch.GetTxFromMainChain(txHash)
	if mainTx == nil {
		return errors.New(fmt.Sprintf("tx %x does not exist in main chain", txHash))
	}

	mainEtd := mainTx.ExtendTxData()
	if mainEtd == nil || mainEtd.FuncName != DIMCFuncName {
		return errors.New(fmt.Sprintf("not expected tx %s", mainEtd))
	}

	mainFromInt, _ := mainEtd.Params.Get("from")
	mainFrom := common.BytesToAddress(common.FromHex(string(mainFromInt.([]byte))))
	//mainChainIdInt, _ := mainEtd.Params.Get("chainId")
	//mainChainId := string(mainChainIdInt.([]byte))
	mainAmountInt, _ := mainEtd.Params.Get("amount")
	mainAmount := mainAmountInt.(*big.Int)

	state.AddBalance(mainFrom, mainAmount)

	return nil
}

func wfcc_ValidateCb(tx *types.Transaction, state *st.StateDB, cch core.CrossChainHelper) error{

	fmt.Println("wfcc_ValidateCb")

	etd := tx.ExtendTxData()
	fmt.Printf("params are : %s\n", etd.Params.String())

	//tx from ethereum, the params have not been converted to []byte
	fromInt, _ := etd.Params.Get("from")
	from := common.HexToAddress(fromInt.(string))
	amountInt, _ := etd.Params.Get("amount")
	amount := amountInt.(*big.Int)

	fmt.Printf("from is %X, amount is %v\n", from.Hex(), amount)

	if state.GetBalance(from).Cmp(amount) < 0 {
		return errors.New("no enough balance to withdraw")
	}

	return nil
}

func wfcc_ApplyCb(tx *types.Transaction, state *st.StateDB, cch core.CrossChainHelper) error{

	fmt.Println("wfcc_ApplyCb")

	etd := tx.ExtendTxData()
	fmt.Printf("params are : %s\n", etd.Params.String())

	//tx from ethereum, the params have not been converted to []byte
	fromInt, _ := etd.Params.Get("from")
	from := common.HexToAddress(fromInt.(string))
	amountInt, _ := etd.Params.Get("amount")
	amount := amountInt.(*big.Int)

	fmt.Printf("from is %X, amount is %v\n", from.Hex(), amount)

	if state.GetBalance(from).Cmp(amount) < 0 {
		return errors.New("no enough balance to withdraw")
	}

	state.SubBalance(from, amount)

	return nil
}

func wfmc_ValidateCb(tx *types.Transaction, state *st.StateDB, cch core.CrossChainHelper) error{

	fmt.Println("wfmc_ValidateCb")

	etd := tx.ExtendTxData()
	fmt.Printf("params are : %s\n", etd.Params.String())

	//tx from ethereum, the params have not been converted to []byte
	fromInt, _ := etd.Params.Get("from")
	from := common.HexToAddress(fromInt.(string))
	chainIdInt, _ := etd.Params.Get("chainId")
	chainId := chainIdInt.(string)
	txHashInt, _ := etd.Params.Get("txHash")
	txHash := txHashInt.(common.Hash)

	fmt.Printf("from is %X, txHash is %x, chainId is %s\n", from.Hex(), txHash, chainId)

	childTx := cch.GetTxFromChildChain(txHash, chainId)
	if childTx == nil {
		return errors.New(fmt.Sprintf("tx %x does not exist in child chain %s", txHash, chainId))
	}

	childEtd := childTx.ExtendTxData()
	if childEtd == nil || childEtd.FuncName != WFCCFuncName {
		return errors.New(fmt.Sprintf("not expected tx %s", childEtd))
	}

	childFromInt, _ := childEtd.Params.Get("from")
	childFrom := common.BytesToAddress(common.FromHex(string(childFromInt.([]byte))))
	childAmountInt, _ := childEtd.Params.Get("amount")
	childAmount := childAmountInt.(*big.Int)

	if childFrom != from {
		return errors.New("params are not consistent with tx in main chain")
	}

	chainInfo := core.GetChainInfo(cch.GetChainInfoDB(), chainId)
	if state.GetChainBalance(chainInfo.Owner).Cmp(childAmount) < 0 {
		return errors.New("no enough balance to withdraw")
	}

	return nil
}

func wfmc_ApplyCb(tx *types.Transaction, state *st.StateDB, cch core.CrossChainHelper) error{

	fmt.Println("wfmc_ApplyCb")

	etd := tx.ExtendTxData()
	fmt.Printf("params are : %s\n", etd.Params.String())

	//tx from ethereum, the params have not been converted to []byte
	fromInt, _ := etd.Params.Get("from")
	from := common.HexToAddress(fromInt.(string))
	chainIdInt, _ := etd.Params.Get("chainId")
	chainId := chainIdInt.(string)
	txHashInt, _ := etd.Params.Get("txHash")
	txHash := txHashInt.(common.Hash)

	fmt.Printf("from is %X, txHash is %x, chainId is %s\n", from.Hex(), txHash, chainId)

	childTx := cch.GetTxFromChildChain(txHash, chainId)
	if childTx == nil {
		return errors.New(fmt.Sprintf("tx %x does not exist in child chain %s", txHash, chainId))
	}

	childEtd := childTx.ExtendTxData()
	if childEtd == nil || childEtd.FuncName != WFCCFuncName {
		return errors.New(fmt.Sprintf("not expected tx %s", childEtd))
	}

	childFromInt, _ := childEtd.Params.Get("from")
	childFrom := common.BytesToAddress(common.FromHex(string(childFromInt.([]byte))))
	childAmountInt, _ := childEtd.Params.Get("amount")
	childAmount := childAmountInt.(*big.Int)

	if childFrom != from {
		return errors.New("params are not consistent with tx in main chain")
	}

	chainInfo := core.GetChainInfo(cch.GetChainInfoDB(), chainId)
	chainOwner := chainInfo.Owner
	if state.GetChainBalance(chainOwner).Cmp(childAmount) < 0 {
		return errors.New("no enough balance to withdraw")
	}

	state.SubChainBalance(chainOwner, childAmount)
	state.AddBalance(from, childAmount)

	return nil
}

func sb2mc_ValidateCb(tx *types.Transaction, state *st.StateDB, cch core.CrossChainHelper) error{

	fmt.Println("sb2mc_ValidateCb")

	etd := tx.ExtendTxData()
	fmt.Printf("params are : %s\n", etd.Params.String())

	//tx from ethereum, the params have not been converted to []byte
	fromInt, _ := etd.Params.Get("from")
	from := common.HexToAddress(fromInt.(string))
	blockInt, _ := etd.Params.Get("block")
	block := blockInt.(string)

	fmt.Printf("from is %X, txHash is %x, block is %s\n", from.Hex(), block)

	err := cch.VerifyBlock(from, block)
	if err != nil {
		return errors.New(fmt.Sprintf("block does not pass verfication", from, block))
	}

	return nil
}

func sb2mc_ApplyCb(tx *types.Transaction, state *st.StateDB, cch core.CrossChainHelper) error{

	fmt.Println("sb2mc_ApplyCb")

	etd := tx.ExtendTxData()
	fmt.Printf("params are : %s\n", etd.Params.String())

	//tx from ethereum, the params have not been converted to []byte
	fromInt, _ := etd.Params.Get("from")
	from := common.HexToAddress(fromInt.(string))
	blockInt, _ := etd.Params.Get("block")
	block := blockInt.(string)

	fmt.Printf("from is %X, txHash is %x, block is %s\n", from.Hex(), block)

	return cch.SaveBlock2MainBlock(block)
}
