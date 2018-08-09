package main

import (
	"fmt"
	"io"
	"bytes"
	"testing"
	"net/http"
	"github.com/tendermint/go-crypto"
	"github.com/tendermint/go-rpc/client"
	"github.com/tendermint/go-rpc/server"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
)


func helloHttp(rw http.ResponseWriter, req *http.Request) {
	fmt.Printf("Request from: %v\n", req.RemoteAddr)
	rw.WriteHeader(http.StatusOK)
	fmt.Fprint(rw, "Hello http\n")
}

func status() *http.ServeMux {
	mux := http.NewServeMux()

	statusRPCFunc := rpcserver.NewRPCFunc(StatusResult, "")

	mapRPC := make(map[string]*rpcserver.RPCFunc)
	mapRPC["status"] = statusRPCFunc

	rpcserver.RegisterRPCFuncs(mux, mapRPC)

	return mux
}

func StatusResult() (ctypes.TMResult, error) {

	privKey := crypto.GenPrivKeyEd25519()
	return &ctypes.ResultStatus{
		NodeInfo:          nil,
		PubKey:            privKey.PubKey(),
		LatestBlockHash:   nil,
		LatestAppHash:     nil,
		LatestBlockHeight: 399,
		LatestBlockTime:   7777777777}, nil
}

func TestMemoryServer(t *testing.T) {
	//Start the HTTP server using an in memory Listener
	//listener := rpcserver.StartChannelServer(http.HandlerFunc(helloHttp)).(*rpcserver.ChannelListener)

	listener := rpcserver.NewChannelListener()

	client := rpcclient.NewChannelClient(listener)

	rpcserver.StartChannelServer(listener, status())

	//Make an HTTP request. Any domain works
	response, err := client.Get("bar")
	if err != nil {
		t.Fatal(err)
	}



	//Validate the results
	if response.StatusCode != http.StatusOK {
		t.Fatalf("Status code is %q", response.Status)
	}

	buf := &bytes.Buffer{}
	_, err = io.Copy(buf, response.Body)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(buf.String())

	//if buf.String() != "Hello http\n" {
	//	t.Fatalf("Output is %q", buf.String())
	//}

	// Test Tendermint Status API
	tmResult := new(ctypes.TMResult)
	_, err = client.Call("status", map[string]interface{}{}, tmResult)
	if err != nil {
		t.Fatal(err)
	}
	status := (*tmResult).(*ctypes.ResultStatus)
	fmt.Println(status)


	//Close the in memory listener, stopping the server
	listener.Close()

}

