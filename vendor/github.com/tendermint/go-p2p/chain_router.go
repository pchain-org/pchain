package p2p

import (
	"fmt"
	//cmn "github.com/tendermint/go-common"
)

// ChainRouter used in P2P Switch for multi-chain Reactor
type ChainRouter struct {
	reactors     map[string]Reactor
	chDescs      []*ChannelDescriptor
	reactorsByCh map[byte]Reactor
}

func (cr *ChainRouter) AddReactor(name string, reactor Reactor) {
	// Validate the reactor.
	// No two reactors can share the same channel.
	reactorChannels := reactor.GetChannels()
	for _, chDesc := range reactorChannels {
		chID := chDesc.ID
		if cr.reactorsByCh[chID] != nil {
			log.Debugf("！！Channel %X has multiple reactors %v & %v", chID, cr.reactorsByCh[chID], reactor)
		}
		cr.chDescs = append(cr.chDescs, chDesc)
		cr.reactorsByCh[chID] = reactor
	}
	cr.reactors[name] = reactor
}

// ChainChannel used in each MConnection for multi-chain Channel
type ChainChannel struct {
	channels    []*Channel
	channelsIdx map[byte]*Channel
}
