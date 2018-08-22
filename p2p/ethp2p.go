package p2p

import (
	eth "github.com/ethereum/go-ethereum/node"
	ethp2p "github.com/ethereum/go-ethereum/p2p"
)

type EthP2PServer struct {
	server       *ethp2p.Server
	mainConfig   ethp2p.Config
	mainProtocol []ethp2p.Protocol
	//we should put child chain's static-nodes, trusted-nodes to main chain's config
	//here just remember all child chain's config, and only main chain's config is applied
	//should remove this configs map later
	configs   map[string]ethp2p.Config
	protocols map[string][]ethp2p.Protocol
}

//this function should be called with main chain's configuration
func NewEthP2PServer(mainNode *eth.Node) *EthP2PServer {

	config, protocols, err := mainNode.GatherP2PConfigAndProtocols()
	if err != nil {
		return nil
	}

	server := &ethp2p.Server{Config: config}
	server.Protocols = append(server.Protocols, protocols...)

	p2pSrv := &EthP2PServer{
		server:       server,
		mainConfig:   config,
		mainProtocol: protocols,
		configs:      make(map[string]ethp2p.Config),
		protocols:    make(map[string][]ethp2p.Protocol),
	}

	mainNode.SetP2PServer(p2pSrv)

	return p2pSrv
}

func (srv *EthP2PServer) AddNodeConfig(chainId string, node *eth.Node) error {

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

func (srv *EthP2PServer) Start() error {
	return srv.server.Start()
}

func (srv *EthP2PServer) Stop() {
	srv.server.Stop()
}

//this function will restart the p2p server, it should only be called when add a node dynamically
//at the initial stage of pchain system, all config/procotols should be gathered and applied together
func (srv *EthP2PServer) Hookup(chainId string, node *eth.Node) error {

	srv.AddNodeConfig(chainId, node)

	srv.Stop()
	srv.Start()

	return nil
}

func (srv *EthP2PServer) Takeoff(chainId string) error {
	return nil
}

func (srv *EthP2PServer) Server() *ethp2p.Server {
	return srv.server
}
