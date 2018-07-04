package p2p

import (
	"errors"
	//"fmt"
	"math/rand"
	"net"
	"sync"
	"time"

	. "github.com/tendermint/go-common"
	cfg "github.com/tendermint/go-config"
	"github.com/tendermint/go-crypto"
)

const (
	reconnectAttempts = 30
	reconnectInterval = 3 * time.Second
)

//-----------------------------------------------------------------------------

/*
The `Switch` handles peer connections and exposes an API to receive incoming messages
on `Reactors`.  Each `Reactor` is responsible for handling incoming messages of one
or more `Channels`.  So while sending outgoing messages is typically performed on the peer,
incoming messages are received on the reactor.
*/
type Switch struct {
	BaseService

	config    cfg.Config
	listeners []Listener
	//reactors     map[string]Reactor
	//chDescs      []*ChannelDescriptor
	//reactorsByCh map[byte]Reactor
	reactorsByChainId map[string]*ChainRouter

	peers       *PeerSet
	dialing     *CMap
	nodeInfo    *NodeInfo             // our node info
	nodePrivKey crypto.PrivKeyEd25519 // our node privkey

	filterConnByAddr   func(net.Addr) error
	filterConnByPubKey func(crypto.PubKeyEd25519) error
}

var (
	ErrSwitchDuplicatePeer = errors.New("Duplicate peer")
	//ErrSwitchMaxPeersPerIPRange = errors.New("IP range has too many peers")
)

// NewSwitch creates a new Switch with the given config.
func NewSwitch(config cfg.Config) *Switch {
	setConfigDefaults(config)

	sw := &Switch{
		config:            config,
		reactorsByChainId: make(map[string]*ChainRouter),
		//reactors:     make(map[string]Reactor),
		//chDescs:      make([]*ChannelDescriptor, 0),
		//reactorsByCh: make(map[byte]Reactor),
		peers:    NewPeerSet(),
		dialing:  NewCMap(),
		nodeInfo: nil,
	}
	sw.BaseService = *NewBaseService(log, "P2P Switch", sw)
	return sw
}

//---------------------------------------------------------------------
// Switch setup

// AddReactor adds the given reactor to the switch.
// NOTE: Not goroutine safe.
func (sw *Switch) AddReactor(chainID, name string, reactor Reactor) Reactor {
	// Create a new Chain Router if chain id not existed
	chainRouter, ok := sw.reactorsByChainId[chainID]
	if !ok {
		chainRouter = &ChainRouter{
			reactors:     make(map[string]Reactor),
			chDescs:      make([]*ChannelDescriptor, 0),
			reactorsByCh: make(map[byte]Reactor),
		}
		sw.reactorsByChainId[chainID] = chainRouter
	}
	chainRouter.AddReactor(name, reactor)
	reactor.SetSwitch(sw)
	return reactor
}

// Reactors returns a map of reactors registered on the switch.
// NOTE: Not goroutine safe.
func (sw *Switch) Reactors(chainID string) map[string]Reactor {
	return sw.reactorsByChainId[chainID].reactors
}

// Reactor returns the reactor with the given name.
// NOTE: Not goroutine safe.
func (sw *Switch) Reactor(chainID, name string) Reactor {
	return sw.reactorsByChainId[chainID].reactors[name]
}

// ChainRouter returns the Chain Router with the Chain ID.
// NOTE: Not goroutine safe.
func (sw *Switch) ChainRouter(chainID string) *ChainRouter {
	return sw.reactorsByChainId[chainID]
}

// AddListener adds the given listener to the switch for listening to incoming peer connections.
// NOTE: Not goroutine safe.
func (sw *Switch) AddListener(l Listener) {
	sw.listeners = append(sw.listeners, l)
}

// Listeners returns the list of listeners the switch listens on.
// NOTE: Not goroutine safe.
func (sw *Switch) Listeners() []Listener {
	return sw.listeners
}

// IsListening returns true if the switch has at least one listener.
// NOTE: Not goroutine safe.
func (sw *Switch) IsListening() bool {
	return len(sw.listeners) > 0
}

// SetNodeInfo sets the switch's NodeInfo for checking compatibility and handshaking with other nodes.
// NOTE: Not goroutine safe.
func (sw *Switch) SetNodeInfo(nodeInfo *NodeInfo) {
	sw.nodeInfo = nodeInfo
}

// NodeInfo returns the switch's NodeInfo.
// NOTE: Not goroutine safe.
func (sw *Switch) NodeInfo() *NodeInfo {
	return sw.nodeInfo
}

