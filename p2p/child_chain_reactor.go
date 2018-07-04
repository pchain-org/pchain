package p2p

import (
	"bytes"
	"fmt"
	"github.com/pchain/common/plogger"
	"github.com/tendermint/go-p2p"
	"github.com/tendermint/go-wire"
	"reflect"
)

const (
	// ChainChannel is a channel for Child ChainID messages exchanging
	ChainChannel = byte(0x10)

	maxChildChainMessageSize = 1048576 // 1MB

)

var plog = plogger.GetLogger("ChildChainReactor")

// ChainReactor is only available for main chain used
type ChainReactor struct {
	p2p.BaseReactor
}

func NewChainReactor() *ChainReactor {
	r := &ChainReactor{}
	r.BaseReactor = *p2p.NewBaseReactor(nil, "ChainReactor", r)
	return r
}

// GetChannels implements Reactor
func (r *ChainReactor) GetChannels() []*p2p.ChannelDescriptor {
	return []*p2p.ChannelDescriptor{
		{
			ID:                ChainChannel,
			Priority:          1,
			SendQueueCapacity: 100,
		},
	}
}

// Implements Reactor
func (r *ChainReactor) Receive(chID byte, src *p2p.Peer, msgBytes []byte) {
	_, msg, err := DecodeMessage(msgBytes)
	if err != nil {
		plog.Warn("Error decoding message", "error", err)
		return
	}
	plog.Debug("Receive", "src", src, "chId", chID, "msg", msg)

	switch msg := msg.(type) {
	case *ccRequestMessage:
		// Check the chain id from request matched the local network
		update := r.checkAndUpdateNetwork(msg.childChainID, src)

		// Send Response Message back to peer
		if update {
			r.sendResponse(src, msg.childChainID)
		}

	case *ccResponseMessage:
		// Check the chain id from request matched the local network
		r.checkAndUpdateNetwork(msg.childChainID, src)

	default:
		plog.Warnf("Unknown message type %v", reflect.TypeOf(msg))
	}
}

func (r *ChainReactor) checkAndUpdateNetwork(chainID string, src *p2p.Peer) bool {

	if chainID == "pchain" {
		// Main Chain should not be add to network
		return false
	}

	// Check if the Chain ID in the current node info network
	if !r.Switch.NodeInfo().ExistNetwork(chainID) {
		return false
	}

	// Check if the Chain ID in the peer node info
	if !src.ExistNetwork(chainID) {
		// Add chain id into Src Peer Network
		src.AddNetwork(chainID)

		// Add the Channel into Src Peer MConnection
		src.AddChainChannelByChainID(chainID, r.Switch.ChainRouter(chainID))
		return true
	}

	return false
}

// broadcastNewChainIDRequest send child chain id to all the peers
func (r *ChainReactor) broadcastNewChainIDRequest(childChainID string) {
	r.Switch.Broadcast("pchain", ChainChannel, struct{ ChildChainMessage }{&ccRequestMessage{childChainID}})
}

// sendResponse sends chain id back to the peer.
func (r *ChainReactor) sendResponse(p *p2p.Peer, childChainID string) {
	p.Send("pchain", ChainChannel, struct{ ChildChainMessage }{&ccResponseMessage{childChainID: childChainID}})
}

//-----------------------------------------------------------------------------
// Messages

const (
	msgTypeChildChainRequest  = byte(0x01)
	msgTypeChildChainResponse = byte(0x02)
)

// ChildChainMessage is a primary type for Child ChainID messages. Underneath, it could contain
// either ccRequestMessage, or ccResponseMessage messages.
type ChildChainMessage interface{}

var _ = wire.RegisterInterface(
	struct{ ChildChainMessage }{},
	wire.ConcreteType{&ccRequestMessage{}, msgTypeChildChainRequest},
	wire.ConcreteType{&ccResponseMessage{}, msgTypeChildChainResponse},
)

// DecodeMessage implements interface registered above.
func DecodeMessage(bz []byte) (msgType byte, msg ChildChainMessage, err error) {
	msgType = bz[0]
	n := new(int)
	r := bytes.NewReader(bz)
	msg = wire.ReadBinary(struct{ ChildChainMessage }{}, r, maxChildChainMessageSize, n, &err).(struct{ ChildChainMessage }).ChildChainMessage
	return
}

/*
A ccRequestMessage tell other peer that I have joined the new chain
*/
type ccRequestMessage struct {
	childChainID string
}

func (m *ccRequestMessage) String() string {
	return fmt.Sprintf("[ccRequest %v]", m.childChainID)
}

/*
A ccResponseMessage response the peer that I have added the child chain id to the peer's nodeinfo
*/
type ccResponseMessage struct {
	childChainID string
}

func (m *ccResponseMessage) String() string {
	return fmt.Sprintf("[ccResponse %v]", m.childChainID)
}
