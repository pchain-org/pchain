package rpcserver

import (
	"errors"
	"net"
	"sync/atomic"
)

var _ net.Listener = (*ChannelListener)(nil)

// Channel Listener Implement the net.Listener
type ChannelListener struct {
	connections chan net.Conn
	closed      chan struct{}
	isClosed    uint32
}

func NewChannelListener() *ChannelListener {
	return &ChannelListener{
		connections: make(chan net.Conn),
		closed:      make(chan struct{}),
		isClosed:    0,
	}
}

func (cl *ChannelListener) Accept() (net.Conn, error) {
	select {
	case newConnection := <-cl.connections:
		return newConnection, nil
	case <-cl.closed:
		return nil, errors.New("Listener closed")
	}
}

func (cl *ChannelListener) Close() error {
	if atomic.CompareAndSwapUint32(&cl.isClosed, 0, 1) {
		close(cl.closed)
	}
	return nil
}

func (cl *ChannelListener) Addr() net.Addr {
	return channelAddr{}
}

func (cl *ChannelListener) Dial(network, addr string) (net.Conn, error) {
	select {
	case <-cl.closed:
		return nil, errors.New("Listener closed")
	default:
	}
	//Create an channel transport
	serverSide, clientSide := net.Pipe()
	//Pass half to the server
	cl.connections <- serverSide
	//Return the other half to the client
	return clientSide, nil

}

// Type Channel Address implement net.Addr
type channelAddr struct{}

func (channelAddr) Network() string {
	return "channel"
}

func (channelAddr) String() string {
	return "local"
}
