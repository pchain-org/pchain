package core

import (
	"fmt"

	ctypes "github.com/ethereum/go-ethereum/consensus/tendermint/rpc/core/types"
)

//-----------------------------------------------------------------------------

func NetInfo(context *RPCDataContext) (*ctypes.ResultNetInfo, error) {
	listening := context.p2pSwitch.IsListening()
	listeners := []string{}
	for _, listener := range context.p2pSwitch.Listeners() {
		listeners = append(listeners, listener.String())
	}
	peers := []ctypes.Peer{}
	for _, peer := range context.p2pSwitch.Peers().List() {
		peers = append(peers, ctypes.Peer{
			NodeInfo:         *peer.NodeInfo,
			IsOutbound:       peer.IsOutbound(),
			ConnectionStatus: peer.Connection().Status(),
		})
	}
	return &ctypes.ResultNetInfo{
		Listening: listening,
		Listeners: listeners,
		Peers:     peers,
	}, nil
}

//-----------------------------------------------------------------------------

// Dial given list of seeds
func UnsafeDialSeeds(context *RPCDataContext, seeds []string) (*ctypes.ResultDialSeeds, error) {

	if len(seeds) == 0 {
		return &ctypes.ResultDialSeeds{}, fmt.Errorf("No seeds provided")
	}
	// starts go routines to dial each seed after random delays
	log.Info("DialSeeds", "addrBook", context.addrBook, "seeds", seeds)
	err := context.p2pSwitch.DialSeeds(context.addrBook, seeds)
	if err != nil {
		return &ctypes.ResultDialSeeds{}, err
	}
	return &ctypes.ResultDialSeeds{"Dialing seeds in progress. See /net_info for details"}, nil
}

//-----------------------------------------------------------------------------

func Genesis(context *RPCDataContext) (*ctypes.ResultGenesis, error) {
	return &ctypes.ResultGenesis{context.genDoc}, nil
}
