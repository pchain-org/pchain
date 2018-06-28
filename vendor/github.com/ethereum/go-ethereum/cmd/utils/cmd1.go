package utils

import (
	"fmt"
	"os"
	"os/signal"
	"github.com/ethereum/go-ethereum/internal/debug"
	"github.com/ethereum/go-ethereum/logger"
	"github.com/ethereum/go-ethereum/logger/glog"
	"github.com/ethereum/go-ethereum/node"
)

func StartNode1(stack *node.Node) error{

	//emmark
	fmt.Println("StartNode->stack.Start()")
	if err := stack.Start1(); err != nil {
		Fatalf("Error starting protocol stack: %v", err)
	}

	fmt.Println("pow recover 3")
	go func() {
		sigc := make(chan os.Signal, 1)
		signal.Notify(sigc, os.Interrupt)
		defer signal.Stop(sigc)
		<-sigc
		glog.V(logger.Info).Infoln("Got interrupt, shutting down...")
		go stack.Stop()
		for i := 10; i > 0; i-- {
			<-sigc
			if i > 1 {
				glog.V(logger.Info).Infof("Already shutting down, interrupt %d more times for panic.", i-1)
			}
		}
		fmt.Println("pow recover 4")
		debug.Exit() // ensure trace and CPU profile data is flushed.
		debug.LoudPanic("boom")
	}()
	fmt.Println("pow recover 5")
	return nil
}

