package rpc

import (
	"fmt"
	"github.com/ethereum/go-ethereum/cmd/utils"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/rpc"
	"gopkg.in/urfave/cli.v1"
	"net"
	"net/http"
	"strings"
)

var (
	httpListener       net.Listener
	httpMux            *http.ServeMux
	httpHandlerMapping map[string]*rpc.Server

	wsListener       net.Listener
	wsMux            *http.ServeMux
	wsOrigins        []string
	wsHandlerMapping map[string]*rpc.Server
)

func StartRPC(ctx *cli.Context) error {

	// Use Default Config
	rpcConfig := node.DefaultConfig

	// Setup the config from context
	utils.SetHTTP(ctx, &rpcConfig)
	utils.SetWS(ctx, &rpcConfig)
	wsOrigins = rpcConfig.WSOrigins

	httperr := startHTTP(rpcConfig.HTTPEndpoint(), rpcConfig.HTTPCors, rpcConfig.HTTPVirtualHosts, rpcConfig.HTTPTimeouts)
	if httperr != nil {
		return httperr
	}

	wserr := startWS(rpcConfig.WSEndpoint())
	if wserr != nil {
		return wserr
	}

	return nil
}

func StopRPC() {
	// Stop HTTP Listener
	if httpListener != nil {
		httpAddr := httpListener.Addr().String()
		httpListener.Close()
		httpListener = nil
		log.Info("HTTP endpoint closed", "url", fmt.Sprintf("http://%s", httpAddr))
	}
	if httpMux != nil {
		for _, httpHandler := range httpHandlerMapping {
			httpHandler.Stop()
		}
	}

	// Stop WS Listener
	if wsListener != nil {
		wsAddr := wsListener.Addr().String()
		wsListener.Close()
		wsListener = nil
		log.Info("WebSocket endpoint closed", "url", fmt.Sprintf("ws://%s", wsAddr))
	}
	if wsMux != nil {
		for _, wsHandler := range wsHandlerMapping {
			wsHandler.Stop()
		}
	}
}

func IsHTTPRunning() bool {
	return httpListener != nil && httpMux != nil
}

func IsWSRunning() bool {
	return wsListener != nil && wsMux != nil
}

func HookupHTTP(chainId string, httpHandler *rpc.Server) error {
	if httpMux != nil {
		log.Infof("Hookup HTTP for (chainId, http Handler): (%v, %v)", chainId, httpHandler)
		if httpHandler != nil {
			httpMux.Handle("/"+chainId, httpHandler)
			httpHandlerMapping[chainId] = httpHandler
		}
	}
	return nil
}

func HookupWS(chainId string, wsHandler *rpc.Server) error {
	if wsMux != nil {
		log.Infof("Hookup WS for (chainId, ws Handler): (%v, %v)", chainId, wsHandler)
		if wsHandler != nil {
			wsMux.Handle("/"+chainId, wsHandler.WebsocketHandler(wsOrigins))
			wsHandlerMapping[chainId] = wsHandler
		}
	}
	return nil
}

func startHTTP(endpoint string, cors []string, vhosts []string, timeouts rpc.HTTPTimeouts) error {
	// Short circuit if the HTTP endpoint isn't being exposed
	if endpoint == "" {
		return nil
	}

	var err error
	httpListener, httpMux, err = startPChainHTTPEndpoint(endpoint, cors, vhosts, timeouts)
	if err != nil {
		return err
	}
	httpHandlerMapping = make(map[string]*rpc.Server)

	log.Info("HTTP endpoint opened", "url", fmt.Sprintf("http://%s", endpoint), "cors", strings.Join(cors, ","), "vhosts", strings.Join(vhosts, ","))
	return nil
}

func startPChainHTTPEndpoint(endpoint string, cors []string, vhosts []string, timeouts rpc.HTTPTimeouts) (net.Listener, *http.ServeMux, error) {
	var (
		listener net.Listener
		err      error
	)
	if listener, err = net.Listen("tcp", endpoint); err != nil {
		return nil, nil, err
	}
	mux := http.NewServeMux()
	go rpc.NewHTTPServer(cors, vhosts, timeouts, mux).Serve(listener)
	return listener, mux, err
}

func startWS(endpoint string) error {
	// Short circuit if the WS endpoint isn't being exposed
	if endpoint == "" {
		return nil
	}

	var err error
	wsListener, wsMux, err = startPChainWSEndpoint(endpoint)
	if err != nil {
		return err
	}
	wsHandlerMapping = make(map[string]*rpc.Server)

	log.Info("WebSocket endpoint opened", "url", fmt.Sprintf("ws://%s", wsListener.Addr()))
	return nil
}

func startPChainWSEndpoint(endpoint string) (net.Listener, *http.ServeMux, error) {
	var (
		listener net.Listener
		err      error
	)
	if listener, err = net.Listen("tcp", endpoint); err != nil {
		return nil, nil, err
	}
	mux := http.NewServeMux()
	wsServer := &http.Server{Handler: mux}
	go wsServer.Serve(listener)
	return listener, mux, err
}
