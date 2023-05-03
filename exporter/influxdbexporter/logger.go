// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package influxdbexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/influxdbexporter"

import (
	"github.com/influxdata/influxdb-observability/common"
	"go.uber.org/zap"
)

type zapInfluxLogger struct {
	*zap.SugaredLogger
}

func newZapInfluxLogger(logger *zap.Logger) common.Logger {
	return &common.ErrorLogger{
		Logger: &zapInfluxLogger{
			logger.Sugar(),
		},
	}
}

func (l zapInfluxLogger) Debug(msg string, kv ...interface{}) {
	l.SugaredLogger.Debugw(msg, kv...)
}
