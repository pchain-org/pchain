package rpc

import (
	"net"
	"net/http"
	"github.com/tendermint/go-rpc/server"
	"github.com/pchain/chain"
	"github.com/ethereum/go-ethereum/logger/glog"
	"fmt"
	"gopkg.in/urfave/cli.v1"
	"github.com/ethereum/go-ethereum/cmd/utils"
	"strconv"
)

var routes map[string]http.Handler = make(map[string]http.Handler)
var listeners []net.Listener

func Register(chainId string, handler http.Handler) {
	if chainId != "" {
		routes[chainId] = handler
	}
}

func StartRPC(ctx *cli.Context, chains []*chain.Chain) error {

	host := utils.MakeHTTPRpcHost(ctx)
	port := ctx.GlobalInt(utils.RPCPortFlag.Name)

	listenAddrs := []string{"tcp://" + host + ":" + strconv.Itoa(port)}

	// we may expose the rpc over both a unix and tcp socket
	listeners = make([]net.Listener, len(listenAddrs))
	for i, listenAddr := range listenAddrs {

		mux := http.NewServeMux()

		for _, chain := range chains {
			fmt.Printf("pchain StartRPC for (chainId, rpchandler): (%v, %v)\n", chain.Id, chain.RpcHandler)
			if chain.RpcHandler != nil {
				mux.Handle("/" + chain.Id, chain.RpcHandler)
			} else {
				mux.Handle("/" + chain.Id, defaultHandler())
			}
		}
		listener, err := rpcserver.StartHTTPServer(listenAddr, mux)
		if err != nil {
			return err
		}
		listeners[i] = listener
	}
	return nil
}

func StopRPC() {
	for _, l := range listeners {
		glog.Info("Closing rpc listener", "listener", l)
		if err := l.Close(); err != nil {
			glog.Error("Error closing listener", "listener", l, "error", err)
		}
	}
}

func defaultHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		w.WriteHeader(http.StatusNotImplemented)
		w.Write([]byte(fmt.Sprintf("blank output for not existed chain\n")))
	})
}