// NOTE: Not goroutine safe.
// NOTE: Overwrites sw.nodeInfo.PubKey
func (sw *Switch) SetNodePrivKey(nodePrivKey crypto.PrivKeyEd25519) {
	sw.nodePrivKey = nodePrivKey
	if sw.nodeInfo != nil {
		sw.nodeInfo.PubKey = nodePrivKey.PubKey().(crypto.PubKeyEd25519)
	}
}

//---------------------------------------------------------------------
// Service start/stop

// OnStart implements BaseService. It starts all the reactors, peers, and listeners.
func (sw *Switch) OnStart() error {
	// Start reactors
	//TODO Change to Common Reactors, PEX, Chain Reactor will be move to StartChainReactor
	for _, chainRouter := range sw.reactorsByChainId {
		for _, reactor := range chainRouter.reactors {
			_, err := reactor.Start()
			if err != nil {
				return err
			}
		}
	}

	// Start peers
	for _, peer := range sw.peers.List() {
		sw.startInitPeer(peer)
	}
	// Start listeners
	for _, listener := range sw.listeners {
		go sw.listenerRoutine(listener)
	}
	return nil
}

// OnStop implements BaseService. It stops all listeners, peers, and reactors.
func (sw *Switch) OnStop() {
	// Stop listeners
	for _, listener := range sw.listeners {
		listener.Stop()
	}
	sw.listeners = nil
	// Stop peers
	for _, peer := range sw.peers.List() {
		peer.Stop()
		sw.peers.Remove(peer)
	}
	// Stop reactors
	for _, chainRouter := range sw.reactorsByChainId {
		for _, reactor := range chainRouter.reactors {
			reactor.Stop()
		}
	}
}

// StartChainReactor starts Reactors from specificed Chain ID
func (sw *Switch) StartChainReactor(chainID string) error {

	chainRouter, ok := sw.reactorsByChainId[chainID]
	if ok {
		// Start reactors
		for _, reactor := range chainRouter.reactors {
			_, err := reactor.Start()
			if err != nil {
				return err
			}
		}

		// Start peers for this Chain
		for _, peer := range sw.peers.List() {
			sw.startInitPeer(peer)
		}
	}
	return nil
}

// addPeer performs the Tendermint P2P handshake with a peer
// that already has a SecretConnection. If all goes well,
// it starts the peer and adds it to the switch.
// NOTE: This performs a blocking handshake before the peer is added.
// NOTE: If error is returned, caller is responsible for calling peer.CloseConn()
func (sw *Switch) AddPeer(peer *Peer) error {
	if err := sw.FilterConnByAddr(peer.Addr()); err != nil {
		return err
	}

	if err := sw.FilterConnByPubKey(peer.PubKey()); err != nil {
		return err
	}

	if err := peer.HandshakeTimeout(sw.nodeInfo, time.Duration(sw.config.GetInt(configKeyHandshakeTimeoutSeconds))*time.Second); err != nil {
		return err
	}

	// Avoid self
	if sw.nodeInfo.PubKey.Equals(peer.PubKey()) {
		return errors.New("Ignoring connection from self")
	}

	// Avoid duplicate
	if sw.peers.Has(peer.Key) {
		return ErrSwitchDuplicatePeer
	}

	// Check version, chain id
	if err := sw.nodeInfo.CompatibleWith(peer.NodeInfo); err != nil {
		return err
	}

	// All good. Start peer
	if sw.IsRunning() {
		sw.startInitPeer(peer)
	}

	// Add the peer to .peers
	// We start it first so that a peer in the list is safe to Stop.
	// It should not err since we already checked peers.Has().
	if err := sw.peers.Add(peer); err != nil {
		log.Notice("Ignoring peer", "error", err, "peer", peer)
		peer.Stop()
		return err
	}

	log.Notice("Added peer", "peer", peer)
	return nil
}

func (sw *Switch) FilterConnByAddr(addr net.Addr) error {
	if sw.filterConnByAddr != nil {
		return sw.filterConnByAddr(addr)
	}
	return nil
}

func (sw *Switch) FilterConnByPubKey(pubkey crypto.PubKeyEd25519) error {
	if sw.filterConnByPubKey != nil {
		return sw.filterConnByPubKey(pubkey)
	}
	return nil

}

func (sw *Switch) SetAddrFilter(f func(net.Addr) error) {
	sw.filterConnByAddr = f
}

