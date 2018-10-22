package node

import (
	"errors"
	"fmt"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/rpc"
	"net/http"
	"reflect"
)

func (n *Node) RpcAPIs() []rpc.API {
	return n.rpcAPIs
}

func (n *Node) Start1() error {
	n.lock.Lock()
	defer n.lock.Unlock()

	if err := n.openDataDir(); err != nil {
		return err
	}

	// Short circuit if the node's already running
	if n.p2pServer == nil {
		return ErrNodeStopped
	}

	/*
		p2pConfig, protocols, err := n.GatherP2PConfigAndProtocols()
		if err != nil {
			return err
		}
	*/
	//running := &p2p.Server{Config: n.serverConfig}
	//n.log.Info("Starting peer-to-peer node", "instance", n.serverConfig.Name)
	/*
		for _, service := range services {
			running.Protocols = append(running.Protocols, service.Protocols()...)
		}

		if err := running.Start(); err != nil {
			return convertFileLockError(err)
		}
	*/

	//service should be gathered before
	services := n.services
	//server should be started before
	running := n.p2pServer.Server()
	// Start each of the services
	started := []reflect.Type{}
	for kind, service := range services {
		// Start the next service, stopping all previous upon failure
		if err := service.Start(running); err != nil {
			for _, kind := range started {
				services[kind].Stop()
			}
			running.Stop()

			return err
		}
		// Mark the service started for potential cleanup
		started = append(started, kind)
	}

	// Lastly start the configured RPC interfaces
	if err := n.startRPC1(services); err != nil {
		for _, service := range services {
			service.Stop()
		}
		running.Stop()
		return err
	}
	// Finish initializing the startup
	n.services = services
	n.server = running
	n.stop = make(chan struct{})

	return nil
}

func (n *Node) GatherServices() error {

	// Otherwise copy and specialize the P2P configuration
	services := make(map[reflect.Type]Service)
	for _, constructor := range n.serviceFuncs {
		// Create a new context for the particular service
		ctx := &ServiceContext{
			config:         n.config,
			services:       make(map[reflect.Type]Service),
			EventMux:       n.eventmux,
			AccountManager: n.accman,
		}
		for kind, s := range services { // copy needed for threaded access
			ctx.services[kind] = s
		}
		// Construct and save the service
		service, err := constructor(ctx)
		if err != nil {
			return err
		}
		kind := reflect.TypeOf(service)
		if _, exists := services[kind]; exists {
			return &DuplicateServiceError{Kind: kind}
		}
		services[kind] = service
	}

	n.services = services

	return nil
}

func (n *Node) GatherP2PConfigAndProtocols() (p2p.Config, []p2p.Protocol, error) {

	// Short circuit if the node's already running
	if n.server != nil {
		return p2p.Config{}, nil, ErrNodeRunning
	}

	if n.services == nil || len(n.services) == 0 {
		return p2p.Config{}, nil, errors.New("no service gathered for node yet")
	}

	// Initialize the p2p server. This creates the node key and
	// discovery databases.
	n.serverConfig = n.config.P2P
	n.serverConfig.PrivateKey = n.config.NodeKey()
	n.serverConfig.Name = n.config.NodeName()
	n.serverConfig.Logger = n.log
	if n.serverConfig.StaticNodes == nil {
		n.serverConfig.StaticNodes = n.config.StaticNodes()
	}
	if n.serverConfig.TrustedNodes == nil {
		n.serverConfig.TrustedNodes = n.config.TrustedNodes()
	}
	if n.serverConfig.NodeDatabase == "" {
		n.serverConfig.NodeDatabase = n.config.NodeDB()
	}

	//running := &p2p.Server{Config: n.serverConfig}
	n.log.Info("Starting peer-to-peer node", "instance", n.serverConfig.Name)

	// Gather the protocols and start the freshly assembled P2P server
	protocols := make([]p2p.Protocol, 0)
	for _, service := range n.services {
		protocols = append(protocols, service.Protocols()...)
	}

	return n.serverConfig, protocols, nil
}

func (n *Node) GetRPCHandler() (http.Handler, error) {

	apis := n.apis()
	for _, service := range n.services {
		apis = append(apis, service.APIs()...)
	}

	// Generate the whitelist based on the allowed modules
	whitelist := make(map[string]bool)
	for _, module := range n.config.HTTPModules {
		whitelist[module] = true
	}

	// Register all the APIs exposed by the services
	handler := rpc.NewServer()
	for _, api := range apis {
		if whitelist[api.Namespace] || (len(whitelist) == 0 && api.Public) {
			if err := handler.RegisterName(api.Namespace, api.Service); err != nil {
				return nil, err
			}
			n.log.Debug("HTTP registered", "service", api.Service, "namespace", api.Namespace)
		}
	}

	//emmark
	fmt.Println("(n *Node) startHTTP()->before rpc.NewCorsHandler(cors, handler)")

	// All listeners booted successfully
	n.httpEndpoint = ""
	n.httpListener = nil
	n.httpHandler = nil

	n.rpcAPIs = apis

	return rpc.NewCorsHandler(handler, n.config.HTTPCors), nil
}

func (n *Node) startRPC1(services map[reflect.Type]Service) error {

	// Start the various API endpoints, terminating all in case of errors
	if err := n.startInProc(n.rpcAPIs); err != nil {
		return err
	}
	if err := n.startIPC(n.rpcAPIs); err != nil {
		n.stopInProc()
		return err
	}
	if err := n.startWS(n.wsEndpoint, n.rpcAPIs, n.config.WSModules, n.config.WSOrigins, n.config.WSExposeAll); err != nil {
		n.stopHTTP()
		n.stopIPC()
		n.stopInProc()
		return err
	}

	return nil
}

func (n *Node) GetLogger() log.Logger {
	return n.log
}
