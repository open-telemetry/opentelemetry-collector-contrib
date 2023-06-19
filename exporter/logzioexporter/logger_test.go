// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package logzioexporter

import (
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestLoggerConfigs(tester *testing.T) {
	zapLogger := zap.NewExample()
	exporterLogger := hclog2ZapLogger{
		Zap:  zapLogger,
		name: loggerName,
	}

	assert.Equal(tester, exporterLogger.Name(), loggerName)
	assert.NotNil(tester, exporterLogger.Named("logger"))
	assert.NotNil(tester, exporterLogger.With("key", "val"))
	assert.NotNil(tester, exporterLogger.ResetNamed(loggerName))
	assert.NotNil(tester, exporterLogger.StandardLogger(nil))
	assert.Nil(tester, exporterLogger.StandardWriter(nil))

	assert.False(tester, exporterLogger.IsTrace())
	assert.False(tester, exporterLogger.IsDebug())
	assert.False(tester, exporterLogger.IsInfo())
	assert.False(tester, exporterLogger.IsWarn())
	assert.False(tester, exporterLogger.IsError())
}

func TestLogger(tester *testing.T) {
	zapLogger := zap.NewExample()
	exporterLogger := hclog2ZapLogger{
		Zap:  zapLogger,
		name: loggerName,
	}

	loggerFunc := func() {
		exporterLogger.Trace("Trace msg")
		exporterLogger.Debug("Debug msg")
		exporterLogger.Info("Info msg")
		exporterLogger.Warn("Warn msg")
		exporterLogger.Error("Error msg")
		exporterLogger.Log(hclog.Debug, "log msg")
	}
	assert.NotPanics(tester, loggerFunc, "did not panic")
}
