package mempool

import (
	"github.com/pchain/common/plogger"
)

var logger = plogger.GetLogger("mempool")

/*
func init() {
	logger.SetHandler(
		logger.LvlFilterHandler(
			logger.LvlDebug,
			logger.BypassHandler(),
		),
	)
}
*/
