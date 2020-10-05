package logzioexporter

import (
	"testing"

	assert "github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestLoggerConfigs(tester *testing.T) {
	zapLogger := zap.NewExample()
	exporterLogger := Hclog2ZapLogger{
		Zap:  zapLogger,
		name: loggerName,
	}

	assert.Equal(tester, exporterLogger.name, loggerName)
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
