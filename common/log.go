package common

import (
	"io"

	"os"

	log "github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
)

func getLogWriter(config *Config) (io.WriteCloser, string) {
	logFilePath := config.GetLogFilePath()
	return &lumberjack.Logger{
		Filename:   logFilePath,
		MaxSize:    50, // 50MB
		MaxBackups: 5,
		MaxAge:     30, // 30 days
		Compress:   false,
	}, logFilePath
}

func InitLogWriter(config *Config) io.WriteCloser {
	logFilePath := config.GetLogFilePath()
	if logFilePath == "-" || len(logFilePath) == 0 {
		log.SetOutput(os.Stderr)
		return os.Stderr
	} else {
		// use multi output - to output to file and stdout
		logWriter, _ := getLogWriter(config)
		mw := io.MultiWriter(os.Stderr, logWriter)
		log.SetOutput(mw)
		return logWriter
	}
}
