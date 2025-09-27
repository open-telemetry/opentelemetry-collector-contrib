// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadoglogreceiver/internal"

import (
	"compress/flate"
	"compress/gzip"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"
	"time"

	jsoniter "github.com/json-iterator/go"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
)

var (
	contentType = http.CanonicalHeaderKey("Content-Type")
	contentEnc  = http.CanonicalHeaderKey("Content-Encoding")
)

func ParseRequest(
	req *http.Request,
	logger *zap.Logger,
	enableDdtagsAttribute bool,
) (*plog.Logs, error) {
	body, err := requestBody(req)
	if err != nil {
		return nil, err
	}

	defer func() {
		if closer, ok := body.(io.Closer); ok {
			if closeErr := closer.Close(); closeErr != nil {
				logger.Warn("failed to close reader", zap.Error(closeErr))
			}
		}
	}()

	if validateErr := validateContentType(req); validateErr != nil {
		return nil, validateErr
	}

	datadogRecords, err := parseDatadogRecords(body, logger)
	if err != nil {
		return nil, err
	}

	return convertToLogs(datadogRecords, enableDdtagsAttribute)
}

func requestBody(req *http.Request) (io.Reader, error) {
	contentEncoding := req.Header.Get(contentEnc)
	switch contentEncoding {
	case "", "snappy":
		return req.Body, nil
	case "gzip":
		gzipReader, err := gzip.NewReader(req.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to create gzip reader: %w", err)
		}
		return gzipReader, nil
	case "deflate":
		return flate.NewReader(req.Body), nil
	default:
		return nil, fmt.Errorf("Content-Encoding %q not supported", contentEncoding)
	}
}

func validateContentType(req *http.Request) error {
	reqContentType := req.Header.Get(contentType)
	if reqContentType != "" {
		if _, _, err := mime.ParseMediaType(reqContentType); err != nil {
			return fmt.Errorf("invalid Content-Type: %w", err)
		}
	}
	return nil
}

func parseDatadogRecords(body io.Reader, logger *zap.Logger) ([]DatadogRecord, error) {
	bodyBytes, err := io.ReadAll(body)
	if err != nil {
		logger.Error("Failed to read request body", zap.Error(err))
		return nil, err
	}

	// Try parsing as array first
	var datadogRecords []DatadogRecord
	json := jsoniter.ConfigCompatibleWithStandardLibrary
	arrayErr := json.Unmarshal(bodyBytes, &datadogRecords)
	if arrayErr == nil {
		return datadogRecords, nil
	}

	// If array parsing fails, try single record
	var singleRecord DatadogRecord
	if err := json.Unmarshal(bodyBytes, &singleRecord); err != nil {
		logger.Error(
			"Failed to decode log request as either array or single object",
			zap.String("body", string(bodyBytes)),
			zap.Error(arrayErr), // Log the array parsing error
			zap.Error(err),      // Log the single record parsing error
		)
		return nil, fmt.Errorf("failed to parse as array: %w; as single record: %w", arrayErr, err)
	}

	return []DatadogRecord{singleRecord}, nil
}

func convertToLogs(datadogRecords []DatadogRecord, enableDdtagsAttribute bool) (*plog.Logs, error) {
	logs := plog.NewLogs()
	if len(datadogRecords) == 0 {
		return &logs, nil
	}

	rls := logs.ResourceLogs()
	rls.EnsureCapacity(len(datadogRecords))

	for i := range datadogRecords {
		if err := addRecordToLogs(rls.AppendEmpty(), datadogRecords[i], enableDdtagsAttribute); err != nil {
			return nil, err
		}
	}

	return &logs, nil
}

func addRecordToLogs(rl plog.ResourceLogs, record DatadogRecord, enableDdtagsAttribute bool) error {
	sl := rl.ScopeLogs()
	sl.EnsureCapacity(1)

	ddTagsMap := ddTagsAsAttributeMap(record.Ddtags)
	if enableDdtagsAttribute {
		ddTagsMap.CopyTo(rl.Resource().Attributes().PutEmptyMap("ddtags"))
	} else {
		ddTagsMap.CopyTo(rl.Resource().Attributes())
	}

	logRecord := sl.AppendEmpty().LogRecords().AppendEmpty()
	logRecord.SetObservedTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	if time.Time(record.Timestamp).IsZero() {
		logRecord.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	} else {
		logRecord.SetTimestamp(pcommon.NewTimestampFromTime(time.Time(record.Timestamp)))
	}

	if err := setLogRecordBody(logRecord, record.Message); err != nil {
		return err
	}

	severityNumber, severityText := toSeverity(record.Status, logRecord.Body().AsString())
	setLogRecordAttributes(logRecord, record, severityNumber, severityText)

	return nil
}

func ddTagsAsAttributeMap(ddtags string) pcommon.Map {
	attributesMap := pcommon.NewMap()
	for _, part := range strings.Split(ddtags, ",") {
		kv := strings.SplitN(part, ":", 2)
		switch len(kv) {
		case 0:
			continue
		case 1:
			attributesMap.PutStr(kv[0], "")
		default:
			attributesMap.PutStr(kv[0], kv[1])
		}
	}
	return attributesMap
}

// datadogStatusToSeverity converts Datadog status to OpenTelemetry severity
// Reference: https://docs.datadoghq.com/logs/log_configuration/processors/?tab=ui#log-status-remapper
// OpenTelemetry severity: https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/logs/data-model.md#severity-fields
func toSeverity(status, message string) (plog.SeverityNumber, string) {
	switch strings.ToLower(status) {
	case "emergency":
		return plog.SeverityNumberFatal, "FATAL"
	case "alert", "critical":
		return plog.SeverityNumberError3, "ERROR"
	// Datadog sets severity to "error" for logs which it not able to classify
	// Run extractSeverity to see if we can classify the log
	case "error":
		severity := extractSeverity([]byte(message))
		return mapOtelSeverity(severity)
	case "warning":
		return plog.SeverityNumberWarn, "WARN"
	case "notice", "info":
		return plog.SeverityNumberInfo, "INFO"
	case "debug", "trace":
		return plog.SeverityNumberDebug, "DEBUG"
	default:
		return plog.SeverityNumberUnspecified, "UNSPECIFIED"
	}
}

func setLogRecordBody(logRecord plog.LogRecord, message Message) error {
	switch {
	case message.Str != "":
		logRecord.Body().SetStr(message.Str)
	case message.Map != nil:
		if err := logRecord.Body().SetEmptyMap().FromRaw(message.Map); err != nil {
			return fmt.Errorf("failed to set map from message: %w", err)
		}
	case message.Arr != nil:
		if err := logRecord.Body().SetEmptySlice().FromRaw(message.Arr); err != nil {
			return fmt.Errorf("failed to set array from message: %w", err)
		}
	}
	return nil
}

func setLogRecordAttributes(
	logRecord plog.LogRecord,
	record DatadogRecord,
	severityNumber plog.SeverityNumber,
	severityText string,
) {
	logRecord.SetSeverityNumber(severityNumber)
	logRecord.SetSeverityText(severityText)

	attrs := logRecord.Attributes()
	if record.Hostname != "" {
		attrs.PutStr("hostname", record.Hostname)
	}
	if record.Service != "" {
		attrs.PutStr("service", record.Service)
	}
	if record.Ddsource != "" {
		attrs.PutStr("datadog.log.source", record.Ddsource)
		attrs.PutStr("ddsource", record.Ddsource)
	}

	if record.Status == "error" {
		attrs.PutStr("status", severityText)
	} else {
		attrs.PutStr("status", record.Status)
	}
}
