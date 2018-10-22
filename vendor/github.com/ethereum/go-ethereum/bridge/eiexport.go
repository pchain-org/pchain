package bridge

//this file export modules/functions from go-ethereum/internal
import (
	"gopkg.in/urfave/cli.v1"
	"github.com/ethereum/go-ethereum/internal/debug"
)

func Debug_Setup(ctx *cli.Context, logdir string)  error {
	return debug.Setup(ctx, logdir)
}

func Debug_Exit() {
	debug.Exit()
}