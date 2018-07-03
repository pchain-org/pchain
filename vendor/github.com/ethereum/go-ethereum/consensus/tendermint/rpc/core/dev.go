package core

import (
	"fmt"
	"os"
	"runtime/pprof"
	"strconv"

	ctypes "github.com/ethereum/go-ethereum/consensus/tendermint/rpc/core/types"
)

func UnsafeFlushMempool(context *RPCDataContext) (*ctypes.ResultUnsafeFlushMempool, error) {
	context.mempool.Flush()
	return &ctypes.ResultUnsafeFlushMempool{}, nil
}

func UnsafeSetConfig(context *RPCDataContext, typ, key, value string) (*ctypes.ResultUnsafeSetConfig, error) {
	switch typ {
	case "string":
		context.config.Set(key, value)
	case "int":
		val, err := strconv.Atoi(value)
		if err != nil {
			return nil, fmt.Errorf("non-integer value found. key:%s; value:%s; err:%v", key, value, err)
		}
		context.config.Set(key, val)
	case "bool":
		switch value {
		case "true":
			context.config.Set(key, true)
		case "false":
			context.config.Set(key, false)
		default:
			return nil, fmt.Errorf("bool value must be true or false. got %s", value)
		}
	default:
		return nil, fmt.Errorf("Unknown type %s", typ)
	}
	return &ctypes.ResultUnsafeSetConfig{}, nil
}

var profFile *os.File

func UnsafeStartCPUProfiler(context *RPCDataContext, filename string) (*ctypes.ResultUnsafeProfile, error) {
	var err error
	profFile, err = os.Create(filename)
	if err != nil {
		return nil, err
	}
	err = pprof.StartCPUProfile(profFile)
	if err != nil {
		return nil, err
	}
	return &ctypes.ResultUnsafeProfile{}, nil
}

func UnsafeStopCPUProfiler(context *RPCDataContext) (*ctypes.ResultUnsafeProfile, error) {
	pprof.StopCPUProfile()
	profFile.Close()
	return &ctypes.ResultUnsafeProfile{}, nil
}

func UnsafeWriteHeapProfile(context *RPCDataContext, filename string) (*ctypes.ResultUnsafeProfile, error) {
	memProfFile, err := os.Create(filename)
	if err != nil {
		return nil, err
	}
	pprof.WriteHeapProfile(memProfFile)
	memProfFile.Close()

	return &ctypes.ResultUnsafeProfile{}, nil
}
