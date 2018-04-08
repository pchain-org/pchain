package plogger

import (
	"fmt"
	"syscall"
	"time"

	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
	"github.com/rifflock/lfshook"
	logrus "github.com/sirupsen/logrus"
)

var logger *logrus.Logger
var verbosity = logrus.InfoLevel

func GetLogger(module string) *logrus.Logger {
	if logger != nil {
		return logger
	}

	path := "/tmp/pchain.%Y%m%d-%H.log"
	writer, err := rotatelogs.New(
		path,
		rotatelogs.WithLinkName(path),
		rotatelogs.WithRotationTime(2*time.Hour),
		rotatelogs.WithMaxAge(7*24*time.Hour),
		// rotatelogs.WithClock(rotatelogs.Local),
	)

	if err != nil {
		fmt.Print("init rotatelogs error:", err)
		syscall.Exit(-1)
	}

	logger = logrus.New()
	logger.Formatter = &logrus.TextFormatter{}
	logger.Level = verbosity

	filelineHook := NewHook()
	filelineHook.Field = "file" // Customize source field name
	logger.Hooks.Add(filelineHook)

	logger.Hooks.Add(lfshook.NewHook(
		lfshook.WriterMap{
			logrus.DebugLevel: writer,
			logrus.InfoLevel:  writer,
			logrus.WarnLevel:  writer,
			logrus.ErrorLevel: writer,
			logrus.FatalLevel: writer,
			logrus.PanicLevel: writer,
		},
		&logrus.TextFormatter{},
	))

	return logger
}

func SetVerbosity(level logrus.Level) {
	verbosity = level
	if logger != nil {
		logger.SetLevel(level)
	}
}

func init() {
	GetLogger("main")
}
