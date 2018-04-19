package plogger

import (
	"fmt"
	"syscall"
	"time"

	"github.com/lestrrat-go/file-rotatelogs"
	"github.com/rifflock/lfshook"
	"github.com/sirupsen/logrus"
	"os"
	"runtime"
	"sync"
)

var logger *logrus.Logger
var verbosity = logrus.InfoLevel
var once sync.Once

func GetLogger(module string) *logrus.Logger {
    once.Do(func(){
    	getLogger(module)
	})
    return logger
}

func getLogger(module string) {
	if logger != nil {
		return
	}

	dir , err := initLogDir()
	if err != nil {
		fmt.Println("initLogDir error")
		syscall.Exit(-1)
	}

	fmt.Printf("log directory %s\r\n", dir)
	path := dir + "pchainabcd.%Y%m%d-%H.log"

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
	logger.Level = verbosity
	if runtime.GOOS == "windows"{
	    logger.Formatter = &logrus.TextFormatter{DisableColors:true}
	} else {
		logger.Formatter = &logrus.TextFormatter{}
	}

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

	return
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

func initLogDir() (string, error){
	path := "";
	if runtime.GOOS == "windows" {
		path = ".\\log\\"
	} else {
		path = "/tmp/pchain/log/"
	}

	_, err := os.Stat(path)
	if err == nil {
		return path, nil
	}

	if os.IsNotExist(err) {
        err = os.MkdirAll(path, os.ModePerm)
        if err == nil {
        	return path, nil
		} else {
			fmt.Println("os mkdir error")
			return "", err
		}
	}


	return "", err
}
