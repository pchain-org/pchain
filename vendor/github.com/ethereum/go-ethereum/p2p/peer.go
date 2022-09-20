// Copyright 2014 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package p2p

import (
	"fmt"
	"io"
	"net"
	"sort"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/mclock"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/p2p/discover"
	"github.com/ethereum/go-ethereum/rlp"
)

const (
	baseProtocolVersion    = 5
	baseProtocolLength     = uint64(16)
	baseProtocolMaxMsgSize = 2 * 1024

	snappyProtocolVersion = 5

	pingInterval = 15 * time.Second
)

const (
	// devp2p message codes
	handshakeMsg = 0x00
	discMsg      = 0x01
	pingMsg      = 0x02
	pongMsg      = 0x03

	// PChain message belonging to pchain/64
	BroadcastNewChildChainMsg = 0x04
	ConfirmNewChildChainMsg   = 0x05

	RefreshValidatorNodeInfoMsg = 0x06
	RemoveValidatorNodeInfoMsg  = 0x07
)

// protoHandshake is the RLP structure of the protocol handshake.
type protoHandshake struct {
	Version    uint64
	Name       string
	Caps       []Cap
	ListenPort uint64
	ID         discover.NodeID

	// Ignore additional fields (for forward compatibility).
	Rest []rlp.RawValue `rlp:"tail"`
}

// PeerEventType is the type of peer events emitted by a p2p.Server
type PeerEventType string

const (
	// PeerEventTypeAdd is the type of event emitted when a peer is added
	// to a p2p.Server
	PeerEventTypeAdd PeerEventType = "add"

	// PeerEventTypeDrop is the type of event emitted when a peer is
	// dropped from a p2p.Server
	PeerEventTypeDrop PeerEventType = "drop"

	// PeerEventTypeMsgSend is the type of event emitted when a
	// message is successfully sent to a peer
	PeerEventTypeMsgSend PeerEventType = "msgsend"

	// PeerEventTypeMsgRecv is the type of event emitted when a
	// message is received from a peer
	PeerEventTypeMsgRecv PeerEventType = "msgrecv"

	// PeerEventTypeMsgSend is the type of event emitted when a
	// message is successfully sent to a peer
	PeerEventTypeRefreshValidator PeerEventType = "refreshvalidator"

	// PeerEventTypeMsgRecv is the type of event emitted when a
	// message is received from a peer
	PeerEventTypeRemoveValidator PeerEventType = "removevalidator"
)

// PeerEvent is an event emitted when peers are either added or dropped from
// a p2p.Server or when a message is sent or received on a peer connection
type PeerEvent struct {
	Type     PeerEventType   `json:"type"`
	Peer     discover.NodeID `json:"peer"`
	Error    string          `json:"error,omitempty"`
	Protocol string          `json:"protocol,omitempty"`
	MsgCode  *uint64         `json:"msg_code,omitempty"`
	MsgSize  *uint32         `json:"msg_size,omitempty"`
}

// Peer represents a connected remote node.
type Peer struct {
	rw      *conn
	running map[string]*protoRW
	log     log.Logger
	created mclock.AbsTime

	wg       sync.WaitGroup
	protoErr chan error
	closed   chan struct{}
	disc     chan DiscReason

	// events receives message send / receive events if set
	events *event.Feed

	// srvProtocols must link with Server Protocols
	srvProtocols *[]Protocol
}

// NewPeer returns a peer for testing purposes.
func NewPeer(id discover.NodeID, name string, caps []Cap) *Peer {
	pipe, _ := net.Pipe()
	conn := &conn{fd: pipe, transport: nil, id: id, caps: caps, name: name}
	peer := newPeer(conn, nil)
	close(peer.closed) // ensures Disconnect doesn't block
	return peer
}

// ID returns the node's public key.
func (p *Peer) ID() discover.NodeID {
	return p.rw.id
}

// Name returns the node name that the remote node advertised.
func (p *Peer) Name() string {
	return p.rw.name
}

// Caps returns the capabilities (supported subprotocols) of the remote peer.
func (p *Peer) Caps() []Cap {
	// TODO: maybe return copy
	return p.rw.caps
}

// RemoteAddr returns the remote address of the network connection.
func (p *Peer) RemoteAddr() net.Addr {
	return p.rw.fd.RemoteAddr()
}

