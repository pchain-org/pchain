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

var listenAddrs map[string]interface{}
var listeners map[string]net.Listener
var muxes map[string]*http.ServeMux

func Hookup(chainId string, handler http.Handler) error{

	fmt.Printf("Hookup RPC for (chainId, rpchandler): (%v, %v)\n", chainId, handler)
	for addr, _ := range listenAddrs {

		listeners[addr].Close()
		mux := muxes[addr]

		if handler != nil {
			muxes[addr].Handle("/" + chainId, handler)
		} else {
			muxes[addr].Handle("/" + chainId, defaultHandler())
		}

		listener, err := rpcserver.StartHTTPServer(addr, mux)
		if err != nil {
			return err
		}
		listeners[addr] = listener
	}

	return nil
}

func Takeoff(chainId string) error{

	fmt.Printf("Takeoff RPC for chainId: %v\n", chainId)
	for addr, _ := range listenAddrs {

		listeners[addr].Close()
		mux := muxes[addr]

		mux.Handle("/" + chainId, defaultHandler())

		listener, err := rpcserver.StartHTTPServer(addr, mux)
		if err != nil {
			return err
		}

		listeners[addr] = listener
	}

	return nil
}

func StartRPC(ctx *cli.Context) error {

	host := utils.MakeHTTPRpcHost(ctx)
	port := ctx.GlobalInt(utils.RPCPortFlag.Name)

	addrArr := []string{"tcp://" + host + ":" + strconv.Itoa(port)}

	// we may expose the rpc over both a unix and tcp socket
	listenAddrs = make(map[string]interface{})
	listeners = make(map[string]net.Listener)
	muxes = make(map[string]*http.ServeMux)

	for _, addr := range addrArr {

		mux := http.NewServeMux()
		listener, err := rpcserver.StartHTTPServer(addr, mux)
		if err != nil {
			return err
		}

		listenAddrs[addr] = true
		listeners[addr] = listener
		muxes[addr] = mux
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
