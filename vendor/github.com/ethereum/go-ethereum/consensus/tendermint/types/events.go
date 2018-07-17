package types

import (
	// for registering TMEventData as events.EventData
	. "github.com/tendermint/go-common"
	"github.com/tendermint/go-events"
	"github.com/tendermint/go-wire"
	ethTypes "github.com/ethereum/go-ethereum/core/types"
)

// Functions to generate eventId strings

// Reserved
func EventStringBond() string    { return "Bond" }
func EventStringUnbond() string  { return "Unbond" }
func EventStringRebond() string  { return "Rebond" }
func EventStringDupeout() string { return "Dupeout" }
func EventStringFork() string    { return "Fork" }
func EventStringTx(tx Tx) string { return Fmt("Tx:%X", tx.Hash()) }

func EventStringNewBlock() string         { return "NewBlock" }
func EventStringNewBlockHeader() string   { return "NewBlockHeader" }
func EventStringNewRound() string         { return "NewRound" }
func EventStringNewRoundStep() string     { return "NewRoundStep" }
func EventStringTimeoutPropose() string   { return "TimeoutPropose" }
func EventStringCompleteProposal() string { return "CompleteProposal" }
func EventStringPolka() string            { return "Polka" }
func EventStringUnlock() string           { return "Unlock" }
func EventStringLock() string             { return "Lock" }
func EventStringRelock() string           { return "Relock" }
func EventStringTimeoutWait() string      { return "TimeoutWait" }
func EventStringVote() string             { return "Vote" }

func EventStringRequest() string          { return "Request" }
func EventStringMessage() string          { return "Message" }
func EventStringFinalCommitted() string   { return "FinalCommitted" }

//----------------------------------------

// implements events.EventData
type TMEventData interface {
	events.EventData
	AssertIsTMEventData()
}

const (
	EventDataTypeNewBlock       = byte(0x01)
	EventDataTypeFork           = byte(0x02)
	EventDataTypeTx             = byte(0x03)
	EventDataTypeNewBlockHeader = byte(0x04)

	EventDataTypeRoundState = byte(0x11)
	EventDataTypeVote       = byte(0x12)

	EventDataTypeRequest		= byte(0x21)
	EventDataTypeMessage		= byte(0x22)
	EventDataTypeFinalCommitted	= byte(0x23)
)

var _ = wire.RegisterInterface(
	struct{ TMEventData }{},
	wire.ConcreteType{EventDataNewBlock{}, EventDataTypeNewBlock},
	wire.ConcreteType{EventDataNewBlockHeader{}, EventDataTypeNewBlockHeader},
	// wire.ConcreteType{EventDataFork{}, EventDataTypeFork },
	wire.ConcreteType{EventDataTx{}, EventDataTypeTx},
	wire.ConcreteType{EventDataRoundState{}, EventDataTypeRoundState},
	wire.ConcreteType{EventDataVote{}, EventDataTypeVote},

	wire.ConcreteType{EventDataRequest{}, EventDataTypeRequest},
	wire.ConcreteType{EventDataMessage{}, EventDataTypeMessage},
	wire.ConcreteType{EventDataFinalCommitted{}, EventDataTypeFinalCommitted},
)


// Most event messages are basic types (a block, a transaction)
// but some (an input to a call tx or a receive) are more exotic

type EventDataNewBlock struct {
	Block *Block `json:"block"`
}

// light weight event for benchmarking
type EventDataNewBlockHeader struct {
	Height int `json:"height"`
}

// All txs fire EventDataTx
type EventDataTx struct {
	Height int           `json:"height"`
	Tx     Tx            `json:"tx"`
	Data   []byte        `json:"data"`
	Log    string        `json:"log"`
	Error  string        `json:"error"` // this is redundant information for now
}

// NOTE: This goes into the replay WAL
type EventDataRoundState struct {
	Height uint64    `json:"height"`
	Round  int    `json:"round"`
	Step   string `json:"step"`

	// private, not exposed to websockets
	RoundState interface{} `json:"-"`
}

type EventDataVote struct {
	Vote *Vote
}


// EventDataRequest is posted to propose a proposal
type EventDataRequest struct {
	Proposal *ethTypes.Block `json:"proposal"`
}

