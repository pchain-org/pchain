package ethapi

import (
	"context"
	"fmt"
	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/pkg/errors"
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
	JCC_ARGS_PUBKEY  = "pubkey"
	JCC_ARGS_CHAINID = "chainId"
	JCC_ARGS_DEPOSIT = "depositAmount"

	// Deposit In Main Chain Parameters
	DIMC_ARGS_FROM    = "from"
	DIMC_ARGS_CHAINID = "chainId"
	DIMC_ARGS_AMOUNT  = "amount"

	// Deposit In Child Chain Parameters
	DICC_ARGS_FROM    = "from"
	DICC_ARGS_CHAINID = "chainId"
	DICC_ARGS_TXHASH  = "txHash"

	// Withdraw From Child Chain Parameters
	WFCC_ARGS_FROM    = "from"
	WFCC_ARGS_CHAINID = "chainId"
	WFCC_ARGS_AMOUNT  = "amount"

	// Withdraw From Main Chain Parameters
	WFMC_ARGS_FROM    = "from"
	WFMC_ARGS_CHAINID = "chainId"
	WFMC_ARGS_TXHASH  = "txHash"

	// Save Block To Main Chain Parameters
	SB2MCFuncName_ARGS_FROM = "from"
)

type PublicChainAPI struct {
	am *accounts.Manager
	b  Backend
}

// NewPublicChainAPI creates a new Etheruem protocol API.
func NewPublicChainAPI(b Backend) *PublicChainAPI {
	return &PublicChainAPI{
		am: b.AccountManager(),
		b:  b,
	}
}

func (s *PublicChainAPI) CreateChildChain(ctx context.Context, from common.Address, chainId string,
	minValidators *hexutil.Uint, minDepositAmount *hexutil.Big, startBlock, endBlock *hexutil.Big, gas *hexutil.Uint64, gasPrice *hexutil.Big) (common.Hash, error) {

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
		Gas:          gas,
		GasPrice:     gasPrice,
		Value:        nil,
		Data:         nil,
		Nonce:        nil,
		ExtendTxData: etd,
	}

	return s.b.GetInnerAPIBridge().SendTransaction(ctx, args)
}

func (s *PublicChainAPI) JoinChildChain(ctx context.Context, from common.Address, pubkey string, chainId string,
	depositAmount *hexutil.Big, gas *hexutil.Uint64, gasPrice *hexutil.Big) (common.Hash, error) {

	if chainId == "" || strings.Contains(chainId, ";") {
		return common.Hash{}, errors.New("chainId is nil or empty, or contains ';', should be meaningful")
	}

	params := types.MakeKeyValueSet()
	params.Set(JCC_ARGS_FROM, from)
	params.Set(JCC_ARGS_PUBKEY, pubkey)
	params.Set(JCC_ARGS_CHAINID, chainId)
	params.Set(JCC_ARGS_DEPOSIT, depositAmount)

	etd := &types.ExtendTxData{
		FuncName: JCCFuncName,
		Params:   params,
	}

	args := SendTxArgs{
		From:         from,
		To:           nil,
		Gas:          gas,
		GasPrice:     gasPrice,
		Value:        nil,
		Data:         nil,
		Nonce:        nil,
		ExtendTxData: etd,
	}

	return s.b.GetInnerAPIBridge().SendTransaction(ctx, args)
}

func (s *PublicChainAPI) DepositInMainChain(ctx context.Context, from common.Address, chainId string,
	amount *hexutil.Big, gas *hexutil.Uint64, gasPrice *hexutil.Big) (common.Hash, error) {

	if chainId == "" || strings.Contains(chainId, ";") {
		return common.Hash{}, errors.New("chainId is nil or empty, or contains ';', should be meaningful")
	}

	if chainId == "pchain" {
		return common.Hash{}, errors.New("chainId should not be \"pchain\"")
	}

	if s.b.ChainConfig().PChainId != "pchain" {
		return common.Hash{}, errors.New("this api can only be called in main chain - pchain")
	}

	params := types.MakeKeyValueSet()
	params.Set(DIMC_ARGS_FROM, from)
	params.Set(DIMC_ARGS_CHAINID, chainId)
	params.Set(DIMC_ARGS_AMOUNT, amount)

	etd := &types.ExtendTxData{
		FuncName: DIMCFuncName,
		Params:   params,
	}

	args := SendTxArgs{
		From:         from,
		To:           nil,
		Gas:          gas,
		GasPrice:     gasPrice,
		Value:        nil,
		Data:         nil,
		Nonce:        nil,
		ExtendTxData: etd,
	}

	return s.b.GetInnerAPIBridge().SendTransaction(ctx, args)
}

