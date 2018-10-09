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
	pabi "github.com/pchain/abi"
	"github.com/pkg/errors"
	"math/big"
	"strings"
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

	input, err := pabi.ChainABI.Pack(pabi.CreateChildChain.String(), chainId, uint16(*minValidators), (*big.Int)(minDepositAmount), (*big.Int)(startBlock), (*big.Int)(endBlock))
	if err != nil {
		return common.Hash{}, err
	}

	args := SendTxArgs{
		From:     from,
		To:       &pabi.ChainContractMagicAddr,
		Gas:      gas,
		GasPrice: gasPrice,
		Value:    nil,
		Input:    (*hexutil.Bytes)(&input),
		Nonce:    nil,
	}

	return s.b.GetInnerAPIBridge().SendTransaction(ctx, args)
}

func (s *PublicChainAPI) JoinChildChain(ctx context.Context, from common.Address, pubkey string, chainId string,
	depositAmount *hexutil.Big, gas *hexutil.Uint64, gasPrice *hexutil.Big) (common.Hash, error) {

	if chainId == "" || strings.Contains(chainId, ";") {
		return common.Hash{}, errors.New("chainId is nil or empty, or contains ';', should be meaningful")
	}

	input, err := pabi.ChainABI.Pack(pabi.JoinChildChain.String(), pubkey, chainId, (*big.Int)(depositAmount))
	if err != nil {
		return common.Hash{}, err
	}

	args := SendTxArgs{
		From:     from,
		To:       &pabi.ChainContractMagicAddr,
		Gas:      gas,
		GasPrice: gasPrice,
		Value:    nil,
		Input:    (*hexutil.Bytes)(&input),
		Nonce:    nil,
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

	input, err := pabi.ChainABI.Pack(pabi.DepositInMainChain.String(), chainId, (*big.Int)(amount))
	if err != nil {
		return common.Hash{}, err
	}

	args := SendTxArgs{
		From:     from,
		To:       &pabi.ChainContractMagicAddr,
		Gas:      gas,
		GasPrice: gasPrice,
		Value:    nil,
		Input:    (*hexutil.Bytes)(&input),
		Nonce:    nil,
	}

	return s.b.GetInnerAPIBridge().SendTransaction(ctx, args)
}

func (s *PublicChainAPI) DepositInChildChain(ctx context.Context, from common.Address, txHash common.Hash) (common.Hash, error) {

	chainId := s.b.ChainConfig().PChainId
	if chainId == "pchain" {
		return common.Hash{}, errors.New("this api can only be called in child chain")
	}

	input, err := pabi.ChainABI.Pack(pabi.DepositInChildChain.String(), chainId, txHash)
	if err != nil {
		return common.Hash{}, err
	}

	args := SendTxArgs{
		From:     from,
		To:       &pabi.ChainContractMagicAddr,
		Gas:      nil,
		GasPrice: nil,
		Value:    nil,
		Input:    (*hexutil.Bytes)(&input),
		Nonce:    nil,
	}

	return s.b.GetInnerAPIBridge().SendTransaction(ctx, args)
}

func (s *PublicChainAPI) WithdrawFromChildChain(ctx context.Context, from common.Address,
	amount *hexutil.Big, gas *hexutil.Uint64, gasPrice *hexutil.Big) (common.Hash, error) {

	chainId := s.b.ChainConfig().PChainId
	if chainId == "pchain" {
		return common.Hash{}, errors.New("this api can only be called in child chain")
	}

	input, err := pabi.ChainABI.Pack(pabi.WithdrawFromChildChain.String(), chainId, (*big.Int)(amount))
	if err != nil {
		return common.Hash{}, err
	}

	args := SendTxArgs{
		From:     from,
		To:       &pabi.ChainContractMagicAddr,
		Gas:      gas,
		GasPrice: gasPrice,
		Value:    nil,
		Input:    (*hexutil.Bytes)(&input),
		Nonce:    nil,
	}

	return s.b.GetInnerAPIBridge().SendTransaction(ctx, args)
}

func (s *PublicChainAPI) WithdrawFromMainChain(ctx context.Context, from common.Address, chainId string, txHash common.Hash) (common.Hash, error) {

	if chainId == "pchain" {
		return common.Hash{}, errors.New("argument can't be the main chain - pchain")
	}

	input, err := pabi.ChainABI.Pack(pabi.WithdrawFromMainChain.String(), chainId, txHash)
	if err != nil {
		return common.Hash{}, err
	}

	args := SendTxArgs{
		From:     from,
		To:       &pabi.ChainContractMagicAddr,
		Gas:      nil,
		GasPrice: nil,
		Value:    nil,
		Input:    (*hexutil.Bytes)(&input),
		Nonce:    nil,
	}

	return s.b.GetInnerAPIBridge().SendTransaction(ctx, args)
}

func (s *PublicChainAPI) GetTxFromChildChainByHash(ctx context.Context, chainId string, txHash common.Hash) (common.Hash, error) {
	cch := s.b.GetCrossChainHelper()

	childTx := cch.GetTxFromChildChain(txHash, chainId)
	if childTx == nil {
		return common.Hash{}, fmt.Errorf("tx %x does not exist in child chain %s", txHash, chainId)
	}

	return txHash, nil
}

func init() {
	//CreateChildChain
	core.RegisterValidateCb(pabi.CreateChildChain, ccc_ValidateCb)
	core.RegisterApplyCb(pabi.CreateChildChain, ccc_ApplyCb)

	//JoinChildChain
	core.RegisterValidateCb(pabi.JoinChildChain, jcc_ValidateCb)
	core.RegisterApplyCb(pabi.JoinChildChain, jcc_ApplyCb)

	//DepositInMainChain
	core.RegisterValidateCb(pabi.DepositInMainChain, dimc_ValidateCb)
	core.RegisterApplyCb(pabi.DepositInMainChain, dimc_ApplyCb)

	//DepositInChildChain
	core.RegisterValidateCb(pabi.DepositInChildChain, dicc_ValidateCb)
	core.RegisterApplyCb(pabi.DepositInChildChain, dicc_ApplyCb)

	//WithdrawFromChildChain
	core.RegisterValidateCb(pabi.WithdrawFromChildChain, wfcc_ValidateCb)
	core.RegisterApplyCb(pabi.WithdrawFromChildChain, wfcc_ApplyCb)

	//WithdrawFromMainChain
	core.RegisterValidateCb(pabi.WithdrawFromMainChain, wfmc_ValidateCb)
	core.RegisterApplyCb(pabi.WithdrawFromMainChain, wfmc_ApplyCb)

	//SD2MCFuncName
	core.RegisterValidateCb(pabi.SaveDataToMainChain, sd2mc_ValidateCb)
	core.RegisterApplyCb(pabi.SaveDataToMainChain, sd2mc_ApplyCb)
}

func ccc_ValidateCb(tx *types.Transaction, signer types.Signer, state *state.StateDB, cch core.CrossChainHelper) error {

	from, err := types.Sender(signer, tx)
	if err != nil {
		return core.ErrInvalidSender
	}

	var args pabi.CreateChildChainArgs
	data := tx.Data()
	if err := pabi.ChainABI.UnpackMethodInputs(&args, pabi.CreateChildChain.String(), data[4:]); err != nil {
		return err
	}

	if err := cch.CanCreateChildChain(from, args.ChainId, args.MinValidators, args.MinDepositAmount, args.StartBlock, args.EndBlock); err != nil {
		return err
	}

	return nil
}

func ccc_ApplyCb(tx *types.Transaction, signer types.Signer, state *state.StateDB, ops *types.PendingOps, cch core.CrossChainHelper) error {

	from, err := types.Sender(signer, tx)
	if err != nil {
		return core.ErrInvalidSender
	}

	var args pabi.CreateChildChainArgs
	data := tx.Data()
	if err := pabi.ChainABI.UnpackMethodInputs(&args, pabi.CreateChildChain.String(), data[4:]); err != nil {
		return err
	}

	if err := cch.CanCreateChildChain(from, args.ChainId, args.MinValidators, args.MinDepositAmount, args.StartBlock, args.EndBlock); err != nil {
		return err
	}

	op := types.CreateChildChainOp{
		From:             from,
		ChainId:          args.ChainId,
		MinValidators:    args.MinValidators,
		MinDepositAmount: args.MinDepositAmount,
		StartBlock:       args.StartBlock,
		EndBlock:         args.EndBlock,
	}
	if ok := ops.Append(&op); !ok {
		return fmt.Errorf("pending ops conflict: %v", op)
	}
	return nil
}

func jcc_ValidateCb(tx *types.Transaction, signer types.Signer, state *state.StateDB, cch core.CrossChainHelper) error {
	from, err := types.Sender(signer, tx)
	if err != nil {
		return core.ErrInvalidSender
	}

	var args pabi.JoinChildChainArgs
	data := tx.Data()
	if err := pabi.ChainABI.UnpackMethodInputs(&args, pabi.JoinChildChain.String(), data[4:]); err != nil {
		return err
	}

	// Check Balance
	if state.GetBalance(from).Cmp(args.DepositAmount) == -1 {
		return core.ErrInsufficientFunds
	}

	if err := cch.ValidateJoinChildChain(from, args.PubKey, args.ChainId, args.DepositAmount); err != nil {
		return err
	}

	return nil
}

func jcc_ApplyCb(tx *types.Transaction, signer types.Signer, state *state.StateDB, ops *types.PendingOps, cch core.CrossChainHelper) error {
	from, err := types.Sender(signer, tx)
	if err != nil {
		return core.ErrInvalidSender
	}

	var args pabi.JoinChildChainArgs
	data := tx.Data()
	if err := pabi.ChainABI.UnpackMethodInputs(&args, pabi.JoinChildChain.String(), data[4:]); err != nil {
		return err
	}

	// Check Balance
	if state.GetBalance(from).Cmp(args.DepositAmount) == -1 {
		return core.ErrInsufficientFunds
	}

	if err := cch.ValidateJoinChildChain(from, args.PubKey, args.ChainId, args.DepositAmount); err != nil {
		return err
	}

	op := types.JoinChildChainOp{
		From:          from,
		PubKey:        args.PubKey,
		ChainId:       args.ChainId,
		DepositAmount: args.DepositAmount,
	}
	if ok := ops.Append(&op); !ok {
		return fmt.Errorf("pending ops conflict: %v", op)
	}

	// Everything fine, Lock the Balance for this account
	state.SubBalance(from, args.DepositAmount)
	state.AddChildChainDepositBalance(from, args.ChainId, args.DepositAmount)

	return nil
}

func dimc_ValidateCb(tx *types.Transaction, signer types.Signer, state *state.StateDB, cch core.CrossChainHelper) error {
	from, err := types.Sender(signer, tx)
	if err != nil {
		return core.ErrInvalidSender
	}

	var args pabi.DepositInMainChainArgs
	data := tx.Data()
	if err := pabi.ChainABI.UnpackMethodInputs(&args, pabi.DepositInMainChain.String(), data[4:]); err != nil {
		return err
	}

	running := core.CheckChildChainRunning(cch.GetChainInfoDB(), args.ChainId)
	if !running {
		return fmt.Errorf("%s chain not running", args.ChainId)
	}

	if state.GetBalance(from).Cmp(args.Amount) < 0 {
		return fmt.Errorf("%x has no enough balance for deposit", from)
	}

	return nil
}

func dimc_ApplyCb(tx *types.Transaction, signer types.Signer, state *state.StateDB, ops *types.PendingOps, cch core.CrossChainHelper) error {
	from, err := types.Sender(signer, tx)
	if err != nil {
		return core.ErrInvalidSender
	}

	var args pabi.DepositInMainChainArgs
	data := tx.Data()
	if err := pabi.ChainABI.UnpackMethodInputs(&args, pabi.DepositInMainChain.String(), data[4:]); err != nil {
		return err
	}

	running := core.CheckChildChainRunning(cch.GetChainInfoDB(), args.ChainId)
	if !running {
		return fmt.Errorf("%s chain not running", args.ChainId)
	}

	if state.GetBalance(from).Cmp(args.Amount) < 0 {
		return fmt.Errorf("%x has no enough balance for deposit", from)
	}

	op := types.MarkMainChainToChildChainTxOp{
		CrossChainTx: types.CrossChainTx{
			From:    from,
			ChainId: args.ChainId,
			TxHash:  tx.Hash(),
		},
	}
	if ok := ops.Append(&op); !ok {
		return fmt.Errorf("pending ops conflict: %v", op)
	}

	chainInfo := core.GetChainInfo(cch.GetChainInfoDB(), args.ChainId)
	state.SubBalance(from, args.Amount)
	state.AddChainBalance(chainInfo.Owner, args.Amount)

	return nil
}

func dicc_ValidateCb(tx *types.Transaction, signer types.Signer, state *state.StateDB, cch core.CrossChainHelper) error {
	from, err := types.Sender(signer, tx)
	if err != nil {
		return core.ErrInvalidSender
	}

	var args pabi.DepositInChildChainArgs
	data := tx.Data()
	if err := pabi.ChainABI.UnpackMethodInputs(&args, pabi.DepositInChildChain.String(), data[4:]); err != nil {
		return err
	}

	dimcTx := cch.GetTxFromMainChain(args.TxHash)
	if dimcTx == nil {
		return fmt.Errorf("tx %x does not exist in main chain", args.TxHash)
	}

	// check from the main chain perspective
	if s := cch.ValidateToChildChainTx(from, args.ChainId, args.TxHash); s != core.CrossChainTxReady {
		return fmt.Errorf("tx %x has wrong state: %v", args.TxHash, s)
	}

	// check from the child chain perspective
	if cch.IsTxUsedOnChildChain(from, args.ChainId, args.TxHash) {
		return fmt.Errorf("tx %x already used in child chain", args.TxHash)
	}

	dimcFrom, err := types.Sender(signer, dimcTx)
	if err != nil {
		return core.ErrInvalidSender
	}

	var dimcArgs pabi.DepositInMainChainArgs
	dimcData := dimcTx.Data()
	if err := pabi.ChainABI.UnpackMethodInputs(&dimcArgs, pabi.DepositInMainChain.String(), dimcData[4:]); err != nil {
		return err
	}

	if from != dimcFrom || args.ChainId != dimcArgs.ChainId {
		return errors.New("params are not consistent with tx in main chain")
	}

	return nil
}

func dicc_ApplyCb(tx *types.Transaction, signer types.Signer, state *state.StateDB, ops *types.PendingOps, cch core.CrossChainHelper) error {
	from, err := types.Sender(signer, tx)
	if err != nil {
		return core.ErrInvalidSender
	}

	var args pabi.DepositInChildChainArgs
	data := tx.Data()
	if err := pabi.ChainABI.UnpackMethodInputs(&args, pabi.DepositInChildChain.String(), data[4:]); err != nil {
		return err
	}

	dimcTx := cch.GetTxFromMainChain(args.TxHash)
	if dimcTx == nil {
		return fmt.Errorf("tx %x does not exist in main chain", args.TxHash)
	}

	// check from the main chain perspective
	if s := cch.ValidateToChildChainTx(from, args.ChainId, args.TxHash); s != core.CrossChainTxReady {
		return fmt.Errorf("tx %x has wrong state: %v", args.TxHash, s)
	}

	// check from the child chain perspective
	if cch.IsTxUsedOnChildChain(from, args.ChainId, args.TxHash) {
		return fmt.Errorf("tx %x already used in child chain", args.TxHash)
	}

	dimcFrom, err := types.Sender(signer, dimcTx)
	if err != nil {
		return core.ErrInvalidSender
	}

	var dimcArgs pabi.DepositInMainChainArgs
	dimcData := dimcTx.Data()
	if err := pabi.ChainABI.UnpackMethodInputs(&dimcArgs, pabi.DepositInMainChain.String(), dimcData[4:]); err != nil {
		return err
	}

	if from != dimcFrom || args.ChainId != dimcArgs.ChainId {
		return errors.New("params are not consistent with tx in main chain")
	}

	op := types.MarkTxUsedOnChildChainOp{
		CrossChainTx: types.CrossChainTx{
			From:    from,
			ChainId: args.ChainId,
			TxHash:  args.TxHash,
		},
	}
	if ok := ops.Append(&op); !ok {
		return fmt.Errorf("pending ops conflict: %v", op)
	}

	state.AddBalance(dimcFrom, dimcArgs.Amount)

	return nil
}

func wfcc_ValidateCb(tx *types.Transaction, signer types.Signer, state *state.StateDB, cch core.CrossChainHelper) error {
	from, err := types.Sender(signer, tx)
	if err != nil {
		return core.ErrInvalidSender
	}

	var args pabi.WithdrawFromChildChainArgs
	data := tx.Data()
	if err := pabi.ChainABI.UnpackMethodInputs(&args, pabi.WithdrawFromChildChain.String(), data[4:]); err != nil {
		return err
	}

	if state.GetBalance(from).Cmp(args.Amount) < 0 {
		return errors.New("no enough balance to withdraw")
	}

	return nil
}

func wfcc_ApplyCb(tx *types.Transaction, signer types.Signer, state *state.StateDB, ops *types.PendingOps, cch core.CrossChainHelper) error {
	from, err := types.Sender(signer, tx)
	if err != nil {
		return core.ErrInvalidSender
	}

	var args pabi.WithdrawFromChildChainArgs
	data := tx.Data()
	if err := pabi.ChainABI.UnpackMethodInputs(&args, pabi.WithdrawFromChildChain.String(), data[4:]); err != nil {
		return err
	}

	if state.GetBalance(from).Cmp(args.Amount) < 0 {
		return errors.New("no enough balance to withdraw")
	}

	state.SubBalance(from, args.Amount)

	return nil
}

func wfmc_ValidateCb(tx *types.Transaction, signer types.Signer, state *state.StateDB, cch core.CrossChainHelper) error {
	from, err := types.Sender(signer, tx)
	if err != nil {
		return core.ErrInvalidSender
	}

	var args pabi.WithdrawFromMainChainArgs
	data := tx.Data()
	if err := pabi.ChainABI.UnpackMethodInputs(&args, pabi.WithdrawFromMainChain.String(), data[4:]); err != nil {
		return err
	}

	wfccTx := cch.GetTxFromChildChain(args.TxHash, args.ChainId)
	if wfccTx == nil {
		return fmt.Errorf("tx %x does not exist in child chain %s", args.TxHash, args.ChainId)
	}

	if s := cch.ValidateFromChildChainTx(from, args.ChainId, args.TxHash); s != core.CrossChainTxReady {
		return fmt.Errorf("tx %x has wrong state: %v", args.TxHash, s)
	}

	wfccFrom, err := types.Sender(signer, wfccTx)
	if err != nil {
		return core.ErrInvalidSender
	}

	var wfccArgs pabi.WithdrawFromChildChainArgs
	wfccData := wfccTx.Data()
	if err := pabi.ChainABI.UnpackMethodInputs(&wfccArgs, pabi.WithdrawFromChildChain.String(), wfccData[4:]); err != nil {
		return err
	}

	if from != wfccFrom || args.ChainId != wfccArgs.ChainId {
		return errors.New("params are not consistent with tx in child chain")
	}

	chainInfo := core.GetChainInfo(cch.GetChainInfoDB(), wfccArgs.ChainId)
	if state.GetChainBalance(chainInfo.Owner).Cmp(wfccArgs.Amount) < 0 {
		return errors.New("no enough balance to withdraw")
	}

	return nil
}

func wfmc_ApplyCb(tx *types.Transaction, signer types.Signer, state *state.StateDB, ops *types.PendingOps, cch core.CrossChainHelper) error {
	from, err := types.Sender(signer, tx)
	if err != nil {
		return core.ErrInvalidSender
	}

	var args pabi.WithdrawFromMainChainArgs
	data := tx.Data()
	if err := pabi.ChainABI.UnpackMethodInputs(&args, pabi.WithdrawFromMainChain.String(), data[4:]); err != nil {
		return err
	}

	wfccTx := cch.GetTxFromChildChain(args.TxHash, args.ChainId)
	if wfccTx == nil {
		return fmt.Errorf("tx %x does not exist in child chain %s", args.TxHash, args.ChainId)
	}

	if s := cch.ValidateFromChildChainTx(from, args.ChainId, args.TxHash); s != core.CrossChainTxReady {
		return fmt.Errorf("tx %x has wrong state: %v", args.TxHash, s)
	}

	wfccFrom, err := types.Sender(signer, wfccTx)
	if err != nil {
		return core.ErrInvalidSender
	}

	var wfccArgs pabi.WithdrawFromChildChainArgs
	wfccData := wfccTx.Data()
	if err := pabi.ChainABI.UnpackMethodInputs(&wfccArgs, pabi.WithdrawFromChildChain.String(), wfccData[4:]); err != nil {
		return err
	}

	if from != wfccFrom || args.ChainId != wfccArgs.ChainId {
		return errors.New("params are not consistent with tx in child chain")
	}

	chainInfo := core.GetChainInfo(cch.GetChainInfoDB(), wfccArgs.ChainId)
	if state.GetChainBalance(chainInfo.Owner).Cmp(wfccArgs.Amount) < 0 {
		return errors.New("no enough balance to withdraw")
	}

	op := types.MarkChildChainToMainChainTxUsedOp{
		CrossChainTx: types.CrossChainTx{
			From:    from,
			ChainId: args.ChainId,
			TxHash:  args.TxHash,
		},
	}
	if ok := ops.Append(&op); !ok {
		return fmt.Errorf("pending ops conflict: %v", op)
	}

	state.SubChainBalance(chainInfo.Owner, wfccArgs.Amount)
	state.AddBalance(wfccFrom, wfccArgs.Amount)

	return nil
}

func sd2mc_ValidateCb(tx *types.Transaction, signer types.Signer, state *state.StateDB, cch core.CrossChainHelper) error {

	var bs []byte
	data := tx.Data()
	if err := pabi.ChainABI.UnpackMethodInputs(&bs, pabi.SaveDataToMainChain.String(), data[4:]); err != nil {
		return err
	}

	err := cch.VerifyChildChainProofData(bs)
	if err != nil {
		return fmt.Errorf("data can not pass verification: %v", err)
	}

	return nil
}

func sd2mc_ApplyCb(tx *types.Transaction, signer types.Signer, state *state.StateDB, ops *types.PendingOps, cch core.CrossChainHelper) error {
	var bs []byte
	data := tx.Data()
	if err := pabi.ChainABI.UnpackMethodInputs(&bs, pabi.SaveDataToMainChain.String(), data[4:]); err != nil {
		return err
	}

	op := types.SaveDataToMainChainOp{
		Data: bs,
	}
	if ok := ops.Append(&op); !ok {
		return fmt.Errorf("pending ops conflict: %v", op)
	}

	return nil
}
