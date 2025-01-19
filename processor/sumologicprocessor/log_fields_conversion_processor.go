// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sumologicprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/sumologicprocessor"

import (
	"encoding/hex"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

const (
	SeverityNumberAttributeName = "loglevel"
	SeverityTextAttributeName   = "severitytext"
	SpanIDAttributeName         = "spanid"
	TraceIDAttributeName        = "traceid"
)

type logFieldAttribute struct {
	Enabled bool   `mapstructure:"enabled"`
	Name    string `mapstructure:"name"`
}

type logFieldAttributesConfig struct {
	SeverityNumberAttribute *logFieldAttribute `mapstructure:"severity_number"`
	SeverityTextAttribute   *logFieldAttribute `mapstructure:"severity_text"`
	SpanIDAttribute         *logFieldAttribute `mapstructure:"span_id"`
	TraceIDAttribute        *logFieldAttribute `mapstructure:"trace_id"`
}

// spanIDToHexOrEmptyString returns a hex string from SpanID.
// An empty string is returned, if SpanID is empty.
func spanIDToHexOrEmptyString(id pcommon.SpanID) string {
	if id.IsEmpty() {
		return ""
	}
	return hex.EncodeToString(id[:])
}

// traceIDToHexOrEmptyString returns a hex string from TraceID.
// An empty string is returned, if TraceID is empty.
func traceIDToHexOrEmptyString(id pcommon.TraceID) string {
	if id.IsEmpty() {
		return ""
	}
	return hex.EncodeToString(id[:])
}

var severityNumberToLevel = map[string]string{
	plog.SeverityNumberUnspecified.String(): "UNSPECIFIED",
	plog.SeverityNumberTrace.String():       "TRACE",
	plog.SeverityNumberTrace2.String():      "TRACE2",
	plog.SeverityNumberTrace3.String():      "TRACE3",
	plog.SeverityNumberTrace4.String():      "TRACE4",
	plog.SeverityNumberDebug.String():       "DEBUG",
	plog.SeverityNumberDebug2.String():      "DEBUG2",
	plog.SeverityNumberDebug3.String():      "DEBUG3",
	plog.SeverityNumberDebug4.String():      "DEBUG4",
	plog.SeverityNumberInfo.String():        "INFO",
	plog.SeverityNumberInfo2.String():       "INFO2",
	plog.SeverityNumberInfo3.String():       "INFO3",
	plog.SeverityNumberInfo4.String():       "INFO4",
	plog.SeverityNumberWarn.String():        "WARN",
	plog.SeverityNumberWarn2.String():       "WARN2",
	plog.SeverityNumberWarn3.String():       "WARN3",
	plog.SeverityNumberWarn4.String():       "WARN4",
	plog.SeverityNumberError.String():       "ERROR",
	plog.SeverityNumberError2.String():      "ERROR2",
	plog.SeverityNumberError3.String():      "ERROR3",
	plog.SeverityNumberError4.String():      "ERROR4",
	plog.SeverityNumberFatal.String():       "FATAL",
	plog.SeverityNumberFatal2.String():      "FATAL2",
	plog.SeverityNumberFatal3.String():      "FATAL3",
	plog.SeverityNumberFatal4.String():      "FATAL4",
}

// logFieldsConversionProcessor converts specific log entries to attributes which leads to presenting them as fields
// in the backend
type logFieldsConversionProcessor struct {
	LogFieldsAttributes *logFieldAttributesConfig
}

func newLogFieldConversionProcessor(logFieldsAttributes *logFieldAttributesConfig) *logFieldsConversionProcessor {
	return &logFieldsConversionProcessor{
		logFieldsAttributes,
	}
}

func (proc *logFieldsConversionProcessor) addAttributes(log plog.LogRecord) {
	if log.SeverityNumber() != plog.SeverityNumberUnspecified {
		if _, found := log.Attributes().Get(SeverityNumberAttributeName); !found &&
			proc.LogFieldsAttributes.SeverityNumberAttribute.Enabled {
			level := severityNumberToLevel[log.SeverityNumber().String()]
			log.Attributes().PutStr(proc.LogFieldsAttributes.SeverityNumberAttribute.Name, level)
		}
	}
	if _, found := log.Attributes().Get(SeverityTextAttributeName); !found &&
		proc.LogFieldsAttributes.SeverityTextAttribute.Enabled {
		log.Attributes().PutStr(proc.LogFieldsAttributes.SeverityTextAttribute.Name, log.SeverityText())
	}
	if _, found := log.Attributes().Get(SpanIDAttributeName); !found &&
		proc.LogFieldsAttributes.SpanIDAttribute.Enabled {
		log.Attributes().PutStr(proc.LogFieldsAttributes.SpanIDAttribute.Name, spanIDToHexOrEmptyString(log.SpanID()))
	}
	if _, found := log.Attributes().Get(TraceIDAttributeName); !found &&
		proc.LogFieldsAttributes.TraceIDAttribute.Enabled {
		log.Attributes().PutStr(proc.LogFieldsAttributes.TraceIDAttribute.Name, traceIDToHexOrEmptyString(log.TraceID()))
	}
}

func (proc *logFieldsConversionProcessor) processLogs(logs plog.Logs) error {
	if !proc.isEnabled() {
		return nil
	}

	rls := logs.ResourceLogs()
	for i := 0; i < rls.Len(); i++ {
		ills := rls.At(i).ScopeLogs()

		for j := 0; j < ills.Len(); j++ {
			logs := ills.At(j).LogRecords()
			for k := 0; k < logs.Len(); k++ {
				proc.addAttributes(logs.At(k))
			}
		}
	}
	return nil
}

func (proc *logFieldsConversionProcessor) processMetrics(_ pmetric.Metrics) error {
	// No-op. Metrics should not be translated.
	return nil
}

func (proc *logFieldsConversionProcessor) processTraces(_ ptrace.Traces) error {
	// No-op. Traces should not be translated.
	return nil
}

func (proc *logFieldsConversionProcessor) isEnabled() bool {
	return proc.LogFieldsAttributes.SeverityNumberAttribute.Enabled ||
		proc.LogFieldsAttributes.SeverityTextAttribute.Enabled ||
		proc.LogFieldsAttributes.SpanIDAttribute.Enabled ||
		proc.LogFieldsAttributes.TraceIDAttribute.Enabled
}

func (*logFieldsConversionProcessor) ConfigPropertyName() string {
	return "field_attributes"
}
