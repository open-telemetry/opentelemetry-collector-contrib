// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cloudfoundryreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/cloudfoundryreceiver"

import (
	"fmt"
	"slices"
	"time"

	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

const (
	attributeNamePrefix = "org.cloudfoundry."
)

var ResourceAttributesKeys = []string{
	"index",
	"ip",
	"deployment",
	"id",
	"job",
	"product",
	"instance_group",
	"instance_id",
	"origin",
	"system_domain",
	"source_id",
	"source_type",
	"process_type",
	"process_id",
	"process_instance_id",
}

var allowResourceAttributes = featuregate.GlobalRegistry().MustRegister(
	"cloudfoundry.resourceAttributes.allow",
	featuregate.StageAlpha,
	featuregate.WithRegisterDescription("When enabled, envelope tags are copied to the metrics as resource attributes instead of datapoint attributes"),
	featuregate.WithRegisterFromVersion("v0.109.0"),
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
		if allowResourceAttributes.IsEnabled() {
			attrs := getEnvelopeDatapointAttributes(envelope)
			attrs.CopyTo(dataPoint.Attributes())
		} else {
			copyEnvelopeAttributes(dataPoint.Attributes(), envelope)
		}
	case *loggregator_v2.Envelope_Gauge:
		for name, value := range message.Gauge.GetMetrics() {
			metric := metricSlice.AppendEmpty()
			metric.SetName(namePrefix + name)
			dataPoint := metric.SetEmptyGauge().DataPoints().AppendEmpty()
			dataPoint.SetDoubleValue(value.Value)
			dataPoint.SetTimestamp(pcommon.Timestamp(envelope.GetTimestamp()))
			dataPoint.SetStartTimestamp(pcommon.NewTimestampFromTime(startTime))
			if allowResourceAttributes.IsEnabled() {
				attrs := getEnvelopeDatapointAttributes(envelope)
				attrs.CopyTo(dataPoint.Attributes())
			} else {
				copyEnvelopeAttributes(dataPoint.Attributes(), envelope)
			}
		}
	}
}

func convertEnvelopeToLogs(envelope *loggregator_v2.Envelope, logSlice plog.LogRecordSlice, startTime time.Time) error {
	log := logSlice.AppendEmpty()
	log.SetTimestamp(pcommon.Timestamp(envelope.GetTimestamp()))
	log.SetObservedTimestamp(pcommon.NewTimestampFromTime(startTime))
	logLine := string(envelope.GetLog().GetPayload())
	log.Body().SetStr(logLine)
	//exhaustive:enforce
	switch envelope.GetLog().GetType() {
	case loggregator_v2.Log_OUT:
		log.SetSeverityText(plog.SeverityNumberInfo.String())
		log.SetSeverityNumber(plog.SeverityNumberInfo)
	case loggregator_v2.Log_ERR:
		log.SetSeverityText(plog.SeverityNumberError.String())
		log.SetSeverityNumber(plog.SeverityNumberError)
	default:
		return fmt.Errorf("unsupported envelope log type: %s", envelope.GetLog().GetType())
	}
	if allowResourceAttributes.IsEnabled() {
		attrs := getEnvelopeDatapointAttributes(envelope)
		attrs.CopyTo(log.Attributes())
	} else {
		copyEnvelopeAttributes(log.Attributes(), envelope)
	}
	return nil
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

func getEnvelopeDatapointAttributes(envelope *loggregator_v2.Envelope) pcommon.Map {
	attrs := pcommon.NewMap()
	for key, value := range envelope.Tags {
		if !slices.Contains(ResourceAttributesKeys, key) {
			attrs.PutStr(attributeNamePrefix+key, value)
		}
	}
	return attrs
}

func getEnvelopeResourceAttributes(envelope *loggregator_v2.Envelope) pcommon.Map {
	attrs := pcommon.NewMap()
	for key, value := range envelope.Tags {
		if slices.Contains(ResourceAttributesKeys, key) {
			attrs.PutStr(attributeNamePrefix+key, value)
		}
	}
	if envelope.SourceId != "" {
		attrs.PutStr(attributeNamePrefix+"source_id", envelope.SourceId)
	}
	if envelope.InstanceId != "" {
		attrs.PutStr(attributeNamePrefix+"instance_id", envelope.InstanceId)
	}
	return attrs
}
