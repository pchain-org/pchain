package rpc

import (
	"fmt"
	"github.com/ethereum/go-ethereum/cmd/utils"
	"github.com/ethereum/go-ethereum/log"
	"github.com/tendermint/go-rpc/server"
	"gopkg.in/urfave/cli.v1"
	"net"
	"net/http"
	"strconv"
)

var listeners map[string]net.Listener
var muxes map[string]*http.ServeMux

func Hookup(chainId string, handler http.Handler) error {

	log.Infof("Hookup RPC for (chainId, rpc Handler): (%v, %v)", chainId, handler)
	for _, mux := range muxes {
		if handler != nil {
			mux.Handle("/"+chainId, handler)
		} else {
			mux.Handle("/"+chainId, defaultHandler())
		}
	}

	return nil
}

func StartRPC(ctx *cli.Context) error {

	host := utils.MakeHTTPRpcHost(ctx)
	port := ctx.GlobalInt(utils.RPCPortFlag.Name)

	addrArr := []string{"tcp://" + host + ":" + strconv.Itoa(port)}

	// we may expose the rpc over both a unix and tcp socket
	listeners = make(map[string]net.Listener)
	muxes = make(map[string]*http.ServeMux)

	for _, addr := range addrArr {

		mux := http.NewServeMux()
		listener, err := rpcserver.StartHTTPServer(addr, mux)
		if err != nil {
			return err
		}

		listeners[addr] = listener
		muxes[addr] = mux
	}
	return nil
}

func StopRPC() {
	for _, l := range listeners {
		log.Info("Closing rpc listener", "listener", l)
		if err := l.Close(); err != nil {
			log.Error("Error closing listener", "listener", l, "error", err)
		}
	}
}

func defaultHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		w.WriteHeader(http.StatusNotImplemented)
		w.Write([]byte(fmt.Sprintf("blank output for not existed chain\n")))
	})
}
