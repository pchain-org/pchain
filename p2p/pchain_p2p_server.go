package p2p

import (
	"github.com/ethereum/go-ethereum/cmd/utils"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/p2p"
	"gopkg.in/urfave/cli.v1"
)

type PChainP2PServer struct {
	serverConfig p2p.Config
	server       *p2p.Server
}

func NewP2PServer(ctx *cli.Context) *PChainP2PServer {

	// Load Default P2P config
	config := &node.Config{
		GeneralDataDir: utils.MakeDataDir(ctx),
		DataDir:        utils.MakeDataDir(ctx), // Just for pass the check, P2P always use GeneralDataDir
		P2P:            node.DefaultConfig.P2P,
	}

	// Setup the config from context
	utils.SetP2PConfig(ctx, &config.P2P)

	// Initialize the p2p server. This creates the node key and
	// discovery databases.
	serverConfig := config.P2P
	serverConfig.PrivateKey = config.NodeKey()
	serverConfig.Name = config.NodeName()
	if serverConfig.StaticNodes == nil {
		serverConfig.StaticNodes = config.StaticNodes()
	}
	if serverConfig.TrustedNodes == nil {
		serverConfig.TrustedNodes = config.TrustedNodes()
	}
	if serverConfig.NodeDatabase == "" {
		serverConfig.NodeDatabase = config.NodeDB()
	}
	running := &p2p.Server{Config: serverConfig}
	log.Info("Create peer-to-peer node", "instance", serverConfig.Name)

	return &PChainP2PServer{
		serverConfig: serverConfig,
		server:       running,
	}
}

func (srv *PChainP2PServer) Server() *p2p.Server {
	return srv.server
}

func (srv *PChainP2PServer) Stop() {
	srv.server.Stop()
}
