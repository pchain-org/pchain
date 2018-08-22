package rpcserver

import (
	"net"
	"net/http"
)

func StartChannelServer(listener net.Listener, handler http.Handler) {

	logger.Infoln("Starting RPC Channel server")

	go func() {
		res := http.Serve(
			listener,
			RecoverAndLogHandler(handler),
		)
		logger.Warnln("RPC Channel server stopped", "result", res)
	}()
}
