// Copyright OpenTelemetry Authors
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

package azuremonitorexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azuremonitorexporter"

import (
	"context"

	"os"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/model/otlp"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"
)

type logExporter struct {
	config           *Config
	transportChannel transportChannel
	logger           *zap.Logger
}

// type logVisitor struct {
// 	processed int
// 	err       error
// 	exporter  *logExporter
// }

// // Called for each tuple of Resource, InstrumentationLibrary, and Span
// func (v *traceVisitor) visit(
// 	resource pdata.Resource,
// 	instrumentationLibrary pdata.InstrumentationLibrary, span pdata.Span) (ok bool) {

// 	envelope, err := spanToEnvelope(resource, instrumentationLibrary, span, v.exporter.logger)
// 	if err != nil {
// 		// record the error and short-circuit
// 		v.err = consumererror.NewPermanent(err)
// 		return false
// 	}

// 	// apply the instrumentation key to the envelope
// 	envelope.IKey = v.exporter.config.InstrumentationKey

// 	// This is a fire and forget operation
// 	v.exporter.transportChannel.Send(envelope)
// 	v.processed++

// 	return true
// }

var logsMarshaler = otlp.NewJSONLogsMarshaler()

func (exporter *logExporter) onLogData(context context.Context, logData pdata.Logs) error {
	buf, err := logsMarshaler.MarshalLogs(logData)
	if err != nil {
		return err
	}
	os.Stdout.Write(buf[:])

	return nil
}

// Returns a new instance of the trace exporter
func newLogsExporter(config *Config, transportChannel transportChannel, set component.ExporterCreateSettings) (component.LogsExporter, error) {
	exporter := &logExporter{
		config:           config,
		transportChannel: transportChannel,
		logger:           set.Logger,
	}

	return exporterhelper.NewLogsExporter(config, set, exporter.onLogData)
}
