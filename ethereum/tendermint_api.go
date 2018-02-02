package ethereum

import (
	"github.com/ethereum/go-ethereum/rpc"
	"golang.org/x/net/context"
	"github.com/tendermint/tendermint/rpc/core/types"
	"fmt"
	"github.com/tendermint/go-wire"
)

type PublicTendermintAPI struct {
	Client Client
}

// NewPublicEthereumAPI creates a new Etheruem protocol API.
func NewPublicTendermintAPI(client Client) *PublicTendermintAPI {
	return &PublicTendermintAPI{client}
}

func TdmAPIs(client Client) []rpc.API {
	return []rpc.API{
		{
			Namespace: "tdm",
			Version:   "1.0",
			Service:   NewPublicTendermintAPI(client),
			Public:    true,
		},
	}
}

// GasPrice returns a suggestion for a gas price.
func (s *PublicTendermintAPI) GetBlock(ctx context.Context, blockNumber rpc.BlockNumber) (string, error) {

	var result core_types.TMResult

	//fmt.Printf("GetBlock() called with startBlock: %v\n", blockNumber)
	params := map[string]interface{}{
		"height":  blockNumber,
	}

	_, err := s.Client.Call("block", params, &result)
	if err != nil {
		fmt.Println(err)
	}

	//fmt.Printf("tdm_getBlock: %v\n", result)
	return result.(*core_types.ResultBlock).Block.String(), nil
}

// GasPrice returns a suggestion for a gas price.
func (s *PublicTendermintAPI) GetValidator(ctx context.Context, addr string) (string, error) {

	var result core_types.TMResult

	//fmt.Printf("GetBlock() called with startBlock: %v\n", blockNumber)
	//params := map[string]interface{}{
	//	"addr":  addr,
	//}

	params := map[string]interface{}{}
	_, err := s.Client.Call("validators", params, &result)
	if err != nil {
		fmt.Println(err)
	}

	fmt.Printf("tdm_getValidator: %v\n", result)
	return string(wire.JSONBytes(result.(*core_types.ResultValidators))), nil
}
