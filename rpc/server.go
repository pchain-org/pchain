package rpc

import (
	"fmt"
	"github.com/ethereum/go-ethereum/cmd/utils"
	"github.com/pchain/common/plogger"
	"github.com/tendermint/go-rpc/server"
	"gopkg.in/urfave/cli.v1"
	"net"
	"net/http"
	"strconv"
)

var logger = plogger.GetLogger("rpc")

var listenAddrs map[string]interface{}
var listeners map[string]net.Listener
var muxes map[string]*http.ServeMux

func Hookup(chainId string, handler http.Handler) error {

	fmt.Printf("Hookup RPC for (chainId, rpchandler): (%v, %v)\n", chainId, handler)
	for addr := range listenAddrs {

		//listeners[addr].Close()
		//mux := muxes[addr]

		if handler != nil {
			muxes[addr].Handle("/"+chainId, handler)
		} else {
			muxes[addr].Handle("/"+chainId, defaultHandler())
		}

		//listener, err := rpcserver.StartHTTPServer(addr, mux)
		//if err != nil {
		//	return err
		//}
		//listeners[addr] = listener
	}

	return nil
}

func Takeoff(chainId string) error {

	fmt.Printf("Takeoff RPC for chainId: %v\n", chainId)
	for addr, _ := range listenAddrs {

		listeners[addr].Close()
		mux := muxes[addr]

		mux.Handle("/"+chainId, defaultHandler())

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

/*
func StartRPC1(ctx *cli.Context, chainIds []string, handlers []http.Handler) error {

	host := utils.MakeHTTPRpcHost(ctx)
	port := ctx.GlobalInt(utils.RPCPortFlag.Name)

	listenAddrs := []string{"tcp://" + host + ":" + strconv.Itoa(port)}

	// we may expose the rpc over both a unix and tcp socket
	listeners = make(map[string]net.Listener, len(listenAddrs))
	for _, listenAddr := range listenAddrs {

		mux := http.NewServeMux()

		for j, chainId := range chainIds {
			handler := handlers[j]
			fmt.Printf("pchain StartRPC for (chainId, rpchandler): (%v, %v)\n", chainId, handler)
			if  handler != nil {
				mux.Handle("/" + chainId, handler)
			} else {
				mux.Handle("/" + chainId, handler)
			}
		}
		listener, err := rpcserver.StartHTTPServer(listenAddr, mux)
		if err != nil {
			return err
		}
		listeners[listenAddr] = listener
	}
	return nil
}
*/
func StopRPC() {
	for _, l := range listeners {
		logger.Info("Closing rpc listener", "listener", l)
		if err := l.Close(); err != nil {
			logger.Error("Error closing listener", "listener", l, "error", err)
		}
	}
}

func defaultHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		w.WriteHeader(http.StatusNotImplemented)
		w.Write([]byte(fmt.Sprintf("blank output for not existed chain\n")))
	})
}
