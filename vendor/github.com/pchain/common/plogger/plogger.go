package plogger

import (
	"fmt"
	"syscall"
	"time"

	"errors"
	"github.com/lestrrat-go/file-rotatelogs"
	"github.com/rifflock/lfshook"
	"github.com/sirupsen/logrus"
	"os"
	"path/filepath"
	"sync"
)

var logger *logrus.Logger
var verbosity = logrus.InfoLevel
var folder string
var once sync.Once

func GetLogger(module string) *logrus.Logger {
	once.Do(func() {
		getLogger(module)
	})
	return logger
}

func getLogger(module string) {
	if logger != nil {
		return
	}

	logger = logrus.New()
	logger.Level = verbosity
	logger.Formatter = &logrus.TextFormatter{ForceColors: true, FullTimestamp: true}

	filelineHook := NewHook()
	filelineHook.Field = "file" // Customize source field name
	logger.Hooks.Add(filelineHook)

	return
}

func SetVerbosity(level logrus.Level) {
	verbosity = level
	if logger != nil {
		logger.SetLevel(level)
	}
}

func SetLogFolder(logFolder string) {
	folder = logFolder
}

func initLogDir() (string, error) {

	if folder == "" {
		return "", errors.New("Please set the log folder first")
	}

	_, err := os.Stat(folder)
	if err == nil {
		return folder, nil
	}

	if os.IsNotExist(err) {
		err = os.MkdirAll(folder, os.ModePerm)
		if err == nil {
			return folder, nil
		} else {
			fmt.Println("os mkdir error")
			return "", err
		}
	}

	return "", err
}

func InitLogWriter() {
	dir, err := initLogDir()
	if err != nil {
		fmt.Println("initLogDir error")
		syscall.Exit(-1)
	}

	absPath, _ := filepath.Abs(dir)
	fmt.Printf("PChain Log Folder: %s\n", absPath)
	path := filepath.Join(dir, "pchain.%Y%m%d-%H.log")

	writer, err := rotatelogs.New(
		path,
		rotatelogs.WithRotationTime(2*time.Hour),
		rotatelogs.WithMaxAge(7*24*time.Hour),
		// rotatelogs.WithClock(rotatelogs.Local),
	)

	if err != nil {
		fmt.Print("init rotatelogs error:", err)
		syscall.Exit(-1)
	}

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
}
