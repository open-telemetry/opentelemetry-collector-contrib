package prometheusexporter

import (
	"fmt"
	"go.uber.org/zap"
)

type promLogger struct {
	realLog *zap.Logger
}

func NewLog(zapLog *zap.Logger) *promLogger {
	return &promLogger{
		realLog: zapLog,
	}
}

func (l *promLogger) Println(v ...interface{}) {
	l.realLog.Error(fmt.Sprintln(v...))
}
