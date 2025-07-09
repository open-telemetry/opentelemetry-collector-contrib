// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elbaccesslogs

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	conventions "go.opentelemetry.io/otel/semconv/v1.27.0"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/unmarshaler"
)

type elbAccessLogUnmarshaler struct {
	buildInfo component.BuildInfo
	logger    *zap.Logger
}

func NewELBAccessLogUnmarshaler(buildInfo component.BuildInfo, logger *zap.Logger) unmarshaler.AWSUnmarshaler {
	return &elbAccessLogUnmarshaler{
		buildInfo: buildInfo,
		logger:    logger,
	}
}

// UnmarshalAWSLogs processes a file containing ELB access logs.
func (f *elbAccessLogUnmarshaler) UnmarshalAWSLogs(reader io.Reader) (plog.Logs, error) {
	scanner := bufio.NewScanner(reader)

	// Initialize scopeLogsByResource
	scopeLogsByResource := map[string]plog.ScopeLogs{}

	var line string
	var fields []string

	// Read first line to determine format
	if !scanner.Scan() {
		return plog.Logs{}, fmt.Errorf("no log lines found")
	}
	line = scanner.Text()

	fields, err := extractFields(line)
	if err != nil {
		return plog.Logs{}, fmt.Errorf("failed to parse log line: %w", err)
	}
	if len(fields) == 0 {
		return plog.Logs{}, fmt.Errorf("log line has no fields: %s", line)
	}

	// Check for control message
	if fields[0] == EnableControlMessage {
		f.logger.Info(fmt.Sprintf("Control message received: %s", line))
		return plog.NewLogs(), nil
	}

	// Determine syntax
	syntax, err := findLogSyntaxByField(fields[0])
	if err != nil {
		return plog.Logs{}, fmt.Errorf("unable to determine log syntax: %w", err)
	}

	for {
		// Process lines based on determined syntax
		switch syntax {
		case albAccessLogs:
			record, err := convertTextToAlbAccessLogRecord(fields)
			if err != nil {
				return plog.Logs{}, fmt.Errorf("unable to convert log line to ALB record: %w", err)
			}
			f.addToAlbAccessLogs(scopeLogsByResource, record)
		case nlbAccessLogs:
			record, err := convertTextToNlbAccessLogRecord(fields)
			if err != nil {
				return plog.Logs{}, fmt.Errorf("unable to convert log line to NLB record: %w", err)
			}
			f.addToNlbAccessLogs(scopeLogsByResource, record)
		case clbAccessLogs:
			record, err := convertTextToClbAccessLogRecord(fields)
			if err != nil {
				return plog.Logs{}, fmt.Errorf("unable to convert log line to NLB record: %w", err)
			}
			f.addToClbAccessLogs(scopeLogsByResource, record)
		default:
			return plog.Logs{}, fmt.Errorf("unsupported log syntax: %s", syntax)
		}

		// Refill with next line until we reach the scanner end
		if !scanner.Scan() {
			break
		}

		line = scanner.Text()
		fields, err = extractFields(line)
		if len(fields) == 0 {
			return plog.Logs{}, fmt.Errorf("log line has no fields: %s", line)
		}
	}

	// Handle potential scanner errors
	if err := scanner.Err(); err != nil {
		return plog.Logs{}, fmt.Errorf("error scanning log lines: %w", err)
	}

	return f.createLogs(scopeLogsByResource), nil
}

// createLogs based on the scopeLogsByResource map
func (f *elbAccessLogUnmarshaler) createLogs(scopeLogsByResource map[string]plog.ScopeLogs) plog.Logs {
	logs := plog.NewLogs()

	for resourceId, scopeLogs := range scopeLogsByResource {
		rl := logs.ResourceLogs().AppendEmpty()
		attr := rl.Resource().Attributes()
		attr.PutStr(string(conventions.CloudProviderKey), conventions.CloudProviderAWS.Value.AsString())
		attr.PutStr(string(conventions.CloudResourceIDKey), resourceId)

		scopeLogs.MoveTo(rl.ScopeLogs().AppendEmpty())
	}

	return logs
}