// LocalAddr returns the local address of the network connection.
func (p *Peer) LocalAddr() net.Addr {
	return p.rw.fd.LocalAddr()
}

// Disconnect terminates the peer connection with the given reason.
// It returns immediately and does not wait until the connection is closed.
func (p *Peer) Disconnect(reason DiscReason) {
	select {
	case p.disc <- reason:
	case <-p.closed:
	}
}

// String implements fmt.Stringer.
func (p *Peer) String() string {
	return fmt.Sprintf("Peer %x %v", p.rw.id[:8], p.RemoteAddr())
}

// Inbound returns true if the peer is an inbound connection
func (p *Peer) Inbound() bool {
	return p.rw.flags&inboundConn != 0
}

func newPeer(conn *conn, protocols []Protocol) *Peer {
	protomap := matchProtocols(protocols, conn.caps, conn)
	p := &Peer{
		rw:       conn,
		running:  protomap,
		created:  mclock.Now(),
		disc:     make(chan DiscReason),
		protoErr: make(chan error, len(protomap)+1), // protocols + pingLoop
		closed:   make(chan struct{}),
		log:      log.New("id", conn.id, "conn", conn.flags),
	}
	return p
}

func (p *Peer) Log() log.Logger {
	return p.log
}

func (p *Peer) run() (remoteRequested bool, err error) {
	var (
		writeStart = make(chan struct{}, 1)
		writeErr   = make(chan error, 1)
		readErr    = make(chan error, 1)
		reason     DiscReason // sent to the peer
	)
	p.wg.Add(2)
	go p.readLoop(readErr)
	go p.pingLoop()

	// Start all protocol handlers.
	writeStart <- struct{}{}
	p.startProtocols(writeStart, writeErr)

	// Wait for an error or disconnect.
loop:
	for {
		select {
		case err = <-writeErr:
			// A write finished. Allow the next write to start if
			// there was no error.
			if err != nil {
				reason = DiscNetworkError
				break loop
			}
			writeStart <- struct{}{}
		case err = <-readErr:
			if r, ok := err.(DiscReason); ok {
				remoteRequested = true
				reason = r
			} else {
				reason = DiscNetworkError
			}
			break loop
		case err = <-p.protoErr:
			reason = discReasonForError(err)
			break loop
		case err = <-p.disc:
			break loop
		}
	}

	close(p.closed)
	p.rw.close(reason)
	p.wg.Wait()
	return remoteRequested, err
}

func (p *Peer) pingLoop() {
	ping := time.NewTimer(pingInterval)
	defer p.wg.Done()
	defer ping.Stop()
	for {
		select {
		case <-ping.C:
			if err := SendItems(p.rw, pingMsg); err != nil {
				p.protoErr <- err
				return
			}
			ping.Reset(pingInterval)
		case <-p.closed:
			return
		}
	}
}

func (p *Peer) readLoop(errc chan<- error) {
	defer p.wg.Done()
	for {
		msg, err := p.rw.ReadMsg()
		if err != nil {
			errc <- err
			return
		}
		msg.ReceivedAt = time.Now()
		if err = p.handle(msg); err != nil {
			errc <- err
			return
		}
	}
}