func (sw *Switch) SetPubKeyFilter(f func(crypto.PubKeyEd25519) error) {
	sw.filterConnByPubKey = f
}

func (sw *Switch) startInitPeer(peer *Peer) {
	peer.Start() // spawn send/recv routines

	sameNetwork := peer.GetSameNetwork(sw.nodeInfo.Networks)
	for _, chainId := range sameNetwork {
		for _, reactor := range sw.reactorsByChainId[chainId].reactors {
			reactor.AddPeer(peer)
		}
	}
}

// Dial a list of seeds asynchronously in random order
func (sw *Switch) DialSeeds(addrBook *AddrBook, seeds []string) error {

	netAddrs, err := NewNetAddressStrings(seeds)
	if err != nil {
		return err
	}

	if addrBook != nil {
		// add seeds to `addrBook`
		ourAddrS := sw.nodeInfo.ListenAddr
		ourAddr, _ := NewNetAddressString(ourAddrS)
		for _, netAddr := range netAddrs {
			// do not add ourselves
			if netAddr.Equals(ourAddr) {
				continue
			}
			addrBook.AddAddress(netAddr, ourAddr)
		}
		addrBook.Save()
	}

	// permute the list, dial them in random order.
	perm := rand.Perm(len(netAddrs))
	for i := 0; i < len(perm); i++ {
		go func(i int) {
			time.Sleep(time.Duration(rand.Int63n(3000)) * time.Millisecond)
			j := perm[i]
			sw.dialSeed(netAddrs[j])
		}(i)
	}
	return nil
}

func (sw *Switch) dialSeed(addr *NetAddress) {
	peer, err := sw.DialPeerWithAddress(addr, true)
	if err != nil {
		log.Error("Error dialing seed", "error", err)
		return
	} else {
		log.Notice("Connected to seed", "peer", peer)
	}
}

// DialPeerWithAddress dials the given peer and runs sw.addPeer if it connects and authenticates successfully.
// If `persistent == true`, the switch will always try to reconnect to this peer if the connection ever fails.
func (sw *Switch) DialPeerWithAddress(addr *NetAddress, persistent bool) (*Peer, error) {
	sw.dialing.Set(addr.IP.String(), addr)
	defer sw.dialing.Delete(addr.IP.String())

	peer, err := newOutboundPeerWithConfig(addr, sw.reactorsByChainId, sw.StopPeerForError, sw.nodePrivKey, peerConfigFromGoConfig(sw.config))
	if err != nil {
		log.Info("Failed dialing peer", "address", addr, "error", err)
		return nil, err
	}
	if persistent {
		peer.makePersistent()
	}
	err = sw.AddPeer(peer)
	if err != nil {
		log.Info("Failed adding peer", "address", addr, "error", err)
		peer.CloseConn()
		return nil, err
	}
	log.Notice("Dialed and added peer", "address", addr, "peer", peer)
	return peer, nil
}

// IsDialing returns true if the switch is currently dialing the given ID.
func (sw *Switch) IsDialing(addr *NetAddress) bool {
	return sw.dialing.Has(addr.IP.String())
}

//---------------------------------------------------------------------
// Peers

// Broadcast runs a go routine for each attempted send, which will block trying
// to send for defaultSendTimeoutSeconds. Returns a channel which receives
// success values for each attempted send (false if times out). Channel will be
// closed once msg send to all peers.
//
// NOTE: Broadcast uses goroutines, so order of broadcast may not be preserved.
func (sw *Switch) Broadcast(chainID string, chID byte, msg interface{}) chan bool {
	successChan := make(chan bool, len(sw.peers.List()))
	log.Debug("Broadcast", "channel", chID, "msg", msg)
	var wg sync.WaitGroup
	for _, peer := range sw.peers.List() {
		// Bypass the Peer who are not in the same network
		if !peer.IsInTheSameNetwork(chainID) {
			continue
		}

		wg.Add(1)
		go func(peer *Peer) {
			defer wg.Done()
			success := peer.Send(chainID, chID, msg)
			successChan <- success
		}(peer)
	}
	go func() {
		wg.Wait()
		close(successChan)
	}()
	return successChan
}

// NumPeers returns the count of outbound/inbound and outbound-dialing peers.
func (sw *Switch) NumPeers() (outbound, inbound, dialing int) {
	peers := sw.peers.List()
	for _, peer := range peers {
		if peer.outbound {
			outbound++
		} else {
			inbound++
		}
	}
	dialing = sw.dialing.Size()
	return
}

