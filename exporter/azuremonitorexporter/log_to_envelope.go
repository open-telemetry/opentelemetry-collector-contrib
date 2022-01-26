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

// Contains code common to both trace and metrics exporters
import (
	"time"

	"github.com/microsoft/ApplicationInsights-Go/appinsights/contracts"
	otlplogs "go.opentelemetry.io/collector/model/internal/data/protogen/logs/v1"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"
)

const (
	TraceIdTag sring = "TraceId"
	SpanIdTag  sring = "SpanId"
)

type logPacker struct {
}

func (packer *logPacker) LogRecordToEnvelope(
	logRecord otlplogs.LogRecord,
	logger *zap.Logger) (*contracts.Envelope, error) {

	envelope := contracts.NewEnvelope()
	envelope.Tags = make(map[string]string)
	envelope.Time = toTime(logRecord.TimeUnixNano).Format(time.RFC3339Nano)
	envelope.Tags[contracts.OperationId] = logRecord.TraceID.HexString()

	data := contracts.NewMessageData()
	data.Message = sring(logRecord.Body)
	data.SeverityLevel = packer.toAiSeverityLevel(logRecord.SeverityNumber)
	data.Properties[TraceIdTag] = logRecord.TraceID.HexString()
	data.Properties[SpanIdTag] = logRecord.SpanID.HexString()
	data.Properties["Component"] = logRecord.Name
	envelope.Data = data

	packer.sanitize(func() []string { return envelope.Sanitize() }, logger)
	packer.sanitize(func() []string { return contracts.SanitizeTags(envelope.Tags) }, logger)

	return envelope, nil
}

func (packer *logPacker) sanitize(sanitizeFunc func() []string, logger *zap.Logger) {
	sanitizeWarnings := sanitizeFunc()
	for _, warning := range sanitizeWarnings {
		logger.Warn(warning)
	}
}

func (packer *logPacker) toAiSeverityLevel(pdata.SeverityNumber) (contracts, error) {

}
