// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
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
