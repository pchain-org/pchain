package ethapi

import (
	"encoding/binary"
	"fmt"
	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	st "github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
	"math/big"
	"strings"
)

const (
	CCCFuncName   = "CreateChildChain"
	JCCFuncName   = "JoinChildChain"
	DIMCFuncName  = "DepositInMainChain"
	DICCFuncName  = "DepositInChildChain"
	WFCCFuncName  = "WithdrawFromChildChain"
	WFMCFuncName  = "WithdrawFromMainChain"
	SB2MCFuncName = "SaveBlockToMainChain"

	// Create Child Chain Parameters
	CCC_ARGS_FROM                = "from"
	CCC_ARGS_CHAINID             = "chainId"
	CCC_ARGS_VALIDATOR_THRESHOLD = "validatorThreshold"
	CCC_ARGS_TOKEN_THRESHOLD     = "tokenThreshold"
	CCC_ARGS_START_BLOCK         = "startBlock"
	CCC_ARGS_END_BLOCK           = "endBlock"

	// Join Child Chain Parameters
	JCC_ARGS_FROM    = "from"
	JCC_ARGS_CHAINID = "chainId"
	JCC_ARGS_DEPOSIT = "depositAmount"
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

func (s *PublicChainAPI) CreateChildChain(ctx context.Context, from common.Address, chainId string,
	minValidators uint16, minDepositAmount *big.Int, startBlock, endBlock uint64) (common.Hash, error) {

	if chainId == "" || strings.Contains(chainId, ";") {
		return common.Hash{}, errors.New("chainId is nil or empty, or contains ';', should be meaningful")
	}

	params := types.MakeKeyValueSet()
	params.Set(CCC_ARGS_FROM, from)
	params.Set(CCC_ARGS_CHAINID, chainId)
	params.Set(CCC_ARGS_VALIDATOR_THRESHOLD, minValidators)
	params.Set(CCC_ARGS_TOKEN_THRESHOLD, minDepositAmount)
	params.Set(CCC_ARGS_START_BLOCK, startBlock)
	params.Set(CCC_ARGS_END_BLOCK, endBlock)

	etd := &types.ExtendTxData{
		FuncName: CCCFuncName,
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

func (s *PublicChainAPI) JoinChildChain(ctx context.Context, from common.Address, chainId string, depositAmount *big.Int) (common.Hash, error) {

	if chainId == "" || strings.Contains(chainId, ";") {
		return common.Hash{}, errors.New("chainId is nil or empty, or contains ';', should be meaningful")
	}

	params := types.MakeKeyValueSet()
	params.Set(JCC_ARGS_FROM, from)
	params.Set(JCC_ARGS_CHAINID, chainId)
	params.Set(JCC_ARGS_DEPOSIT, depositAmount)

	etd := &types.ExtendTxData{
		FuncName: JCCFuncName,
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

	etd := &types.ExtendTxData{
		FuncName: DIMCFuncName,
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

	etd := &types.ExtendTxData{
		FuncName: DICCFuncName,
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

	etd := &types.ExtendTxData{
		FuncName: WFCCFuncName,
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

	etd := &types.ExtendTxData{
		FuncName: WFMCFuncName,
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

	etd := &types.ExtendTxData{
		FuncName: SB2MCFuncName,
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

func init() {
	//CreateChildChain
	core.RegisterValidateCb(CCCFuncName, ccc_ValidateCb)
	core.RegisterApplyCb(CCCFuncName, ccc_ApplyCb)

	//JoinChildChain
	core.RegisterValidateCb(JCCFuncName, jcc_ValidateCb)
	core.RegisterApplyCb(JCCFuncName, jcc_ApplyCb)

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

func ccc_ValidateCb(tx *types.Transaction, state *st.StateDB, cch core.CrossChainHelper) error {

	fmt.Println("ccc_ValidateCb")

	etd := tx.ExtendTxData()
	fmt.Printf("params are : %s\n", etd.Params.String())

	//tx from ethereum, the params have not been converted to []byte
	fromVar, _ := etd.Params.Get(CCC_ARGS_FROM)
	from := fromVar.(common.Address)
	chainIdVar, _ := etd.Params.Get(CCC_ARGS_CHAINID)
	chainId := chainIdVar.(string)
	minValidatorsVar, _ := etd.Params.Get(CCC_ARGS_VALIDATOR_THRESHOLD)
	minValidators := minValidatorsVar.(uint16)
	minDepositAmountVar, _ := etd.Params.Get(CCC_ARGS_TOKEN_THRESHOLD)
	minDepositAmount := minDepositAmountVar.(*big.Int)
	startBlockVar, _ := etd.Params.Get(CCC_ARGS_START_BLOCK)
	startBlock := startBlockVar.(uint64)
	endBlockVar, _ := etd.Params.Get(CCC_ARGS_END_BLOCK)
	endBlock := endBlockVar.(uint64)

	err := cch.CanCreateChildChain(from, chainId, minValidators, minDepositAmount, startBlock, endBlock)
	if err != nil {
		return err
	}

	fmt.Printf("from is %X, childId is %s\n", from.Hex(), chainId)

	return nil
}

func ccc_ApplyCb(tx *types.Transaction, state *st.StateDB, cch core.CrossChainHelper) error {

	fmt.Println("ccc_ApplyCb")

	etd := tx.ExtendTxData()

	//tx from ethereum, the params have not been converted to []byte
	fromVar, _ := etd.Params.Get(CCC_ARGS_FROM)
	from := common.BytesToAddress(fromVar.([]byte))
	chainIdVar, _ := etd.Params.Get(CCC_ARGS_CHAINID)
	chainId := string(chainIdVar.([]byte))

	minValidatorsVar, _ := etd.Params.Get(CCC_ARGS_VALIDATOR_THRESHOLD)
	minValidators := convertByteSliceToUint16(minValidatorsVar.([]byte))

	minDepositAmountVar, _ := etd.Params.Get(CCC_ARGS_TOKEN_THRESHOLD)
	minDepositAmount := new(big.Int).SetBytes(minDepositAmountVar.([]byte))

	startBlockVar, _ := etd.Params.Get(CCC_ARGS_START_BLOCK)
	startBlock := convertByteSliceToUint64(startBlockVar.([]byte))

	endBlockVar, _ := etd.Params.Get(CCC_ARGS_END_BLOCK)
	endBlock := convertByteSliceToUint64(endBlockVar.([]byte))

	err := cch.CreateChildChain(from, chainId, minValidators, minDepositAmount, startBlock, endBlock)
	if err != nil {
		return err
	}

	// TODO Move to apply commit callback
	//cch.GetTypeMutex().Post(core.CreateChildChainEvent{From: from, ChainId: chainId})

	return nil
}

func jcc_ValidateCb(tx *types.Transaction, state *st.StateDB, cch core.CrossChainHelper) error {

	etd := tx.ExtendTxData()

	fromVar, _ := etd.Params.Get(JCC_ARGS_FROM)
	from := fromVar.(common.Address)
	chainIdVar, _ := etd.Params.Get(JCC_ARGS_CHAINID)
	chainId := chainIdVar.(string)
	depositAmountVar, _ := etd.Params.Get(JCC_ARGS_DEPOSIT)
	depositAmount := depositAmountVar.(*big.Int)

	if err := cch.ValidateJoinChildChain(from, chainId, depositAmount); err != nil {
		return err
	}

	return nil
}

func jcc_ApplyCb(tx *types.Transaction, state *st.StateDB, cch core.CrossChainHelper) error {

	etd := tx.ExtendTxData()

	fromVar, _ := etd.Params.Get(JCC_ARGS_FROM)
	from := common.BytesToAddress(fromVar.([]byte))
	chainIdVar, _ := etd.Params.Get(JCC_ARGS_CHAINID)
	chainId := string(chainIdVar.([]byte))

	depositAmountVar, _ := etd.Params.Get(JCC_ARGS_DEPOSIT)
	depositAmount := new(big.Int).SetBytes(depositAmountVar.([]byte))

	err := cch.JoinChildChain(from, chainId, depositAmount)
	if err != nil {
		return err
	}

	return nil
}

func dimc_ValidateCb(tx *types.Transaction, state *st.StateDB, cch core.CrossChainHelper) error {

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

func dimc_ApplyCb(tx *types.Transaction, state *st.StateDB, cch core.CrossChainHelper) error {

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

func dicc_ValidateCb(tx *types.Transaction, state *st.StateDB, cch core.CrossChainHelper) error {

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

func dicc_ApplyCb(tx *types.Transaction, state *st.StateDB, cch core.CrossChainHelper) error {

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

func wfcc_ValidateCb(tx *types.Transaction, state *st.StateDB, cch core.CrossChainHelper) error {

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

func wfcc_ApplyCb(tx *types.Transaction, state *st.StateDB, cch core.CrossChainHelper) error {

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

func wfmc_ValidateCb(tx *types.Transaction, state *st.StateDB, cch core.CrossChainHelper) error {

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

func wfmc_ApplyCb(tx *types.Transaction, state *st.StateDB, cch core.CrossChainHelper) error {

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

func sb2mc_ValidateCb(tx *types.Transaction, state *st.StateDB, cch core.CrossChainHelper) error {

	fmt.Println("sb2mc_ValidateCb")

	etd := tx.ExtendTxData()
	fmt.Printf("params are : %s\n", etd.Params.String())

	//tx from ethereum, the params have not been converted to []byte
	fromInt, _ := etd.Params.Get("from")
	from := common.HexToAddress(fromInt.(string))
	blockInt, _ := etd.Params.Get("block")
	block := blockInt.(string)

	fmt.Printf("from is %X, txHash is %x, block is %s\n", from.Hex(), block)

	err := cch.VerifyTdmBlock(from, block)
	if err != nil {
		return errors.New(fmt.Sprintf("block does not pass verfication", from, block))
	}

	return nil
}

func sb2mc_ApplyCb(tx *types.Transaction, state *st.StateDB, cch core.CrossChainHelper) error {

	fmt.Println("sb2mc_ApplyCb")

	etd := tx.ExtendTxData()
	fmt.Printf("params are : %s\n", etd.Params.String())

	//tx from ethereum, the params have not been converted to []byte
	fromInt, _ := etd.Params.Get("from")
	from := common.HexToAddress(fromInt.(string))
	blockInt, _ := etd.Params.Get("block")
	block := blockInt.(string)

	fmt.Printf("from is %X, txHash is %x, block is %s\n", from.Hex(), block)

	return cch.SaveTdmBlock2MainBlock(block)
}

// ---------------------------------------------
// Utility Func
const (
	UINT64_BYTE_SIZE = 8
	UINT16_BYTE_SIZE = 2
)

// Convert the Byte Slice to uint64
func convertByteSliceToUint64(input []byte) uint64 {
	result := make([]byte, UINT64_BYTE_SIZE)

	l := UINT64_BYTE_SIZE - len(input)
	copy(result[l:], input)

	return binary.BigEndian.Uint64(result)
}

// Convert the Byte Slice to uint16
func convertByteSliceToUint16(input []byte) uint16 {
	result := make([]byte, UINT16_BYTE_SIZE)

	l := UINT16_BYTE_SIZE - len(input)
	copy(result[l:], input)

	return binary.BigEndian.Uint16(result)
}
