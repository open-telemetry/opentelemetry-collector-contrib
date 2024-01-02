// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

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