func (s *PublicChainAPI) DepositInChildChain(ctx context.Context, from common.Address, txHash common.Hash) (common.Hash, error) {

	chainId := s.b.ChainConfig().PChainId
	if chainId == "pchain" {
		return common.Hash{}, errors.New("this api can only be called in child chain")
	}

	params := types.MakeKeyValueSet()
	params.Set(DICC_ARGS_FROM, from)
	params.Set(DICC_ARGS_CHAINID, chainId)
	params.Set(DICC_ARGS_TXHASH, txHash)

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
		ExtendTxData: etd,
	}

	return s.b.GetInnerAPIBridge().SendTransaction(ctx, args)
}

func (s *PublicChainAPI) WithdrawFromChildChain(ctx context.Context, from common.Address,
	amount *hexutil.Big, gas *hexutil.Uint64, gasPrice *hexutil.Big) (common.Hash, error) {

	chainId := s.b.ChainConfig().PChainId
	if chainId == "pchain" {
		return common.Hash{}, errors.New("this api can only be called in child chain")
	}

	params := types.MakeKeyValueSet()
	params.Set(WFCC_ARGS_FROM, from)
	params.Set(WFCC_ARGS_CHAINID, chainId)
	params.Set(WFCC_ARGS_AMOUNT, amount)

	etd := &types.ExtendTxData{
		FuncName: WFCCFuncName,
		Params:   params,
	}

	args := SendTxArgs{
		From:         from,
		To:           nil,
		Gas:          gas,
		GasPrice:     gasPrice,
		Value:        nil,
		Data:         nil,
		Nonce:        nil,
		ExtendTxData: etd,
	}

	return s.b.GetInnerAPIBridge().SendTransaction(ctx, args)
}

func (s *PublicChainAPI) WithdrawFromMainChain(ctx context.Context, from common.Address, chainId string, txHash common.Hash) (common.Hash, error) {

	if chainId == "pchain" {
		return common.Hash{}, errors.New("argument can't be the main chain - pchain")
	}

	params := types.MakeKeyValueSet()
	params.Set(WFMC_ARGS_FROM, from)
	params.Set(WFMC_ARGS_CHAINID, chainId)
	params.Set(WFMC_ARGS_TXHASH, txHash)

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
		ExtendTxData: etd,
	}

	return s.b.GetInnerAPIBridge().SendTransaction(ctx, args)
}

func (s *PublicChainAPI) SaveBlockToMainChain(ctx context.Context, from common.Address, block []byte) (common.Hash, error) {

	localChainId := s.b.ChainConfig().PChainId
	if localChainId != "pchain" {
		return common.Hash{}, errors.New("this api can only be called in main chain")
	}

	params := types.MakeKeyValueSet()
	params.Set(SB2MCFuncName_ARGS_FROM, from)

	etd := &types.ExtendTxData{
		FuncName: SB2MCFuncName,
		Params:   params,
	}

	data := hexutil.Bytes(block)
	args := SendTxArgs{
		From:         from,
		To:           nil,
		Gas:          nil,
		GasPrice:     nil,
		Value:        nil,
		Data:         &data,
		Nonce:        nil,
		ExtendTxData: etd,
	}

	return s.b.GetInnerAPIBridge().SendTransaction(ctx, args)
}

