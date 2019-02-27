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
	listener net.Listener
	mux      *http.ServeMux
)

func Hookup(chainId string, handler http.Handler) error {
	if mux != nil {
		log.Infof("Hookup RPC for (chainId, rpc Handler): (%v, %v)", chainId, handler)
		if handler != nil {
			mux.Handle("/"+chainId, handler)
		} else {
			mux.Handle("/"+chainId, defaultHandler())
		}
	}

	return nil
}

func StartRPC(ctx *cli.Context) error {

	// Use Default Config
	rpcConfig := node.DefaultConfig

	// Setup the config from context
	utils.SetHTTP(ctx, &rpcConfig)

	endpoint := rpcConfig.HTTPEndpoint()
	cors := rpcConfig.HTTPCors
	vhosts := rpcConfig.HTTPVirtualHosts

	// Short circuit if the HTTP endpoint isn't being exposed
	if endpoint == "" {
		return nil
	}

	var err error
	listener, mux, err = startPChainHTTPEndpoint(endpoint, cors, vhosts, rpcConfig.HTTPTimeouts)
	if err != nil {
		return err
	}

	log.Info("HTTP endpoint opened", "url", fmt.Sprintf("http://%s", endpoint), "cors", strings.Join(cors, ","), "vhosts", strings.Join(vhosts, ","))
	return nil
}

func StopRPC() {
	if listener != nil {
		listener.Close()
		listener = nil
		log.Info("HTTP endpoint closed", "url", fmt.Sprintf("http://%s", listener.Addr().String()))
	}
}

func defaultHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		w.WriteHeader(http.StatusNotImplemented)
		w.Write([]byte(fmt.Sprintf("blank output for not existed chain\n")))
	})
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
