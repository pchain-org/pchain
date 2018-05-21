package rpc

import (
	"net"
	"net/http"
	"github.com/tendermint/go-rpc/server"
	"github.com/ethereum/go-ethereum/logger/glog"
	"fmt"
	"gopkg.in/urfave/cli.v1"
	"github.com/ethereum/go-ethereum/cmd/utils"
	"strconv"
)

var listeners map[string]net.Listener
var muxes map[string]*http.ServeMux

func Hookup(chainId string, handler http.Handler) {

	fmt.Printf("pchain StartRPC for (chainId, rpchandler): (%v, %v)\n", chainId, handler)
	for listenAddr, _ := range listeners {

		if handler != nil {
			muxes[listenAddr].Handle("/" + chainId, handler)
		} else {
			muxes[listenAddr].Handle("/" + chainId, defaultHandler())
		}
	}
}

func Takeoff(chainId string) {

	fmt.Printf("pchain StartRPC for chainId: %v\n", chainId)
	for listenAddr, _ := range listeners {

		muxes[listenAddr].Handle("/" + chainId, defaultHandler())
	}
}

func StartRPC(ctx *cli.Context) error {

	host := utils.MakeHTTPRpcHost(ctx)
	port := ctx.GlobalInt(utils.RPCPortFlag.Name)

	listenAddrs := []string{"tcp://" + host + ":" + strconv.Itoa(port)}

	// we may expose the rpc over both a unix and tcp socket
	listeners = make(map[string]net.Listener)
	muxes = make(map[string]*http.ServeMux)
	for _, listenAddr := range listenAddrs {

		mux := http.NewServeMux()
		listener, err := rpcserver.StartHTTPServer(listenAddr, mux)
		if err != nil {
			return err
		}
		listeners[listenAddr] = listener
		muxes[listenAddr] = mux
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