func (s *PublicChainAPI) GetTxFromChildChainByHash(ctx context.Context, chainId string, txHash common.Hash) (common.Hash, error) {
	cch := s.b.GetCrossChainHelper()

	childTx := cch.GetTxFromChildChain(txHash, chainId)
	if childTx == nil {
		return common.Hash{}, errors.New(fmt.Sprintf("tx %x does not exist in child chain %s", txHash, chainId))
	}

	return txHash, nil
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

func ccc_ValidateCb(tx *types.Transaction, state *state.StateDB, cch core.CrossChainHelper) error {
	etd := tx.ExtendTxData()

	from, _ := etd.GetAddress(CCC_ARGS_FROM)
	chainId, _ := etd.GetString(CCC_ARGS_CHAINID)
	minValidators, _ := etd.GetUInt(CCC_ARGS_VALIDATOR_THRESHOLD)
	minDepositAmount, _ := etd.GetBigInt(CCC_ARGS_TOKEN_THRESHOLD)
	startBlock, _ := etd.GetBigInt(CCC_ARGS_START_BLOCK)
	endBlock, _ := etd.GetBigInt(CCC_ARGS_END_BLOCK)

	err := cch.CanCreateChildChain(from, chainId, uint16(minValidators), minDepositAmount, startBlock, endBlock)
	if err != nil {
		return err
	}

	return nil
}

func ccc_ApplyCb(tx *types.Transaction, state *state.StateDB, cch core.CrossChainHelper) error {
	etd := tx.ExtendTxData()

	from, _ := etd.GetAddress(CCC_ARGS_FROM)
	chainId, _ := etd.GetString(CCC_ARGS_CHAINID)
	minValidators, _ := etd.GetUInt(CCC_ARGS_VALIDATOR_THRESHOLD)
	minDepositAmount, _ := etd.GetBigInt(CCC_ARGS_TOKEN_THRESHOLD)
	startBlock, _ := etd.GetBigInt(CCC_ARGS_START_BLOCK)
	endBlock, _ := etd.GetBigInt(CCC_ARGS_END_BLOCK)

	if err := cch.CanCreateChildChain(from, chainId, uint16(minValidators), minDepositAmount, startBlock, endBlock); err != nil {
		return err
	}

	err := cch.CreateChildChain(from, chainId, uint16(minValidators), minDepositAmount, startBlock, endBlock)
	if err != nil {
		return err
	}

	return nil
}

func jcc_ValidateCb(tx *types.Transaction, state *state.StateDB, cch core.CrossChainHelper) error {
	etd := tx.ExtendTxData()

	from, _ := etd.GetAddress(JCC_ARGS_FROM)
	pubkey, _ := etd.GetString(JCC_ARGS_PUBKEY)
	chainId, _ := etd.GetString(JCC_ARGS_CHAINID)
	depositAmount, _ := etd.GetBigInt(JCC_ARGS_DEPOSIT)

	// Check Balance
	if state.GetBalance(from).Cmp(depositAmount) == -1 {
		return core.ErrInsufficientFunds
	}

	if err := cch.ValidateJoinChildChain(from, pubkey, chainId, depositAmount); err != nil {
		return err
	}

	return nil
}

func jcc_ApplyCb(tx *types.Transaction, state *state.StateDB, cch core.CrossChainHelper) error {
	etd := tx.ExtendTxData()

	from, _ := etd.GetAddress(JCC_ARGS_FROM)
	pubkey, _ := etd.GetString(JCC_ARGS_PUBKEY)
	chainId, _ := etd.GetString(JCC_ARGS_CHAINID)
	depositAmount, _ := etd.GetBigInt(JCC_ARGS_DEPOSIT)

	// Check Balance
	if state.GetBalance(from).Cmp(depositAmount) == -1 {
		return core.ErrInsufficientFunds
	}

	if err := cch.ValidateJoinChildChain(from, pubkey, chainId, depositAmount); err != nil {
		return err
	}

	// Add the validator into Chain DB
	err := cch.JoinChildChain(from, pubkey, chainId, depositAmount)
	if err != nil {
		return err
	} else {
		// Everything fine, Lock the Balance for this account
		state.SubBalance(from, depositAmount)
		state.AddChildChainDepositBalance(from, chainId, depositAmount)
	}

	return nil
}

func dimc_ValidateCb(tx *types.Transaction, state *state.StateDB, cch core.CrossChainHelper) error {
	etd := tx.ExtendTxData()

	from, _ := etd.GetAddress(DIMC_ARGS_FROM)
	chainId, _ := etd.GetString(DIMC_ARGS_CHAINID)
	amount, _ := etd.GetBigInt(DIMC_ARGS_AMOUNT)

	running := core.CheckChildChainRunning(cch.GetChainInfoDB(), chainId)
	if !running {
		return errors.New(fmt.Sprintf("%s chain not running", chainId))
	}

	if state.GetBalance(from).Cmp(amount) < 0 {
		return errors.New(fmt.Sprintf("%x has no enough balance for deposit", from))
	}

	return nil
}

func dimc_ApplyCb(tx *types.Transaction, state *state.StateDB, cch core.CrossChainHelper) error {
	etd := tx.ExtendTxData()

	from, _ := etd.GetAddress(DIMC_ARGS_FROM)
	chainId, _ := etd.GetString(DIMC_ARGS_CHAINID)
	amount, _ := etd.GetBigInt(DIMC_ARGS_AMOUNT)

	running := core.CheckChildChainRunning(cch.GetChainInfoDB(), chainId)
	if !running {
		return errors.New(fmt.Sprintf("%s chain not running", chainId))
	}

	if state.GetBalance(from).Cmp(amount) < 0 {
		return errors.New(fmt.Sprintf("%x has no enough balance for deposit", from))
	}

	chainInfo := core.GetChainInfo(cch.GetChainInfoDB(), chainId)
	state.SubBalance(from, amount)
	state.AddChainBalance(chainInfo.Owner, amount)

	// record this cross chain tx
	cch.AddToChildChainTx(chainId, from, tx.Hash())

	return nil
}

func dicc_ValidateCb(tx *types.Transaction, state *state.StateDB, cch core.CrossChainHelper) error {
	etd := tx.ExtendTxData()

	from, _ := etd.GetAddress(DICC_ARGS_FROM)
	chainId, _ := etd.GetString(DICC_ARGS_CHAINID)
	txHash, _ := etd.GetHash(DICC_ARGS_TXHASH)

	mainTx := cch.GetTxFromMainChain(txHash)
	if mainTx == nil {
		return errors.New(fmt.Sprintf("tx %x does not exist in main chain", txHash))
	}

	// check from the main chain perspective
	if !cch.HasToChildChainTx(chainId, from, txHash) {
		return errors.New(fmt.Sprintf("tx %x already been used", txHash))
	}
	// check from the child chain perspective
	if cch.HasUsedChildChainTx(chainId, from, txHash) {
		return errors.New(fmt.Sprintf("tx %x already been used", txHash))
	}

	mainEtd := mainTx.ExtendTxData()
	if mainEtd == nil || mainEtd.FuncName != DIMCFuncName {
		return errors.New(fmt.Sprintf("not expected tx %s", mainEtd))
	}

	mainFrom, _ := mainEtd.GetAddress(DIMC_ARGS_FROM)
	mainChainId, _ := mainEtd.GetString(DIMC_ARGS_CHAINID)

	if mainFrom != from || mainChainId != chainId {
		return errors.New("params are not consistent with tx in main chain")
	}

	return nil
}

func dicc_ApplyCb(tx *types.Transaction, state *state.StateDB, cch core.CrossChainHelper) error {
	etd := tx.ExtendTxData()

	from, _ := etd.GetAddress(DICC_ARGS_FROM)
	chainId, _ := etd.GetString(DICC_ARGS_CHAINID)
	txHash, _ := etd.GetHash(DICC_ARGS_TXHASH)

	mainTx := cch.GetTxFromMainChain(txHash)
	if mainTx == nil {
		return errors.New(fmt.Sprintf("tx %x does not exist in main chain", txHash))
	}

	// check from the main chain perspective
	if !cch.HasToChildChainTx(chainId, from, txHash) {
		return errors.New(fmt.Sprintf("tx %x already been used", txHash))
	}
	// check from the child chain perspective
	if cch.HasUsedChildChainTx(chainId, from, txHash) {
		return errors.New(fmt.Sprintf("tx %x already been used", txHash))
	}

	mainEtd := mainTx.ExtendTxData()
	if mainEtd == nil || mainEtd.FuncName != DIMCFuncName {
		return errors.New(fmt.Sprintf("not expected tx %s", mainEtd))
	}

	mainFrom, _ := mainEtd.GetAddress(DIMC_ARGS_FROM)
	mainChainId, _ := mainEtd.GetString(DIMC_ARGS_CHAINID)
	mainAmount, _ := mainEtd.GetBigInt(DIMC_ARGS_AMOUNT)

	if mainFrom != from || mainChainId != chainId {
		return errors.New("params are not consistent with tx in main chain")
	}

	cch.AppendUsedChildChainTx(chainId, from, txHash)
	// the cross chain tx will be removed(in main chain db) when saving the child chain block to the main chain.

	state.AddBalance(mainFrom, mainAmount)

	return nil
}

func wfcc_ValidateCb(tx *types.Transaction, state *state.StateDB, cch core.CrossChainHelper) error {
	etd := tx.ExtendTxData()

	from, _ := etd.GetAddress(WFCC_ARGS_FROM)
	amount, _ := etd.GetBigInt(WFCC_ARGS_AMOUNT)

	if state.GetBalance(from).Cmp(amount) < 0 {
		return errors.New("no enough balance to withdraw")
	}

	return nil
}

func wfcc_ApplyCb(tx *types.Transaction, state *state.StateDB, cch core.CrossChainHelper) error {
	etd := tx.ExtendTxData()

	from, _ := etd.GetAddress(WFCC_ARGS_FROM)
	amount, _ := etd.GetBigInt(WFCC_ARGS_AMOUNT)

	if state.GetBalance(from).Cmp(amount) < 0 {
		return errors.New("no enough balance to withdraw")
	}

	state.SubBalance(from, amount)

	// the cross chain tx will be added(in main chain db) when saving the child chain block to the main chain.

	return nil
}

func wfmc_ValidateCb(tx *types.Transaction, state *state.StateDB, cch core.CrossChainHelper) error {
	etd := tx.ExtendTxData()

	from, _ := etd.GetAddress(WFMC_ARGS_FROM)
	chainId, _ := etd.GetString(WFMC_ARGS_CHAINID)
	txHash, _ := etd.GetHash(WFMC_ARGS_TXHASH)

	childTx := cch.GetTxFromChildChain(txHash, chainId)
	if childTx == nil {
		return errors.New(fmt.Sprintf("tx %x does not exist in child chain %s", txHash, chainId))
	}

	if !cch.HasFromChildChainTx(chainId, from, txHash) {
		return errors.New(fmt.Sprintf("tx %x already been used", txHash))
	}

	childEtd := childTx.ExtendTxData()
	if childEtd == nil || childEtd.FuncName != WFCCFuncName {
		return errors.New(fmt.Sprintf("not expected tx %s", childEtd))
	}

	childFrom, _ := childEtd.GetAddress(WFCC_ARGS_FROM)
	childAmount, _ := childEtd.GetBigInt(WFCC_ARGS_AMOUNT)

	if childFrom != from {
		return errors.New("params are not consistent with tx in child chain")
	}

	chainInfo := core.GetChainInfo(cch.GetChainInfoDB(), chainId)
	if state.GetChainBalance(chainInfo.Owner).Cmp(childAmount) < 0 {
		return errors.New("no enough balance to withdraw")
	}

	return nil
}

func wfmc_ApplyCb(tx *types.Transaction, state *state.StateDB, cch core.CrossChainHelper) error {
	etd := tx.ExtendTxData()

	from, _ := etd.GetAddress(WFMC_ARGS_FROM)
	chainId, _ := etd.GetString(WFMC_ARGS_CHAINID)
	txHash, _ := etd.GetHash(WFMC_ARGS_TXHASH)

	childTx := cch.GetTxFromChildChain(txHash, chainId)
	if childTx == nil {
		return errors.New(fmt.Sprintf("tx %x does not exist in child chain %s", txHash, chainId))
	}

	if !cch.HasFromChildChainTx(chainId, from, txHash) {
		return errors.New(fmt.Sprintf("tx %x already been used", txHash))
	}

	childEtd := childTx.ExtendTxData()
	if childEtd == nil || childEtd.FuncName != WFCCFuncName {
		return errors.New(fmt.Sprintf("not expected tx %s", childEtd))
	}

	childFrom, _ := childEtd.GetAddress(WFCC_ARGS_FROM)
	childChainId, _ := childEtd.GetString(WFCC_ARGS_CHAINID)
	childAmount, _ := childEtd.GetBigInt(WFCC_ARGS_AMOUNT)

	if childFrom != from {
		return errors.New("params are not consistent with tx in child chain")
	}

	chainInfo := core.GetChainInfo(cch.GetChainInfoDB(), chainId)
	chainOwner := chainInfo.Owner
	if state.GetChainBalance(chainOwner).Cmp(childAmount) < 0 {
		return errors.New("no enough balance to withdraw")
	}

	// delete this cross chain tx
	cch.RemoveFromChildChainTx(childChainId, childFrom, txHash)

	state.SubChainBalance(chainOwner, childAmount)
	state.AddBalance(from, childAmount)

	return nil
}

func sb2mc_ValidateCb(tx *types.Transaction, state *state.StateDB, cch core.CrossChainHelper) error {
	etd := tx.ExtendTxData()

	from, _ := etd.GetAddress(SB2MCFuncName_ARGS_FROM)
	block := []byte(tx.Data())

	err := cch.VerifyChildChainBlock(from, block)
	if err != nil {
		return errors.New("block does not pass verification")
	}

	return nil
}

func sb2mc_ApplyCb(tx *types.Transaction, state *state.StateDB, cch core.CrossChainHelper) error {
	block := []byte(tx.Data())

	return cch.SaveChildChainBlockToMainChain(block)
}
