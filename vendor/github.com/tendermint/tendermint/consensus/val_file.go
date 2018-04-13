package consensus

import (
	"fmt"
	"time"

	auto "github.com/tendermint/go-autofile"
	. "github.com/tendermint/go-common"
	"github.com/tendermint/go-wire"
	"github.com/tendermint/tendermint/types"
)

//--------------------------------------------------------
// types and functions for savings consensus messages

type TimedVALMessage struct {
	Time time.Time  `json:"time"`
	Msg  VALMessage `json:"msg"`
}

type VALMessage interface{}

var _ = wire.RegisterInterface(
	struct{ VALMessage }{},
	// wire.ConcreteType{types.ExMsg{}, 0x01},
	wire.ConcreteType{&types.AcceptVotes{}, 0x01},
	wire.ConcreteType{&types.PreVal{}, 0x02},
)

//--------------------------------------------------------
// Simple write-ahead logger

// Write ahead logger writes msgs to disk before they are processed.
// Can be used for crash-recovery and deterministic replay
// TODO: currently the wal is overwritten during replay catchup
//   give it a mode so it's either reading or appending - must read to end to start appending again
type VAL struct {
	BaseService

	group *auto.Group
}

func NewVAL(valFile string) (*VAL, error) {
	group, err := auto.OpenGroup(valFile)
	if err != nil {
		return nil, err
	}
	val := &VAL{
		group: group,
	}
	val.BaseService = *NewBaseService(log, "VAL", val)
	_, err = val.Start()
	return val, err
}

func (val *VAL) OnStart() error {
	fmt.Println("in func (val *VAL) OnStart() error")
	size, err := val.group.Head.Size()
	if err != nil {
		return err
	} else if size == 0 {
		fmt.Println("size == 0")
		val.writeEpoch(0)
	}
	fmt.Println("size:", size)
	_, err = val.group.Start()
	return err
}

func (val *VAL) OnStop() {
	val.BaseService.OnStop()
	val.group.Stop()
}

// called in newStep and for each pass in receiveRoutine
func (val *VAL) Save(vmsg VALMessage) {
	if val == nil {
		return
	}

	// Write the wal message
	var vmsgBytes = wire.JSONBytes(TimedVALMessage{time.Now(), vmsg})
	err := val.group.WriteLine(string(vmsgBytes))
	if err != nil {
		PanicQ(Fmt("Error writing msg to consensus wal. Error: %v \n\nMessage: %v", err, vmsg))
	}
	// TODO: only flush when necessary
	if err := val.group.Flush(); err != nil {
		PanicQ(Fmt("Error flushing consensus wal buf to file. Error: %v \n", err))
	}
}

func (val *VAL) writeEpoch(epoch int) {
	val.group.WriteLine(Fmt("#EPOCH: %v", epoch))

	// TODO: only flush when necessary
	if err := val.group.Flush(); err != nil {
		PanicQ(Fmt("Error flushing consensus wal buf to file. Error: %v \n", err))
	}
}