func (p *Peer) handle(msg Msg) error {
	switch {
	case msg.Code == pingMsg:
		msg.Discard()
		go SendItems(p.rw, pongMsg)
	case msg.Code == discMsg:
		var reason [1]DiscReason
		// This is the last message. We don't need to discard or
		// check errors because, the connection will be closed after it.
		rlp.Decode(msg.Payload, &reason)
		return reason[0]
	case msg.Code == BroadcastNewChildChainMsg:
		// Got New Child Chain message from peer
		var chainId string
		if err := msg.Decode(&chainId); err != nil {
			return err
		}

		p.log.Infof("Got new child chain msg from Peer %v, Before add protocol. Caps %v, Running Proto %+v", p.String(), p.Caps(), p.Info().Protocols)

		newRunning := p.checkAndUpdateProtocol(chainId)
		if newRunning {
			// Add new protocol to peer, tell back to the peer
			go Send(p.rw, ConfirmNewChildChainMsg, chainId)
		}

		// Add the cap to the peer, start the protocol
		p.log.Infof("Got new child chain msg After add protocol. Caps %v, Running Proto %+v", p.Caps(), p.Info().Protocols)

	case msg.Code == ConfirmNewChildChainMsg:
		// Got New Child Chain message from peer
		var chainId string
		if err := msg.Decode(&chainId); err != nil {
			return err
		}
		p.log.Infof("Got confirm msg from Peer %v, Before add protocol. Caps %v, Running Proto %+v", p.String(), p.Caps(), p.Info().Protocols)
		p.checkAndUpdateProtocol(chainId)
		p.log.Infof("Got confirm msg After add protocol. Caps %v, Running Proto %+v", p.Caps(), p.Info().Protocols)

	case msg.Code == RefreshValidatorNodeInfoMsg:
		p.log.Debug("Got refresh validation node infomation")
		var valNodeInfo P2PValidatorNodeInfo
		if err := msg.Decode(&valNodeInfo); err != nil {
			p.log.Debugf("decode error: %v", err)
			return err
		}
		p.log.Debugf("validation node address: %x", valNodeInfo.Validator.Address)

		if valNodeInfo.Original && p.Info().ID == valNodeInfo.Node.ID.String() {
			valNodeInfo.Node.IP = p.RemoteAddr().(*net.TCPAddr).IP
		}
		valNodeInfo.Original = false

		p.log.Debugf("validator node info: %v", valNodeInfo)

		data, err := rlp.EncodeToBytes(valNodeInfo)
		if err != nil {
			p.log.Debugf("encode error: %v", err)
			return err
		}
		p.events.Send(&PeerEvent{
			Type:     PeerEventTypeRefreshValidator,
			Peer:     p.ID(),
			Protocol: string(data),
		})

		p.log.Debugf("RefreshValidatorNodeInfoMsg handled")

	case msg.Code == RemoveValidatorNodeInfoMsg:
		p.log.Debug("Got remove validation node infomation")
		var valNodeInfo P2PValidatorNodeInfo
		if err := msg.Decode(&valNodeInfo); err != nil {
			p.log.Debugf("decode error: %v", err)
			return err
		}
		p.log.Debugf("validation node address: %x", valNodeInfo.Validator.Address)

		if valNodeInfo.Original {
			valNodeInfo.Node.IP = p.RemoteAddr().(*net.TCPAddr).IP
			valNodeInfo.Original = false
		}
		p.log.Debugf("validator node info: %v", valNodeInfo)

		data, err := rlp.EncodeToBytes(valNodeInfo)
		if err != nil {
			p.log.Debugf("encode error: %v", err)
			return err
		}
		p.events.Send(&PeerEvent{
			Type:     PeerEventTypeRemoveValidator,
			Peer:     p.ID(),
			Protocol: string(data),
		})

		p.log.Debug("RemoveValidatorNodeInfoMsg handled")

	case msg.Code < baseProtocolLength:
		// ignore other base protocol messages
		return msg.Discard()
	default:
		// it's a subprotocol message
		proto, err := p.getProto(msg.Code)
		if err != nil {
			return fmt.Errorf("msg code out of range: %v", msg.Code)
		}
		select {
		case proto.in <- msg:
			return nil
		case <-p.closed:
			return io.EOF
		}
	}
	return nil
}

func (p *Peer) checkAndUpdateProtocol(chainId string) bool {

	childProtocolName := "pchain_" + chainId

	// Check childChainId already added
	// TODO Protoect the changing of running under multi-thread env
	if _, exist := p.running[childProtocolName]; exist {
		p.log.Infof("Child Chain %v is already running on peer", childProtocolName)
		return false
	}

	// Check we are support the same child chain or not
	childProtocolOffset := getLargestOffset(p.running)
	if match, protoRW := matchServerProtocol(*p.srvProtocols, childProtocolName, childProtocolOffset, p.rw); match {
		// Start the ProtoRW and add it to running protoRW
		p.startChildChainProtocol(protoRW)
		// Add the protoRW to peer
		p.running[childProtocolName] = protoRW

		protoCap := protoRW.cap()
		capExist := false
		for _, cap := range p.rw.caps {
			if cap.Name == protoCap.Name && cap.Version == protoCap.Version {
				capExist = true
			}
		}
		if !capExist {
			p.rw.caps = append(p.rw.caps, protoCap)
		}
		return true
	}

	p.log.Infof("No Local Server Protocol matched, perhaps local server has not start the child chain %v yet.", childProtocolName)
	return false
}

