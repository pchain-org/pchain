package rpcserver

import (
	"github.com/ethereum/go-ethereum/log"
	"net"
	"net/http"
)

func StartChannelServer(listener net.Listener, handler http.Handler) {

	log.Info("Starting RPC Channel server")

	go func() {
		res := http.Serve(
			listener,
			RecoverAndLogHandler(handler),
		)
		log.Warn("RPC Channel server stopped", "result", res)
	}()
}
