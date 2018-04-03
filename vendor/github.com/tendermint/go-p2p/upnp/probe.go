package upnp

import (
	"errors"
	"fmt"
	"net"
	"time"
	// . "github.com/tendermint/go-common"
)

type UPNPCapabilities struct {
	PortMapping bool
	Hairpin     bool
}

func makeUPNPListener(intPort int, extPort int) (NAT, net.Listener, net.IP, error) {
	nat, err := Discover()
	if err != nil {
		return nil, nil, nil, errors.New(fmt.Sprintf("NAT upnp could not be discovered: %v", err))
	}
	logger.Info("ourIP: ", nat.(*upnpNAT).ourIP)

	ext, err := nat.GetExternalAddress()
	if err != nil {
		return nat, nil, nil, errors.New(fmt.Sprintf("External address error: %v", err))
	}
	logger.Info("External address: ", ext)

	port, err := nat.AddPortMapping("tcp", extPort, intPort, "Tendermint UPnP Probe", 0)
	if err != nil {
		return nat, nil, ext, errors.New(fmt.Sprintf("Port mapping error: %v", err))
	}
	logger.Info("Port mapping mapped: ", port)

	// also run the listener, open for all remote addresses.
	listener, err := net.Listen("tcp", fmt.Sprintf(":%v", intPort))
	if err != nil {
		return nat, nil, ext, errors.New(fmt.Sprintf("Error establishing listener: %v", err))
	}
	return nat, listener, ext, nil
}

func testHairpin(listener net.Listener, extAddr string) (supportsHairpin bool) {
	// Listener
	go func() {
		inConn, err := listener.Accept()
		if err != nil {
			logger.Info("Listener.Accept() error: ", err)
			return
		}
		logger.Info("Accepted incoming connection: ", inConn.LocalAddr(), "-> ", inConn.RemoteAddr())
		buf := make([]byte, 1024)
		n, err := inConn.Read(buf)
		if err != nil {
			logger.Info("Incoming connection read error: ", err)
			return
		}
		logger.Info("Incoming connection read ", n, " bytes: ", buf)
		if string(buf) == "test data" {
			supportsHairpin = true
			return
		}
	}()

	// Establish outgoing
	outConn, err := net.Dial("tcp", extAddr)
	if err != nil {
		logger.Info("Outgoing connection dial error: ", err)
		return
	}

	n, err := outConn.Write([]byte("test data"))
	if err != nil {
		logger.Info("Outgoing connection write error: ", err)
		return
	}
	logger.Info("Outgoing connection wrote ", n, " bytes")

	// Wait for data receipt
	time.Sleep(1 * time.Second)
	return
}

func Probe() (caps UPNPCapabilities, err error) {
	logger.Info("Probing for UPnP!")

	intPort, extPort := 8001, 8001

	nat, listener, ext, err := makeUPNPListener(intPort, extPort)
	if err != nil {
		return
	}
	caps.PortMapping = true

	// Deferred cleanup
	defer func() {
		err = nat.DeletePortMapping("tcp", intPort, extPort)
		if err != nil {
			logger.Warn("Port mapping delete error: ", err)
		}
		listener.Close()
	}()

	supportsHairpin := testHairpin(listener, fmt.Sprintf("%v:%v", ext, extPort))
	if supportsHairpin {
		caps.Hairpin = true
	}

	return
}