// Peers returns the set of peers that are connected to the switch.
func (sw *Switch) Peers() IPeerSet {
	return sw.peers
}

// StopPeerForError disconnects from a peer due to external error.
// If the peer is persistent, it will attempt to reconnect.
// TODO: make record depending on reason.
func (sw *Switch) StopPeerForError(peer *Peer, reason interface{}) {
	addr := NewNetAddress(peer.Addr())
	log.Notice("Stopping peer for error", "peer", peer, "error", reason)
	sw.stopAndRemovePeer(peer, reason)

	if peer.IsPersistent() {
		go func() {
			log.Notice("Reconnecting to peer", "peer", peer)
			for i := 1; i < reconnectAttempts; i++ {
				if !sw.IsRunning() {
					return
				}

				peer, err := sw.DialPeerWithAddress(addr, true)
				if err != nil {
					if i == reconnectAttempts {
						log.Notice("Error reconnecting to peer. Giving up", "tries", i, "error", err)
						return
					}
					log.Notice("Error reconnecting to peer. Trying again", "tries", i, "error", err)
					time.Sleep(reconnectInterval)
					continue
				}

				log.Notice("Reconnected to peer", "peer", peer)
				return
			}
		}()
	}
}

// StopPeerGracefully disconnects from a peer gracefully.
// TODO: handle graceful disconnects.
func (sw *Switch) StopPeerGracefully(peer *Peer) {
	log.Notice("Stopping peer gracefully")
	sw.stopAndRemovePeer(peer, nil)
}

func (sw *Switch) stopAndRemovePeer(peer *Peer, reason interface{}) {
	sw.peers.Remove(peer)
	peer.Stop()
	for _, chainRouter := range sw.reactorsByChainId {
		for _, reactor := range chainRouter.reactors {
			reactor.RemovePeer(peer, reason)
		}
	}
}

func (sw *Switch) listenerRoutine(l Listener) {
	for {
		inConn, ok := <-l.Connections()
		if !ok {
			break
		}

		// ignore connection if we already have enough
		maxPeers := sw.config.GetInt(configKeyMaxNumPeers)
		if maxPeers <= sw.peers.Size() {
			log.Info("Ignoring inbound connection: already have enough peers", "address", inConn.RemoteAddr().String(), "numPeers", sw.peers.Size(), "max", maxPeers)
			continue
		}

		// New inbound connection!
		err := sw.addPeerWithConnectionAndConfig(inConn, peerConfigFromGoConfig(sw.config))
		if err != nil {
			log.Notice("Ignoring inbound connection: error while adding peer", "address", inConn.RemoteAddr().String(), "error", err)
			continue
		}

		// NOTE: We don't yet have the listening port of the
		// remote (if they have a listener at all).
		// The peerHandshake will handle that
	}

	// cleanup
}

//-----------------------------------------------------------------------------

func (sw *Switch) addPeerWithConnectionAndConfig(conn net.Conn, config *PeerConfig) error {
	peer, err := newInboundPeerWithConfig(conn, sw.reactorsByChainId, sw.StopPeerForError, sw.nodePrivKey, config)
	if err != nil {
		conn.Close()
		return err
	}

	if err = sw.AddPeer(peer); err != nil {
		conn.Close()
		return err
	}

	return nil
}

func peerConfigFromGoConfig(config cfg.Config) *PeerConfig {
	return &PeerConfig{
		AuthEnc:          config.GetBool(configKeyAuthEnc),
		Fuzz:             config.GetBool(configFuzzEnable),
		HandshakeTimeout: time.Duration(config.GetInt(configKeyHandshakeTimeoutSeconds)) * time.Second,
		DialTimeout:      time.Duration(config.GetInt(configKeyDialTimeoutSeconds)) * time.Second,
		MConfig: &MConnConfig{
			SendRate: int64(config.GetInt(configKeySendRate)),
			RecvRate: int64(config.GetInt(configKeyRecvRate)),
		},
		FuzzConfig: &FuzzConnConfig{
			Mode:         config.GetInt(configFuzzMode),
			MaxDelay:     time.Duration(config.GetInt(configFuzzMaxDelayMilliseconds)) * time.Millisecond,
			ProbDropRW:   config.GetFloat64(configFuzzProbDropRW),
			ProbDropConn: config.GetFloat64(configFuzzProbDropConn),
			ProbSleep:    config.GetFloat64(configFuzzProbSleep),
		},
	}
}
