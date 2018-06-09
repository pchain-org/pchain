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
		Type:         nil,
		ExtendTxData: etd,
	}

	return s.b.GetInnerAPIBridge().SendTransaction(ctx, args)
}

func init() {

	core.RegisterValidateCb(CCCFuncName, ccc_ValidateCb)
	core.RegisterApplyCb(CCCFuncName, ccc_ApplyCb)
	//DepositInMainChain
	core.RegisterValidateCb(CCCFuncName, dimc_ValidateCb)
	core.RegisterApplyCb(CCCFuncName, dimc_ApplyCb)
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