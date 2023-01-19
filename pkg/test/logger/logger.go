package logger

import (
	"go.uber.org/zap"
	zapobserver "go.uber.org/zap/zaptest/observer"
)

func GetLogger() (*zap.SugaredLogger, *zapobserver.ObservedLogs) {
	zapper, observer := zapobserver.New(zap.InfoLevel)
	logger := zap.New(zapper).Sugar()
	return logger, observer
}
