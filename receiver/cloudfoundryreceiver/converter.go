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

package cloudfoundryreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/cloudfoundryreceiver"

import (
	"time"

	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

const (
	attributeNamePrefix = "org.cloudfoundry."
)

func convertEnvelopeToMetrics(envelope *loggregator_v2.Envelope, metricSlice pmetric.MetricSlice, startTime time.Time) {
	namePrefix := envelope.Tags["origin"] + "."

	switch message := envelope.Message.(type) {
	case *loggregator_v2.Envelope_Log:
	case *loggregator_v2.Envelope_Counter:
		metric := metricSlice.AppendEmpty()
		metric.SetName(namePrefix + message.Counter.GetName())
		dataPoint := metric.SetEmptySum().DataPoints().AppendEmpty()
		dataPoint.SetDoubleValue(float64(message.Counter.GetTotal()))
		dataPoint.SetTimestamp(pcommon.Timestamp(envelope.GetTimestamp()))
		dataPoint.SetStartTimestamp(pcommon.NewTimestampFromTime(startTime))
		copyEnvelopeAttributes(dataPoint.Attributes(), envelope)
	case *loggregator_v2.Envelope_Gauge:
		for name, value := range message.Gauge.GetMetrics() {
			metric := metricSlice.AppendEmpty()
			metric.SetName(namePrefix + name)
			dataPoint := metric.SetEmptyGauge().DataPoints().AppendEmpty()
			dataPoint.SetDoubleValue(value.Value)
			dataPoint.SetTimestamp(pcommon.Timestamp(envelope.GetTimestamp()))
			dataPoint.SetStartTimestamp(pcommon.NewTimestampFromTime(startTime))
			copyEnvelopeAttributes(dataPoint.Attributes(), envelope)
		}
	}
}

func copyEnvelopeAttributes(attributes pcommon.Map, envelope *loggregator_v2.Envelope) {
	for key, value := range envelope.Tags {
		attributes.PutStr(attributeNamePrefix+key, value)
	}

	if envelope.SourceId != "" {
		attributes.PutStr(attributeNamePrefix+"source_id", envelope.SourceId)
	}

	if envelope.InstanceId != "" {
		attributes.PutStr(attributeNamePrefix+"instance_id", envelope.InstanceId)
	}
}
