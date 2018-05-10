package p2p

import (
	"net"
	"math/rand"
	"github.com/tendermint/go-crypto"
	. "github.com/tendermint/go-common"
	cfg "github.com/tendermint/go-config"
)

//------------------------------------------------------------------
// Switches connected via arbitrary net.Conn; useful for testing

// Returns n switches, connected according to the connect func.
// If connect==Connect2Switches, the switches will be fully connected.
// initSwitch defines how the ith switch should be initialized (ie. with what reactors).
// NOTE: panics if any switch fails to start.
func MakeConnectedSwitches(n int, initSwitch func(int, *Switch) *Switch, connect func([]*Switch, int, int)) []*Switch {
	switches := make([]*Switch, n)
	for i := 0; i < n; i++ {
		switches[i] = makeSwitch(i, "testing", "123.123.123", initSwitch)
	}

	if err := StartSwitches(switches); err != nil {
		panic(err)
	}

	for i := 0; i < n; i++ {
		for j := i; j < n; j++ {
			connect(switches, i, j)
		}
	}

	return switches
}

var PanicOnAddPeerErr = false

// Will connect switches i and j via net.Pipe()
// Blocks until a conection is established.
// NOTE: caller ensures i and j are within bounds
func Connect2Switches(switches []*Switch, i, j int) {
	switchI := switches[i]
	switchJ := switches[j]
	c1, c2 := net.Pipe()
	doneCh := make(chan struct{})
	go func() {
		err := switchI.addPeerWithConnection(c1)
		if PanicOnAddPeerErr && err != nil {
			panic(err)
		}
		doneCh <- struct{}{}
	}()
	go func() {
		err := switchJ.addPeerWithConnection(c2)
		if PanicOnAddPeerErr && err != nil {
			panic(err)
		}
		doneCh <- struct{}{}
	}()
	<-doneCh
	<-doneCh
}

func StartSwitches(switches []*Switch) error {
	for _, s := range switches {
		_, err := s.Start() // start switch and reactors
		if err != nil {
			return err
		}
	}
	return nil
}

func makeSwitch(i int, network, version string, initSwitch func(int, *Switch) *Switch) *Switch {
	privKey := crypto.GenPrivKeyEd25519()
	// new switch, add reactors
	// TODO: let the config be passed in?
	s := initSwitch(i, NewSwitch(cfg.NewMapConfig(nil)))
	s.SetNodeInfo(&NodeInfo{
		PubKey:     privKey.PubKey().(crypto.PubKeyEd25519),
		Moniker:    Fmt("switch%d", i),
		Networks:   NetworkSet{network: {}},
		Version:    version,
		RemoteAddr: Fmt("%v:%v", network, rand.Intn(64512)+1023),
		ListenAddr: Fmt("%v:%v", network, rand.Intn(64512)+1023),
	})
	s.SetNodePrivKey(privKey)
	return s
}

func (sw *Switch) addPeerWithConnection(conn net.Conn) error {
	peer, err := newInboundPeer(conn, sw.reactorsByChainId, sw.StopPeerForError, sw.nodePrivKey)
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