// EventDataMessage is posted for Istanbul engine communication
type EventDataMessage struct {
	Payload []byte `json:"payload"`
}

// FinalCommittedEvent is posted when a proposal is committed
type EventDataFinalCommitted struct {
}


func (_ EventDataNewBlock) AssertIsTMEventData()       {}
func (_ EventDataNewBlockHeader) AssertIsTMEventData() {}
func (_ EventDataTx) AssertIsTMEventData()             {}
func (_ EventDataRoundState) AssertIsTMEventData()     {}
func (_ EventDataVote) AssertIsTMEventData()           {}

func (_ EventDataRequest) AssertIsTMEventData()        {}
func (_ EventDataMessage) AssertIsTMEventData()        {}
func (_ EventDataFinalCommitted) AssertIsTMEventData() {}

//----------------------------------------
// Wrappers for type safety

type Fireable interface {
	events.Fireable
}

type Eventable interface {
	SetEventSwitch(EventSwitch)
}

type EventSwitch interface {
	events.EventSwitch
}

type EventCache interface {
	Fireable
	Flush()
}

func NewEventSwitch() EventSwitch {
	return events.NewEventSwitch()
}

func NewEventCache(evsw EventSwitch) EventCache {
	return events.NewEventCache(evsw)
}

// All events should be based on this FireEvent to ensure they are TMEventData
func fireEvent(fireable events.Fireable, event string, data TMEventData) {
	if fireable != nil {
		fireable.FireEvent(event, data)
	}
}

func AddListenerForEvent(evsw EventSwitch, id, event string, cb func(data TMEventData)) {
	evsw.AddListenerForEvent(id, event, func(data events.EventData) {
		cb(data.(TMEventData))
	})

}

//--- block, tx, and vote events

func FireEventNewBlock(fireable events.Fireable, block EventDataNewBlock) {
	fireEvent(fireable, EventStringNewBlock(), block)
}

func FireEventNewBlockHeader(fireable events.Fireable, header EventDataNewBlockHeader) {
	fireEvent(fireable, EventStringNewBlockHeader(), header)
}

func FireEventVote(fireable events.Fireable, vote EventDataVote) {
	fireEvent(fireable, EventStringVote(), vote)
}

func FireEventTx(fireable events.Fireable, tx EventDataTx) {
	fireEvent(fireable, EventStringTx(tx.Tx), tx)
}

//--- EventDataRoundState events

func FireEventNewRoundStep(fireable events.Fireable, rs EventDataRoundState) {
	fireEvent(fireable, EventStringNewRoundStep(), rs)
}

func FireEventTimeoutPropose(fireable events.Fireable, rs EventDataRoundState) {
	fireEvent(fireable, EventStringTimeoutPropose(), rs)
}

func FireEventTimeoutWait(fireable events.Fireable, rs EventDataRoundState) {
	fireEvent(fireable, EventStringTimeoutWait(), rs)
}

func FireEventNewRound(fireable events.Fireable, rs EventDataRoundState) {
	fireEvent(fireable, EventStringNewRound(), rs)
}

func FireEventCompleteProposal(fireable events.Fireable, rs EventDataRoundState) {
	fireEvent(fireable, EventStringCompleteProposal(), rs)
}

func FireEventPolka(fireable events.Fireable, rs EventDataRoundState) {
	fireEvent(fireable, EventStringPolka(), rs)
}

func FireEventUnlock(fireable events.Fireable, rs EventDataRoundState) {
	fireEvent(fireable, EventStringUnlock(), rs)
}

func FireEventRelock(fireable events.Fireable, rs EventDataRoundState) {
	fireEvent(fireable, EventStringRelock(), rs)
}

func FireEventLock(fireable events.Fireable, rs EventDataRoundState) {
	fireEvent(fireable, EventStringLock(), rs)
}

func FireEventRequest(fireable events.Fireable, rs EventDataRequest) {
	fireEvent(fireable, EventStringRequest(), rs)
}

func FireEventMessage(fireable events.Fireable, rs EventDataMessage) {
	fireEvent(fireable, EventStringMessage(), rs)
}

func FireEventFinalCommitted(fireable events.Fireable, rs EventDataFinalCommitted) {
	fireEvent(fireable, EventStringFinalCommitted(), rs)
}
