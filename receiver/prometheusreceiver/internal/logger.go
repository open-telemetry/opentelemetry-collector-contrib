// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver/internal"

import (
	gokitLog "github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"go.uber.org/zap"
)

const (
	levelKey = "level"
	msgKey   = "msg"
	errKey   = "err"
)

// NewZapToGokitLogAdapter create an adapter for zap.Logger to gokitLog.Logger
func NewZapToGokitLogAdapter(logger *zap.Logger) gokitLog.Logger {
	// need to skip two levels in order to get the correct caller
	// one for this method, the other for gokitLog
	logger = logger.WithOptions(zap.AddCallerSkip(2))
	return &zapToGokitLogAdapter{l: logger.Sugar()}
}

type zapToGokitLogAdapter struct {
	l *zap.SugaredLogger
}

type logData struct {
	level       level.Value
	msg         string
	otherFields []interface{}
}

func (w *zapToGokitLogAdapter) Log(keyvals ...interface{}) error {
	// expecting key value pairs, the number of items need to be even
	if len(keyvals)%2 == 0 {
		// Extract log level and message and log them using corresponding zap function
		ld := extractLogData(keyvals)
		logFunc := levelToFunc(w.l, ld.level)
		logFunc(ld.msg, ld.otherFields...)
	} else {
		// in case something goes wrong
		w.l.Info(keyvals...)
	}
	return nil
}

func extractLogData(keyvals []interface{}) logData {
	ld := logData{
		level: level.InfoValue(), // default
	}

	for i := 0; i < len(keyvals); i += 2 {
		key := keyvals[i]
		val := keyvals[i+1]

		if l, ok := matchLogLevel(key, val); ok {
			ld.level = l
			continue
		}

		if m, ok := matchLogMessage(key, val); ok {
			ld.msg = m
			continue
		}

		if err, ok := matchError(key, val); ok {
			ld.otherFields = append(ld.otherFields, zap.Error(err))
			continue
		}

		ld.otherFields = append(ld.otherFields, key, val)
	}

	return ld
}

// check if a given key-value pair represents go-kit log message and return it
func matchLogMessage(key interface{}, val interface{}) (string, bool) {
	if strKey, ok := key.(string); !ok || strKey != msgKey {
		return "", false
	}

	msg, ok := val.(string)
	if !ok {
		return "", false
	}
	return msg, true
}

// check if a given key-value pair represents go-kit log level and return it
func matchLogLevel(key interface{}, val interface{}) (level.Value, bool) {
	strKey, ok := key.(string)
	if !ok || strKey != levelKey {
		return nil, false
	}

	levelVal, ok := val.(level.Value)
	if !ok {
		return nil, false
	}
	return levelVal, true
}

//revive:disable:error-return

// check if a given key-value pair represents an error and return it
func matchError(key interface{}, val interface{}) (error, bool) {
	strKey, ok := key.(string)
	if !ok || strKey != errKey {
		return nil, false
	}

	err, ok := val.(error)
	if !ok {
		return nil, false
	}
	return err, true
}

//revive:enable:error-return

// find a matching zap logging function to be used for a given level
func levelToFunc(logger *zap.SugaredLogger, lvl level.Value) func(string, ...interface{}) {
	switch lvl {
	case level.DebugValue():
		return logger.Debugw
	case level.InfoValue():
		return logger.Infow
	case level.WarnValue():
		return logger.Warnw
	case level.ErrorValue():
		return logger.Errorw
	}

	// default
	return logger.Infow
}

var _ gokitLog.Logger = (*zapToGokitLogAdapter)(nil)