// addToClbAccessLogs adds clb record to provided logs based
// on the extracted logs of each resource
func (f *elbAccessLogUnmarshaler) addToClbAccessLogs(scopeLogsByResource map[string]plog.ScopeLogs, clbRecord ClbAccessLogRecord) {
	// Convert timestamp first; if invalid, skip log creation
	epochNanoseconds, err := convertToUnixEpoch(clbRecord.Time)
	if err != nil {
		f.logger.Debug("Timestamp cannot be converted to unix epoch nanoseconds", zap.Error(err))
		return
	}

	// Create record log
	recordLog := plog.NewLogRecord()
	// Populate record attributes
	recordLog.Attributes().PutStr(string(conventions.ClientAddressKey), clbRecord.ClientIp)
	recordLog.Attributes().PutStr(string(conventions.HTTPRequestMethodKey), clbRecord.RequestMethod)
	recordLog.Attributes().PutStr(string(conventions.URLFullKey), clbRecord.RequestURI)
	recordLog.Attributes().PutStr(string(conventions.NetworkProtocolNameKey), clbRecord.ProtocolName)
	recordLog.Attributes().PutStr(string(conventions.NetworkProtocolVersionKey), clbRecord.ProtocolVersion)
	recordLog.Attributes().PutInt(string(conventions.ClientPortKey), clbRecord.ClientPort)
	recordLog.Attributes().PutInt(string(conventions.HTTPRequestSizeKey), clbRecord.ReceivedBytes)
	recordLog.Attributes().PutInt(string(conventions.HTTPResponseSizeKey), clbRecord.SentBytes)
	if clbRecord.SSLProtocol != unknownField {
		recordLog.Attributes().PutStr(string(conventions.TLSProtocolVersionKey), strings.ToLower(clbRecord.SSLProtocol))
	}
	if clbRecord.SSLCipher != unknownField {
		recordLog.Attributes().PutStr(string(conventions.TLSCipherKey), clbRecord.SSLCipher)
	}
	if clbRecord.ELBStatusCode != 0 {
		recordLog.Attributes().PutInt(AttributeELBStatusCode, clbRecord.ELBStatusCode)
	}
	if clbRecord.BackendStatusCode != 0 {
		recordLog.Attributes().PutInt(AttributeELBBackendStatusCode, clbRecord.BackendStatusCode)
	}
	// Set timestamp
	recordLog.SetTimestamp(pcommon.Timestamp(epochNanoseconds))

	// Get scope logs
	scopeLogs := f.getScopeLogs(clbRecord.ELB, scopeLogsByResource)

	// move recordLog to scope
	rScope := scopeLogs.LogRecords().AppendEmpty()
	recordLog.MoveTo(rScope)
}

