package rpcserver

import (
	"fmt"
	"net"
	"net/http"

)

func StartChannelServer(listener net.Listener, handler http.Handler) {

	log.Notice(fmt.Sprintln("Starting RPC Channel server"))

	go func() {
		res := http.Serve(
			listener,
			RecoverAndLogHandler(handler),
		)
		log.Crit("RPC Channel server stopped", "result", res)
	}()
}
