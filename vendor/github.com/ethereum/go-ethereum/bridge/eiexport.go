package bridge

//this file export modules/functions from go-ethereum/internal
import (
	"github.com/ethereum/go-ethereum/internal/debug"
	"gopkg.in/urfave/cli.v1"
)

func Debug_Setup(ctx *cli.Context, logdir string) error {
	return debug.Setup(ctx, logdir)
}

func Debug_Exit() {
	debug.Exit()
}

func Debug_LoadPanic(x interface{}) {
	debug.LoudPanic(x)
}

var DebugFlags = debug.Flags
