package node

import (
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

	// Short circuit if the node's server not set
	if n.server == nil {
		return ErrNodeStopped
	}

	//service should be gathered before
	services := n.services
	//server should be started before
	running := n.server
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

func (n *Node) GatherProtocols() []p2p.Protocol {
	// Gather the protocols and start the freshly assembled P2P server
	protocols := make([]p2p.Protocol, 0)
	for _, service := range n.services {
		protocols = append(protocols, service.Protocols()...)
	}

	return protocols
}

func (n *Node) GetRPCHandler() (http.Handler, error) {

	// Generate the whitelist based on the allowed modules
	whitelist := make(map[string]bool)
	for _, module := range n.config.HTTPModules {
		whitelist[module] = true
	}

	// Register all the APIs exposed by the services
	handler := rpc.NewServer()
	for _, api := range n.rpcAPIs {
		if whitelist[api.Namespace] || (len(whitelist) == 0 && api.Public) {
			if err := handler.RegisterName(api.Namespace, api.Service); err != nil {
				return nil, err
			}
			n.log.Debug("HTTP registered", "service", api.Service, "namespace", api.Namespace)
		}
	}

	log.Debugf("(n *Node) startHTTP()->before rpc.NewCorsHandler(cors, handler)")

	// All listeners booted successfully
	n.httpEndpoint = ""
	n.httpListener = nil
	n.httpHandler = nil

	return handler, nil
}

func (n *Node) startRPC1(services map[reflect.Type]Service) error {
	// Gather all the possible APIs to surface
	apis := n.apis()
	for _, service := range services {
		apis = append(apis, service.APIs()...)
	}

	// Start the various API endpoints, terminating all in case of errors
	if err := n.startInProc(apis); err != nil {
		return err
	}
	if err := n.startIPC(apis); err != nil {
		n.stopInProc()
		return err
	}
	if err := n.startWS(n.wsEndpoint, apis, n.config.WSModules, n.config.WSOrigins, n.config.WSExposeAll); err != nil {
		n.stopIPC()
		n.stopInProc()
		return err
	}
	// All API endpoints started successfully
	n.rpcAPIs = apis
	return nil
}

func (n *Node) GetLogger() log.Logger {
	return n.log
}