func countMatchingProtocols(protocols []Protocol, caps []Cap) int {
	n := 0
	for _, cap := range caps {
		for _, proto := range protocols {
			if proto.Name == cap.Name && proto.Version == cap.Version {
				n++
			}
		}
	}
	return n
}

// matchProtocols creates structures for matching named subprotocols.
func matchProtocols(protocols []Protocol, caps []Cap, rw MsgReadWriter) map[string]*protoRW {
	sort.Sort(capsByNameAndVersion(caps))
	offset := baseProtocolLength
	result := make(map[string]*protoRW)

outer:
	for _, cap := range caps {
		for _, proto := range protocols {
			if proto.Name == cap.Name && proto.Version == cap.Version {
				// If an old protocol version matched, revert it
				if old := result[cap.Name]; old != nil {
					offset -= old.Length
				}
				// Assign the new match
				result[cap.Name] = &protoRW{Protocol: proto, offset: offset, in: make(chan Msg), w: rw}
				offset += proto.Length

				continue outer
			}
		}
	}
	return result
}

// matchServerProtocol creates structures for matching named subprotocols.
func matchServerProtocol(protocols []Protocol, name string, offset uint64, rw MsgReadWriter) (bool, *protoRW) {
	for _, proto := range protocols {
		if proto.Name == name {
			// return the new protoRW
			return true, &protoRW{Protocol: proto, offset: offset, in: make(chan Msg), w: rw}
		}
	}
	return false, nil
}

func getLargestOffset(running map[string]*protoRW) uint64 {
	var largestOffset uint64 = 0
	for _, proto := range running {
		offsetEnd := proto.offset + proto.Length
		if offsetEnd > largestOffset {
			largestOffset = offsetEnd
		}
	}
	return largestOffset
}

func (p *Peer) startProtocols(writeStart <-chan struct{}, writeErr chan<- error) {
	p.wg.Add(len(p.running))
	for _, proto := range p.running {
		proto := proto
		proto.closed = p.closed
		proto.wstart = writeStart
		proto.werr = writeErr
		var rw MsgReadWriter = proto
		if p.events != nil {
			rw = newMsgEventer(rw, p.events, p.ID(), proto.Name)
		}
		p.log.Trace(fmt.Sprintf("Starting protocol %s/%d", proto.Name, proto.Version))
		go func() {
			err := proto.Run(p, rw)
			if err == nil {
				p.log.Trace(fmt.Sprintf("Protocol %s/%d returned", proto.Name, proto.Version))
				err = errProtocolReturned
			} else if err != io.EOF {
				p.log.Trace(fmt.Sprintf("Protocol %s/%d failed", proto.Name, proto.Version), "err", err)
			}
			p.protoErr <- err
			p.wg.Done()
		}()
	}
}

func (p *Peer) startChildChainProtocol(proto *protoRW) {
	p.wg.Add(1)

	proto.closed = p.closed
	proto.wstart = p.running["pchain"].wstart
	proto.werr = p.running["pchain"].werr

	var rw MsgReadWriter = proto
	if p.events != nil {
		rw = newMsgEventer(rw, p.events, p.ID(), proto.Name)
	}
	p.log.Trace(fmt.Sprintf("Starting protocol %s/%d", proto.Name, proto.Version))
	go func() {
		err := proto.Run(p, rw)
		if err == nil {
			p.log.Trace(fmt.Sprintf("Protocol %s/%d returned", proto.Name, proto.Version))
			err = errProtocolReturned
		} else if err != io.EOF {
			p.log.Trace(fmt.Sprintf("Protocol %s/%d failed", proto.Name, proto.Version), "err", err)
		}
		p.protoErr <- err
		p.wg.Done()
	}()
}

// getProto finds the protocol responsible for handling
// the given message code.
func (p *Peer) getProto(code uint64) (*protoRW, error) {
	for _, proto := range p.running {
		if code >= proto.offset && code < proto.offset+proto.Length {
			return proto, nil
		}
	}
	return nil, newPeerError(errInvalidMsgCode, "%d", code)
}

type protoRW struct {
	Protocol
	in     chan Msg        // receices read messages
	closed <-chan struct{} // receives when peer is shutting down
	wstart <-chan struct{} // receives when write may start
	werr   chan<- error    // for write results
	offset uint64
	w      MsgWriter
}

