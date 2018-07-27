package p2p

import (
	ethp2p "github.com/ethereum/go-ethereum/p2p"
	eth "github.com/ethereum/go-ethereum/node"
)

type EthP2PServer struct {
	server *ethp2p.Server
	mainConfig ethp2p.Config
	mainProtocol []ethp2p.Protocol
	configs map[string]ethp2p.Config
	protocols map[string][]ethp2p.Protocol
}

func newEthP2PServer(config ethp2p.Config, protocols []ethp2p.Protocol) *EthP2PServer {

	server := &ethp2p.Server{Config: config}
	server.Protocols = append(server.Protocols, protocols...)

	p2pSrv :=  &EthP2PServer{
		server : server,
		mainConfig: config,
		mainProtocol: protocols,
		configs : make(map[string]ethp2p.Config),
		protocols: make(map[string][]ethp2p.Protocol),
	}

	return p2pSrv
}

func StartEthP2PServer(node *eth.Node) (*EthP2PServer, error) {

	config, protocols, err := node.GatherP2PConfigAndProtocols()
	if err != nil {
		return nil, err
	}
	p2pSrv := newEthP2PServer(config, protocols)
	node.SetP2PServer(p2pSrv)

	err = p2pSrv.Start()
	if err != nil {
		return nil, err
	}

	return p2pSrv, nil
}

func (srv *EthP2PServer) Start() error {
	return srv.server.Start()
}

func (srv *EthP2PServer) Stop() {
	srv.server.Stop()
}

func (srv *EthP2PServer) Hookup(chainId string, node *eth.Node) error {

	config, protocols, err := node.GatherP2PConfigAndProtocols()
	if err != nil {
		return err
	}
	srv.configs[chainId] = config
	srv.protocols[chainId] = protocols

	srv.server.Protocols = append(srv.server.Protocols, protocols...)

	node.SetP2PServer(srv)

	return nil
}

func (srv *EthP2PServer) Takeoff(chainId string) error {
	return nil
}

func (srv *EthP2PServer) Server() *ethp2p.Server {
	return srv.server
}