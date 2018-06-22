package core

import (
	rpc "github.com/tendermint/go-rpc/server"
	"github.com/tendermint/go-rpc/types"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
)

// TODO: better system than "unsafe" prefix
var Routes = map[string]*rpc.RPCFunc{
	// subscribe/unsubscribe are reserved for websocket events.
	"subscribe":   rpc.NewWSRPCFunc(SubscribeResult, "event"),
	"unsubscribe": rpc.NewWSRPCFunc(UnsubscribeResult, "event"),

	// info API
	"status":               rpc.NewRPCFunc(StatusResult, ""),
	"net_info":             rpc.NewRPCFunc(NetInfoResult, ""),
	"blockchain":           rpc.NewRPCFunc(BlockchainInfoResult, "minHeight,maxHeight"),
	"genesis":              rpc.NewRPCFunc(GenesisResult, ""),
	"block":                rpc.NewRPCFunc(BlockResult, "height"),
	"current_epochnumber":  rpc.NewRPCFunc(CurrentEpochNumberResult, ""),
	"epoch":                rpc.NewRPCFunc(EpochResult, "number"),
	"commit":               rpc.NewRPCFunc(CommitResult, "height"),
	"tx":                   rpc.NewRPCFunc(TxResult, "hash,prove"),
	"validators":           rpc.NewRPCFunc(ValidatorsResult, ""),
	"dump_consensus_state": rpc.NewRPCFunc(DumpConsensusStateResult, ""),
	"unconfirmed_txs":      rpc.NewRPCFunc(UnconfirmedTxsResult, ""),
	"num_unconfirmed_txs":  rpc.NewRPCFunc(NumUnconfirmedTxsResult, ""),

	// broadcast API
	"broadcast_tx_commit": rpc.NewRPCFunc(BroadcastTxCommitResult, "tx"),
	"broadcast_tx_sync":   rpc.NewRPCFunc(BroadcastTxSyncResult, "tx"),
	"broadcast_tx_async":  rpc.NewRPCFunc(BroadcastTxAsyncResult, "tx"),

	// abci API
	"abci_query": rpc.NewRPCFunc(ABCIQueryResult, "path,data,prove"),
	"abci_info":  rpc.NewRPCFunc(ABCIInfoResult, ""),

	// control API
	"dial_seeds":           rpc.NewRPCFunc(UnsafeDialSeedsResult, "seeds"),
	"unsafe_flush_mempool": rpc.NewRPCFunc(UnsafeFlushMempool, ""),
	"unsafe_set_config":    rpc.NewRPCFunc(UnsafeSetConfigResult, "type,key,value"),

	// profiler API
	"unsafe_start_cpu_profiler": rpc.NewRPCFunc(UnsafeStartCPUProfilerResult, "filename"),
	"unsafe_stop_cpu_profiler":  rpc.NewRPCFunc(UnsafeStopCPUProfilerResult, ""),
	"unsafe_write_heap_profile": rpc.NewRPCFunc(UnsafeWriteHeapProfileResult, "filename"),

	//validator API
	"validator_operation": rpc.NewRPCFunc(ValidatorOperationResult, "from,epoch,power,action,target,signature"),
	"validator_epoch": rpc.NewRPCFunc(ValidatorEpochResult, "address,epoch"),
	"unconfirmed_vo": rpc.NewRPCFunc(UnconfirmedValidatorsOperationResult, ""),
	"confirmed_vo": rpc.NewRPCFunc(ConfirmedValidatorsOperationResult, "epoch"),
}

func SubscribeResult(wsCtx rpctypes.WSRPCContext, event string) (ctypes.TMResult, error) {
	return Subscribe(wsCtx, event)
}

func UnsubscribeResult(wsCtx rpctypes.WSRPCContext, event string) (ctypes.TMResult, error) {
	return Unsubscribe(wsCtx, event)
}

func StatusResult(context *RPCDataContext) (ctypes.TMResult, error) {
	return Status(context)
}

func NetInfoResult(context *RPCDataContext) (ctypes.TMResult, error) {
	return NetInfo(context)
}

func UnsafeDialSeedsResult(context *RPCDataContext, seeds []string) (ctypes.TMResult, error) {
	return UnsafeDialSeeds(context, seeds)
}

