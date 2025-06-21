// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cloudtraillogs // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/unmarshaler/cloudtraillogs"

import (
	"fmt"
	"io"
	"time"

	gojson "github.com/goccy/go-json"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	conventions "go.opentelemetry.io/otel/semconv/v1.27.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/unmarshaler"
)

type CloudTrailLogsUnmarshaler struct {
	buildInfo component.BuildInfo
}

var _ unmarshaler.AWSUnmarshaler = (*CloudTrailLogsUnmarshaler)(nil)

// CloudTrailRecord represents a CloudTrail log record
// There is no builtin CloudTrailRecord we can leverage like in S3
// So we build our own
type CloudTrailRecord struct {
	EventVersion                 string         `json:"eventVersion"`
	EventTime                    string         `json:"eventTime"`
	EventSource                  string         `json:"eventSource"`
	EventName                    string         `json:"eventName"`
	AwsRegion                    string         `json:"awsRegion"`
	SourceIPAddress              string         `json:"sourceIPAddress"`
	UserAgent                    string         `json:"userAgent"`
	RequestID                    string         `json:"requestID"`
	EventID                      string         `json:"eventID"`
	EventType                    string         `json:"eventType"`
	EventCategory                string         `json:"eventCategory"`
	RecipientAccountID           string         `json:"recipientAccountId"`
	UserIdentity                 map[string]any `json:"userIdentity"`
	ResponseElements             map[string]any `json:"responseElements"`
	RequestParameters            map[string]any `json:"requestParameters"`
	Resources                    []any          `json:"resources"`
	ReadOnly                     bool           `json:"readOnly"`
	ManagementEvent              bool           `json:"managementEvent"`
	TLSDetails                   map[string]any `json:"tlsDetails"`
	SessionCredentialFromConsole string         `json:"sessionCredentialFromConsole"`
}

type CloudTrailLogs struct {
	Records []CloudTrailRecord `json:"Records"`
}

func NewCloudTrailLogsUnmarshaler(buildInfo component.BuildInfo) *CloudTrailLogsUnmarshaler {
	return &CloudTrailLogsUnmarshaler{
		buildInfo: buildInfo,
	}
}

func (u *CloudTrailLogsUnmarshaler) UnmarshalAWSLogs(reader io.Reader) (plog.Logs, error) {
	decompressedBuf, err := io.ReadAll(reader)
	if err != nil {
		return plog.Logs{}, fmt.Errorf("failed to read CloudTrail logs: %w", err)
	}

	var cloudTrailLogs CloudTrailLogs
	if err := gojson.Unmarshal(decompressedBuf, &cloudTrailLogs); err != nil {
		return plog.Logs{}, fmt.Errorf("failed to unmarshal CloudTrail logs: %w", err)
	}

	return u.processRecords(cloudTrailLogs.Records)
}

func (u *CloudTrailLogsUnmarshaler) processRecords(records []CloudTrailRecord) (plog.Logs, error) {
	logs := plog.NewLogs()

	if len(records) == 0 {
		return logs, nil
	}

	// Create a single resource logs entry for all records
	resourceLogs := logs.ResourceLogs().AppendEmpty()
	scopeLogs := resourceLogs.ScopeLogs().AppendEmpty()
	scopeLogs.Scope().SetName(metadata.ScopeName)
	scopeLogs.Scope().SetVersion(u.buildInfo.Version)

	// Set resource attributes based on the first record
	// (all records have the same account ID and region)
	u.setResourceAttributes(resourceLogs.Resource().Attributes(), records[0])

	for _, record := range records {
		logRecord := scopeLogs.LogRecords().AppendEmpty()
		if err := u.setLogRecord(logRecord, record); err != nil {
			return plog.Logs{}, err
		}
	}

	return logs, nil
}

func (u *CloudTrailLogsUnmarshaler) setResourceAttributes(attrs pcommon.Map, record CloudTrailRecord) {
	attrs.PutStr(string(conventions.CloudProviderKey), conventions.CloudProviderAWS.Value.AsString())
	attrs.PutStr(string(conventions.CloudRegionKey), record.AwsRegion)
	attrs.PutStr(string(conventions.CloudAccountIDKey), record.RecipientAccountID)
}

func (u *CloudTrailLogsUnmarshaler) setLogRecord(logRecord plog.LogRecord, record CloudTrailRecord) error {
	t, err := time.Parse(time.RFC3339, record.EventTime)
	if err != nil {
		return fmt.Errorf("failed to parse timestamp of log: %w", err)
	}
	logRecord.SetTimestamp(pcommon.NewTimestampFromTime(t))
	u.setLogAttributes(logRecord.Attributes(), record)
	return nil
}

func (u *CloudTrailLogsUnmarshaler) setLogAttributes(attrs pcommon.Map, record CloudTrailRecord) {
	attrs.PutStr("aws.cloudtrail.event_id", record.EventID)
	attrs.PutStr(string(conventions.RPCMethodKey), record.EventName)
	attrs.PutStr(string(conventions.RPCSystemKey), record.EventType)
	attrs.PutStr(string(conventions.RPCServiceKey), record.EventSource)
	attrs.PutStr(string(conventions.AWSRequestIDKey), record.RequestID)
	attrs.PutStr("aws.event.category", record.EventCategory)
	attrs.PutBool("aws.event.read_only", record.ReadOnly)
	attrs.PutStr("net.peer.ip", record.SourceIPAddress)
	attrs.PutStr(string(conventions.UserAgentOriginalKey), record.UserAgent)

	if record.SessionCredentialFromConsole == "true" {
		attrs.PutBool("aws.session.console", true)
	}

	if record.UserIdentity != nil {
		if principalID, ok := record.UserIdentity["principalId"].(string); ok {
			attrs.PutStr("principal.id", principalID)
		}
		if userName, ok := record.UserIdentity["userName"].(string); ok {
			attrs.PutStr("principal.name", userName)
		}
		if arn, ok := record.UserIdentity["arn"].(string); ok {
			attrs.PutStr("principal.arn", arn)
		}
	}

	if record.TLSDetails != nil {
		if tlsVersion, ok := record.TLSDetails["tlsVersion"].(string); ok {
			// Extract only the version number from TLSv1.2 format
			version := extractTLSVersion(tlsVersion)
			attrs.PutStr(string(conventions.TLSProtocolVersionKey), version)
		}
		if cipherSuite, ok := record.TLSDetails["cipherSuite"].(string); ok {
			attrs.PutStr(string(conventions.TLSCipherKey), cipherSuite)
		}
		if hostHeader, ok := record.TLSDetails["clientProvidedHostHeader"].(string); ok {
			attrs.PutStr(string(conventions.ServerAddressKey), hostHeader)
		}
	}

	// Add RequestParameters as a map directly
	if record.RequestParameters != nil {
		requestParamsMap := attrs.PutEmptyMap("aws.request.parameters")
		_ = requestParamsMap.FromRaw(record.RequestParameters)
	}

	// Add ResponseElements as a map directly
	if record.ResponseElements != nil {
		responseElementsMap := attrs.PutEmptyMap("aws.response.elements")
		_ = responseElementsMap.FromRaw(record.ResponseElements)
	}
}

// extract the version number from a TLS version string (e.g. "TLSv1.2" becomes "1.2")
func extractTLSVersion(tlsVersion string) string {
	if len(tlsVersion) > 4 && tlsVersion[:4] == "TLSv" {
		return tlsVersion[4:]
	}
	return tlsVersion
}
