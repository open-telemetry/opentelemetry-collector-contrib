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
}

func (packer *logPacker) LogRecordToEnvelope(logRecord plog.LogRecord, resource pcommon.Resource, instrumentationScope pcommon.InstrumentationScope) *contracts.Envelope {
	envelope := contracts.NewEnvelope()
	envelope.Tags = make(map[string]string)
	envelope.Time = toTime(timestampFromLogRecord(logRecord)).Format(time.RFC3339Nano)

	data := contracts.NewData()

	messageData := contracts.NewMessageData()
	messageData.Properties = make(map[string]string)

	messageData.SeverityLevel = packer.toAiSeverityLevel(logRecord.SeverityNumber())

	messageData.Message = logRecord.Body().Str()

	envelope.Tags[contracts.OperationId] = traceutil.TraceIDToHexOrEmptyString(logRecord.TraceID())
	envelope.Tags[contracts.OperationParentId] = traceutil.SpanIDToHexOrEmptyString(logRecord.SpanID())

	envelope.Name = messageData.EnvelopeName("")

	data.BaseData = messageData
	data.BaseType = messageData.BaseType()
	envelope.Data = data

	resourceAttributes := resource.Attributes()
	applyResourcesToDataProperties(messageData.Properties, resourceAttributes)
	applyInstrumentationScopeValueToDataProperties(messageData.Properties, instrumentationScope)
	applyCloudTagsToEnvelope(envelope, resourceAttributes)

	setAttributesAsProperties(logRecord.Attributes(), messageData.Properties)

	packer.sanitize(func() []string { return messageData.Sanitize() })
	packer.sanitize(func() []string { return envelope.Sanitize() })
	packer.sanitize(func() []string { return contracts.SanitizeTags(envelope.Tags) })

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

func newLogPacker(logger *zap.Logger) *logPacker {
	packer := &logPacker{
		logger: logger,
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
