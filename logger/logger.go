package logger

import (
	"log"
	"os"

	"github.com/inburst/prty/config"
)

var sharedLogger *log.Logger

func InitializeLogger() error {
	err := config.PrepApplicationCacheFolder()
	if err != nil {
		return err
	}

	logFilePath, err := config.GetLogFilePath()
	if err != nil {
		return err
	}

	// ensure file is available for writing
	f, err := os.OpenFile(logFilePath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		return err
	}

	sharedLogger = log.New(f, "prty", log.LstdFlags)
	return nil
}

func Shared() *log.Logger {
	return sharedLogger
}
