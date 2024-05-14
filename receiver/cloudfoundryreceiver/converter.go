// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cloudfoundryreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/cloudfoundryreceiver"

import (
	"strings"
	"time"

	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/otel/trace"
)

const (
	attributeNamePrefix = "org.cloudfoundry."
)

func convertEnvelopeToMetrics(envelope *loggregator_v2.Envelope, metricSlice pmetric.MetricSlice, startTime time.Time) {
	namePrefix := envelope.Tags["origin"] + "."

	switch message := envelope.Message.(type) {
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

func convertEnvelopeToLogs(envelope *loggregator_v2.Envelope, logSlice plog.LogRecordSlice, startTime time.Time) {
	switch envelope.Message.(type) {
	case *loggregator_v2.Envelope_Log:
		log := logSlice.AppendEmpty()
		log.SetTimestamp(pcommon.Timestamp(envelope.GetTimestamp()))
		log.SetObservedTimestamp(pcommon.NewTimestampFromTime(startTime))
		log.Body().SetStr(string(envelope.GetLog().GetPayload()))
		switch envelope.GetLog().GetType() {
		case loggregator_v2.Log_OUT:
			log.SetSeverityText(plog.SeverityNumberInfo.String())
			log.SetSeverityNumber(plog.SeverityNumberInfo)
		case loggregator_v2.Log_ERR:
			log.SetSeverityText(plog.SeverityNumberError.String())
			log.SetSeverityNumber(plog.SeverityNumberError)
		}
		copyEnvelopeAttributes(log.Attributes(), envelope)
		_ = parseLogTracingFields(log)
	default:
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

func parseLogTracingFields(log plog.LogRecord) error {
	if value, found := log.Attributes().Get("org.cloudfoundry.source_type"); !found || value.AsString() != "RTR" {
		return nil
	}
	s := log.Body().AsString()
	quoted := false
	a := strings.FieldsFunc(s, func(r rune) bool {
		if r == '"' {
			quoted = !quoted
		}
		return !quoted && r == ' '
	})

	traceIDStr := strings.Split(a[21], ":")[1]
	traceIDStr = strings.Trim(traceIDStr, "\"")

	spanIDStr := strings.Split(a[22], ":")[1]
	spanIDStr = strings.Trim(spanIDStr, "\"")

	traceID, err := trace.TraceIDFromHex(traceIDStr)
	if err != nil {
		return err
	}

	spanID, err := trace.SpanIDFromHex(spanIDStr)
	if err != nil {
		return err
	}

	log.SetTraceID([16]byte(traceID))
	log.SetSpanID([8]byte(spanID))

	return nil
}