// addToAlbAccessLogs adds alb record to provided logs based
// on the extracted logs of each resource
func (f *elbAccessLogUnmarshaler) addToAlbAccessLogs(scopeLogsByResource map[string]plog.ScopeLogs, albRecord AlbAccessLogRecord) {
	// Convert timestamp first; if invalid, skip log creation
	epochNanoseconds, err := convertToUnixEpoch(albRecord.Time)
	if err != nil {
		f.logger.Debug("Timestamp cannot be converted to unix epoch nanoseconds", zap.Error(err))
		return
	}

	// Create record log
	recordLog := plog.NewLogRecord()
	// Populate record attributes
	recordLog.Attributes().PutStr(string(conventions.NetworkProtocolNameKey), albRecord.Type)
	recordLog.Attributes().PutStr(string(conventions.NetworkProtocolVersionKey), albRecord.ProtocolVersion)
	recordLog.Attributes().PutStr(string(conventions.ClientAddressKey), albRecord.ClientIp)
	recordLog.Attributes().PutStr(string(conventions.HTTPRequestMethodKey), albRecord.RequestMethod)
	recordLog.Attributes().PutStr(string(conventions.URLFullKey), albRecord.RequestURI)
	recordLog.Attributes().PutInt(string(conventions.ClientPortKey), albRecord.ClientPort)
	recordLog.Attributes().PutInt(string(conventions.HTTPRequestSizeKey), albRecord.ReceivedBytes)
	recordLog.Attributes().PutInt(string(conventions.HTTPResponseSizeKey), albRecord.SentBytes)
	recordLog.Attributes().PutInt(AttributeELBStatusCode, albRecord.ELBStatusCode)
	if albRecord.SSLProtocol != unknownField {
		recordLog.Attributes().PutStr(string(conventions.TLSProtocolVersionKey), strings.ToLower(albRecord.SSLProtocol))
	}
	if albRecord.SSLCipher != unknownField {
		recordLog.Attributes().PutStr(string(conventions.TLSCipherKey), albRecord.SSLCipher)
	}

	// Set timestamp
	recordLog.SetTimestamp(pcommon.Timestamp(epochNanoseconds))

	// Get scope logs
	scopeLogs := f.getScopeLogs(albRecord.ELB, scopeLogsByResource)

	// move recordLog to scope
	rScope := scopeLogs.LogRecords().AppendEmpty()
	recordLog.MoveTo(rScope)
}

// addToNlbAccessLogs adds nlb record to provided logs based
// on the extracted logs of each resource
func (f *elbAccessLogUnmarshaler) addToNlbAccessLogs(scopeLogsByResource map[string]plog.ScopeLogs, nlbRecord NlbAccessLogRecord) {
	// Convert timestamp first; if invalid, skip log creation
	epochNanoseconds, err := convertToUnixEpoch(nlbRecord.Time)
	if err != nil {
		f.logger.Debug("Timestamp cannot be converted to unix epoch nanoseconds", zap.Error(err))
		return
	}

	// Create record log
	recordLog := plog.NewLogRecord()
	// Populate record attributes
	recordLog.Attributes().PutStr(string(conventions.NetworkProtocolNameKey), nlbRecord.Type)
	recordLog.Attributes().PutStr(string(conventions.NetworkProtocolVersionKey), nlbRecord.Version)
	recordLog.Attributes().PutStr(string(conventions.ClientAddressKey), nlbRecord.ClientIp)
	recordLog.Attributes().PutInt(string(conventions.ClientPortKey), nlbRecord.ClientPort)
	recordLog.Attributes().PutInt(string(conventions.HTTPRequestSizeKey), nlbRecord.ReceivedBytes)
	recordLog.Attributes().PutInt(string(conventions.HTTPResponseSizeKey), nlbRecord.SentBytes)
	recordLog.Attributes().PutStr(AttributeTlsListenerResourceID, nlbRecord.Listener)
	recordLog.Attributes().PutStr(string(conventions.TLSProtocolVersionKey), nlbRecord.TLSProtocolVersion)
	recordLog.Attributes().PutStr(string(conventions.TLSCipherKey), nlbRecord.TLSCipher)

	// Set timestamp
	recordLog.SetTimestamp(pcommon.Timestamp(epochNanoseconds))

	// Get scope logs
	scopeLogs := f.getScopeLogs(nlbRecord.ELB, scopeLogsByResource)

	// move recordLog to scope
	rScope := scopeLogs.LogRecords().AppendEmpty()
	recordLog.MoveTo(rScope)
}

// getScopeLogs for the given key. If it does not exist yet,
// create new scope logs, and add the key to the logs map.
func (f *elbAccessLogUnmarshaler) getScopeLogs(key string, logs map[string]plog.ScopeLogs) plog.ScopeLogs {
	scopeLogs, ok := logs[key]
	if !ok {
		scopeLogs = plog.NewScopeLogs()
		scopeLogs.Scope().SetName(metadata.ScopeName)
		scopeLogs.Scope().SetVersion(f.buildInfo.Version)
		logs[key] = scopeLogs
	}
	return scopeLogs
}