func BlockchainInfoResult(context *RPCDataContext, min, max int) (ctypes.TMResult, error) {
	return BlockchainInfo(context, min, max)
}

func GenesisResult(context *RPCDataContext) (ctypes.TMResult, error) {
	return Genesis(context)
}

func BlockResult(context *RPCDataContext, height int) (ctypes.TMResult, error) {
	return Block(context, height)
}

func CurrentEpochNumberResult(context *RPCDataContext) (ctypes.TMResult, error) {
	return CurrentEpochNumber(context)
}

func EpochResult(context *RPCDataContext, number int) (ctypes.TMResult, error) {
	return Epoch(context, number)
}

func CommitResult(context *RPCDataContext, height int) (ctypes.TMResult, error) {
	return Commit(context, height)
}

func ValidatorsResult(context *RPCDataContext) (ctypes.TMResult, error) {
	return Validators(context)
}

func DumpConsensusStateResult(context *RPCDataContext) (ctypes.TMResult, error) {
	return DumpConsensusState(context)
}

func UnconfirmedTxsResult(context *RPCDataContext) (ctypes.TMResult, error) {
	return UnconfirmedTxs(context)
}

func NumUnconfirmedTxsResult(context *RPCDataContext) (ctypes.TMResult, error) {
	return NumUnconfirmedTxs(context)
}

// Tx allow user to query the transaction results. `nil` could mean the
// transaction is in the mempool, invalidated, or was not send in the first
// place.
func TxResult(context *RPCDataContext, hash []byte, prove bool) (ctypes.TMResult, error) {
	return Tx(context, hash, prove)
}

func BroadcastTxCommitResult(context *RPCDataContext, tx []byte) (ctypes.TMResult, error) {
	return BroadcastTxCommit(context, tx)
}

func BroadcastTxSyncResult(context *RPCDataContext, tx []byte) (ctypes.TMResult, error) {
	return BroadcastTxSync(context, tx)
}

func BroadcastTxAsyncResult(context *RPCDataContext, tx []byte) (ctypes.TMResult, error) {
	return BroadcastTxAsync(context, tx)
}

func ABCIQueryResult(context *RPCDataContext, path string, data []byte, prove bool) (ctypes.TMResult, error) {
	return ABCIQuery(context, path, data, prove)
}

func ABCIInfoResult(context *RPCDataContext) (ctypes.TMResult, error) {
	return ABCIInfo(context)
}

func UnsafeFlushMempoolResult(context *RPCDataContext) (ctypes.TMResult, error) {
	return UnsafeFlushMempool(context)
}

func UnsafeSetConfigResult(context *RPCDataContext, typ, key, value string) (ctypes.TMResult, error) {
	return UnsafeSetConfig(context, typ, key, value)
}

func UnsafeStartCPUProfilerResult(context *RPCDataContext, filename string) (ctypes.TMResult, error) {
	return UnsafeStartCPUProfiler(context, filename)
}

func UnsafeStopCPUProfilerResult(context *RPCDataContext) (ctypes.TMResult, error) {
	return UnsafeStopCPUProfiler(context)
}

func UnsafeWriteHeapProfileResult(context *RPCDataContext, filename string) (ctypes.TMResult, error) {
	return UnsafeWriteHeapProfile(context, filename)
}



//--------------
//author@liaoyd
func ValidatorOperationResult(context *RPCDataContext, from string, epoch int, power uint64, action string, target string, signature []byte) (ctypes.TMResult, error) {
	//fmt.Println("func ValidatorOperationResult(s string) (ctypes.TMResult, error)")
	return ValidatorOperation(context, from, epoch, power, action, target, signature)
}

func ValidatorEpochResult(context *RPCDataContext, address string, epoch int) (ctypes.TMResult, error) {
	return ValidatorEpoch(context, address, epoch)
}

func UnconfirmedValidatorsOperationResult(context *RPCDataContext) (ctypes.TMResult, error) {
	return UnconfirmedValidatorsOperation(context)
}

func ConfirmedValidatorsOperationResult(context *RPCDataContext, epoch int) (ctypes.TMResult, error) {
	return ConfirmedValidatorsOperation(context, epoch)
}
