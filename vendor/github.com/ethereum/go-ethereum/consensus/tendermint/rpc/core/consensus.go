package core

import (
	"github.com/tendermint/go-wire"
	cm "github.com/ethereum/go-ethereum/consensus/tendermint/consensus"
	ctypes "github.com/ethereum/go-ethereum/consensus/tendermint/rpc/core/types"
	"github.com/ethereum/go-ethereum/consensus/tendermint/types"
)

func DumpConsensusState(context *RPCDataContext) (*ctypes.ResultDumpConsensusState, error) {
	roundState := context.consensusState.GetRoundState()
	peerRoundStates := []string{}
	for _, peer := range context.p2pSwitch.Peers().List() {
		// TODO: clean this up?
		peerState := peer.Data.Get(types.PeerStateKey).(*cm.PeerState)
		peerRoundState := peerState.GetRoundState()
		peerRoundStateStr := peer.Key + ":" + string(wire.JSONBytes(peerRoundState))
		peerRoundStates = append(peerRoundStates, peerRoundStateStr)
	}
	return &ctypes.ResultDumpConsensusState{roundState.String(), peerRoundStates}, nil
}
