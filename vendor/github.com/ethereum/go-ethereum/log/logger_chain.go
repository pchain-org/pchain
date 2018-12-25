package log

import (
	"github.com/mattn/go-colorable"
	"sync"
)

var loggerMap sync.Map

func newChainLogger(chainID string) Logger {

	chainLogger := &logger{[]interface{}{}, new(swapHandler)}

	loggerMap.Store(chainID, chainLogger)

	return chainLogger
}

// NewLogger Create a new Logger for a particular Chain and return it
func NewLogger(chainID, logDir string, logLevel int, fileLine bool, vmodule, backtrace string) Logger {

	// logging
	PrintOrigins(fileLine)

	output := colorable.NewColorableStdout()
	ostream := StreamHandler(output, TerminalFormat(true))
	glogger := NewGlogHandler(ostream)

	if logDir != "" {
		rfh, err := RotatingFileHandler(
			logDir,
			10*1024*1024,
			TerminalFormat(false),
			//JSONFormatOrderedEx(false, true),
		)
		if err != nil {
			panic(err)
		}
		glogger.SetHandler(MultiHandler(ostream, rfh))
	}
	glogger.Verbosity(Lvl(logLevel))
	glogger.Vmodule(vmodule)
	glogger.BacktraceAt(backtrace)

	var logger Logger
	if chainID == "" {
		logger = Root()
		logger.SetHandler(glogger)
	} else {
		logger = newChainLogger(chainID)
		logger.SetHandler(glogger)
	}

	return logger
}

// GetLogger Get Logger from stored map by using Chain ID
func GetLogger(chainID string) Logger {
	logger, find := loggerMap.Load(chainID)
	if find {
		return logger.(Logger)
	} else {
		return nil
	}
}
