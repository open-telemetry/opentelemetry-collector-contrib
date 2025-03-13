// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azuremonitorexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azuremonitorexporter"

import (
	"time"

	"github.com/microsoft/ApplicationInsights-Go/appinsights/contracts"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/traceutil"
)

type logPacker struct {
	logger *zap.Logger
	config *Config
}

func (packer *logPacker) initEnvelope(logRecord plog.LogRecord) (*contracts.Envelope, *contracts.Data) {
	envelope := contracts.NewEnvelope()
	envelope.Tags = make(map[string]string)
	envelope.Time = toTime(timestampFromLogRecord(logRecord)).Format(time.RFC3339Nano)
	return envelope, contracts.NewData()
}

func (packer *logPacker) handleEventData(envelope *contracts.Envelope, data *contracts.Data, logRecord plog.LogRecord) {
	attributes := logRecord.Attributes()
	eventData := contracts.NewEventData()
	if val, ok := attributes.Get(attributeMicrosoftCustomEventName); ok {
		eventData.Name = val.AsString()
	} else if val, ok := attributes.Get(attributeApplicationInsightsEventMarkerAttribute); ok {
		eventData.Name = val.AsString()
	}

	eventData.Properties = make(map[string]string)
	setAttributesAsProperties(attributes, eventData.Properties)

	data.BaseData = eventData
	data.BaseType = eventData.BaseType()
	envelope.Name = eventData.EnvelopeName("")
	envelope.Data = data

	packer.sanitizeAll(envelope, eventData)
}

func (packer *logPacker) handleMessageData(envelope *contracts.Envelope, data *contracts.Data, logRecord plog.LogRecord, resource pcommon.Resource, instrumentationScope pcommon.InstrumentationScope) {
	messageData := contracts.NewMessageData()
	messageData.Properties = make(map[string]string)
	messageData.SeverityLevel = packer.toAiSeverityLevel(logRecord.SeverityNumber())
	messageData.Message = logRecord.Body().AsString()

	envelope.Tags[contracts.OperationId] = traceutil.TraceIDToHexOrEmptyString(logRecord.TraceID())
	envelope.Tags[contracts.OperationParentId] = traceutil.SpanIDToHexOrEmptyString(logRecord.SpanID())

	data.BaseData = messageData
	data.BaseType = messageData.BaseType()
	envelope.Name = messageData.EnvelopeName("")
	envelope.Data = data

	resourceAttributes := resource.Attributes()
	applyResourcesToDataProperties(messageData.Properties, resourceAttributes)
	applyInstrumentationScopeValueToDataProperties(messageData.Properties, instrumentationScope)
	applyCloudTagsToEnvelope(envelope, resourceAttributes)
	applyInternalSdkVersionTagToEnvelope(envelope)

	setAttributesAsProperties(logRecord.Attributes(), messageData.Properties)

	packer.sanitizeAll(envelope, messageData)
}

func (packer *logPacker) sanitizeAll(envelope *contracts.Envelope, data any) {
	if sanitizer, ok := data.(interface{ Sanitize() []string }); ok {
		packer.sanitize(sanitizer.Sanitize)
	}
	packer.sanitize(envelope.Sanitize)
	packer.sanitize(func() []string { return contracts.SanitizeTags(envelope.Tags) })
}

func (packer *logPacker) LogRecordToEnvelope(logRecord plog.LogRecord, resource pcommon.Resource, instrumentationScope pcommon.InstrumentationScope) *contracts.Envelope {
	envelope, data := packer.initEnvelope(logRecord)
	attributes := logRecord.Attributes()

	if packer.config.CustomEventsEnabled && isEventData(attributes) {
		packer.handleEventData(envelope, data, logRecord)
	} else {
		packer.handleMessageData(envelope, data, logRecord, resource, instrumentationScope)
	}

	return envelope
}

func (packer *logPacker) sanitize(sanitizeFunc func() []string) {
	for _, warning := range sanitizeFunc() {
		packer.logger.Warn(warning)
	}
}

func (packer *logPacker) toAiSeverityLevel(sn plog.SeverityNumber) contracts.SeverityLevel {
	switch {
	case sn >= plog.SeverityNumberTrace && sn <= plog.SeverityNumberDebug4:
		return contracts.Verbose
	case sn >= plog.SeverityNumberInfo && sn <= plog.SeverityNumberInfo4:
		return contracts.Information
	case sn >= plog.SeverityNumberWarn && sn <= plog.SeverityNumberWarn4:
		return contracts.Warning
	case sn >= plog.SeverityNumberError && sn <= plog.SeverityNumberError4:
		return contracts.Error
	case sn >= plog.SeverityNumberFatal && sn <= plog.SeverityNumberFatal4:
		return contracts.Critical
	default:
		return contracts.Information
	}
}

func newLogPacker(logger *zap.Logger, config *Config) *logPacker {
	packer := &logPacker{
		logger: logger,
		config: config,
	}
	return packer
}

func timestampFromLogRecord(lr plog.LogRecord) pcommon.Timestamp {
	if lr.Timestamp() != 0 {
		return lr.Timestamp()
	}

	if lr.ObservedTimestamp() != 0 {
		return lr.ObservedTimestamp()
	}

	return pcommon.NewTimestampFromTime(timeNow())
}

func hasOneOfKeys(attrMap pcommon.Map, keys ...string) bool {
	for _, key := range keys {
		_, exists := attrMap.Get(key)
		if exists {
			return true
		}
	}
	return false
}

func isEventData(attrMap pcommon.Map) bool {
	return hasOneOfKeys(attrMap, attributeMicrosoftCustomEventName, attributeApplicationInsightsEventMarkerAttribute)
}