func (rw *protoRW) WriteMsg(msg Msg) (err error) {
	if msg.Code >= rw.Length {
		return newPeerError(errInvalidMsgCode, "not handled")
	}
	msg.Code += rw.offset
	select {
	case <-rw.wstart:
		err = rw.w.WriteMsg(msg)
		// Report write status back to Peer.run. It will initiate
		// shutdown if the error is non-nil and unblock the next write
		// otherwise. The calling protocol code should exit for errors
		// as well but we don't want to rely on that.
		rw.werr <- err
	case <-rw.closed:
		err = fmt.Errorf("shutting down")
	}
	return err
}

func (rw *protoRW) ReadMsg() (Msg, error) {
	select {
	case msg := <-rw.in:
		msg.Code -= rw.offset
		return msg, nil
	case <-rw.closed:
		return Msg{}, io.EOF
	}
}

// PeerInfo represents a short summary of the information known about a connected
// peer. Sub-protocol independent fields are contained and initialized here, with
// protocol specifics delegated to all connected sub-protocols.
type PeerInfo struct {
	ID           string   `json:"id"`   // Unique node identifier (also the encryption key)
	PIAddress    string   `json:"piAddress"`   // consensus pub key
	ConsensusKey string   `json:"consensusPubKey"`   // consensus pub key
	IsValidator  bool     `json:"isValidator"`   // is validator in current epoch current chain
	Name         string   `json:"name"` // Name of the node, including client type, version, OS, custom data
	Caps         []string `json:"caps"` // Sum-protocols advertised by this particular peer
	Network struct {
		LocalAddress  string `json:"localAddress"`  // Local endpoint of the TCP data connection
		RemoteAddress string `json:"remoteAddress"` // Remote endpoint of the TCP data connection
		Inbound       bool   `json:"inbound"`
		Trusted       bool   `json:"trusted"`
		Static        bool   `json:"static"`
	} `json:"network"`
	Protocols map[string]interface{} `json:"protocols"` // Sub-protocol specific metadata fields
}

// Info gathers and returns a collection of metadata known about a peer.
func (p *Peer) Info() *PeerInfo {
	// Gather the protocol capabilities
	var caps []string
	for _, cap := range p.Caps() {
		caps = append(caps, cap.String())
	}
	// Assemble the generic peer metadata
	info := &PeerInfo{
		ID:        p.ID().String(),
		Name:      p.Name(),
		Caps:      caps,
		Protocols: make(map[string]interface{}),
	}
	info.Network.LocalAddress = p.LocalAddr().String()
	info.Network.RemoteAddress = p.RemoteAddr().String()
	info.Network.Inbound = p.rw.is(inboundConn)
	info.Network.Trusted = p.rw.is(trustedConn)
	info.Network.Static = p.rw.is(staticDialedConn)

	// Gather all the running protocol infos
	for _, proto := range p.running {
		protoInfo := interface{}("unknown")
		if query := proto.Protocol.PeerInfo; query != nil {
			if metadata := query(p.ID()); metadata != nil {
				protoInfo = metadata
			} else {
				protoInfo = "handshake"
			}
		}
		info.Protocols[proto.Name] = protoInfo
	}
	return info
}


// Info gathers and returns a collection of metadata known about a peer.
func (p *Peer) Info2(piAddr common.Address, consensusKey string, isValidator  bool) *PeerInfo {
	// Gather the protocol capabilities
	var caps []string
	for _, cap := range p.Caps() {
		caps = append(caps, cap.String())
	}
	// Assemble the generic peer metadata
	info := &PeerInfo{
		ID:        p.ID().String(),
		Name:      p.Name(),
		PIAddress: piAddr.Hex(),
		ConsensusKey: consensusKey,
		IsValidator: isValidator,
		Caps:      caps,
		Protocols: make(map[string]interface{}),
	}
	info.Network.LocalAddress = p.LocalAddr().String()
	info.Network.RemoteAddress = p.RemoteAddr().String()
	info.Network.Inbound = p.rw.is(inboundConn)
	info.Network.Trusted = p.rw.is(trustedConn)
	info.Network.Static = p.rw.is(staticDialedConn)

	// Gather all the running protocol infos
	for _, proto := range p.running {
		protoInfo := interface{}("unknown")
		if query := proto.Protocol.PeerInfo; query != nil {
			if metadata := query(p.ID()); metadata != nil {
				protoInfo = metadata
			} else {
				protoInfo = "handshake"
			}
		}
		info.Protocols[proto.Name] = protoInfo
	}
	return info
}
