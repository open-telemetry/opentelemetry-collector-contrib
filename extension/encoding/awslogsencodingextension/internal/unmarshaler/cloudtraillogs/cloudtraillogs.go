// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cloudtraillogs

import (
	"bytes"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/goccy/go-json"
	"github.com/klauspost/compress/gzip"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	conventions "go.opentelemetry.io/otel/semconv/v1.27.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/metadata"
)

type CloudTrailLogsUnmarshaler struct {
	buildInfo component.BuildInfo
	gzipPool  sync.Pool
}

var _ plog.Unmarshaler = (*CloudTrailLogsUnmarshaler)(nil)

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
	AdditionalEventData          map[string]any `json:"additionalEventData"`
	TLSDetails                   map[string]any `json:"tlsDetails"`
	SessionCredentialFromConsole string         `json:"sessionCredentialFromConsole"`
}

type CloudTrailLogs struct {
	Records []CloudTrailRecord `json:"Records"`
}

func NewCloudTrailLogsUnmarshaler(buildInfo component.BuildInfo) plog.Unmarshaler {
	return &CloudTrailLogsUnmarshaler{
		buildInfo: buildInfo,
		gzipPool:  sync.Pool{},
	}
}

func (u *CloudTrailLogsUnmarshaler) Unmarshal(buf []byte) (plog.Logs, error) {
	return u.UnmarshalLogs(buf)
}

func (u *CloudTrailLogsUnmarshaler) UnmarshalLogs(buf []byte) (plog.Logs, error) {
	gzipReader, ok := u.gzipPool.Get().(*gzip.Reader)
	if !ok {
		var err error
		gzipReader, err = gzip.NewReader(bytes.NewReader(buf))
		if err != nil {
			return plog.Logs{}, fmt.Errorf("failed to create gzip reader: %w", err)
		}
	} else {
		if err := gzipReader.Reset(bytes.NewReader(buf)); err != nil {
			u.gzipPool.Put(gzipReader)
			return plog.Logs{}, fmt.Errorf("failed to reset gzip reader: %w", err)
		}
	}

	defer func() {
		_ = gzipReader.Close()
		u.gzipPool.Put(gzipReader)
	}()

	decompressedBuf, err := io.ReadAll(gzipReader)
	if err != nil {
		return plog.Logs{}, fmt.Errorf("failed to decompress CloudTrail logs: %w", err)
	}

	var cloudTrailLogs CloudTrailLogs
	if err := json.Unmarshal(decompressedBuf, &cloudTrailLogs); err != nil || len(cloudTrailLogs.Records) == 0 {
		return plog.Logs{}, fmt.Errorf("failed to unmarshal CloudTrail logs: %w", err)
	}

	return u.processRecords(cloudTrailLogs.Records), nil
}

func (u *CloudTrailLogsUnmarshaler) processRecords(records []CloudTrailRecord) plog.Logs {
	logs := plog.NewLogs()

	if len(records) == 0 {
		return logs
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
		u.setLogRecord(logRecord, record)
	}

	return logs
}

// checks if the event is related to EC2 instance operations
func isEC2InstanceOperation(eventName string) bool {
	ec2Operations := map[string]bool{
		"StartInstances":     true,
		"StopInstances":      true,
		"TerminateInstances": true,
		"RebootInstances":    true,
		"RunInstances":       true,
	}
	return ec2Operations[eventName]
}

func extractInstanceIDs(requestParams map[string]any) []string {
	var instanceIDs []string

	if instancesSet, ok := requestParams["instancesSet"].(map[string]any); ok {
		if items, ok := instancesSet["items"].([]any); ok {
			for _, item := range items {
				if itemMap, ok := item.(map[string]any); ok {
					if instanceID, ok := itemMap["instanceId"].(string); ok {
						instanceIDs = append(instanceIDs, instanceID)
					}
				}
			}
		}
	}

	return instanceIDs
}

func (u *CloudTrailLogsUnmarshaler) setResourceAttributes(attrs pcommon.Map, record CloudTrailRecord) {
	attrs.PutStr(string(conventions.CloudProviderKey), conventions.CloudProviderAWS.Value.AsString())
	attrs.PutStr(string(conventions.CloudRegionKey), record.AwsRegion)
	attrs.PutStr(string(conventions.CloudAccountIDKey), record.RecipientAccountID)
}

func (u *CloudTrailLogsUnmarshaler) setLogRecord(logRecord plog.LogRecord, record CloudTrailRecord) {
	if t, err := time.Parse(time.RFC3339, record.EventTime); err == nil {
		logRecord.SetTimestamp(pcommon.NewTimestampFromTime(t))
	}
	u.setLogAttributes(logRecord.Attributes(), record)
}

func (u *CloudTrailLogsUnmarshaler) setLogAttributes(attrs pcommon.Map, record CloudTrailRecord) {
	attrs.PutStr("request.event_id", record.EventID)
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
			attrs.PutStr("principal.iam.arn", arn)
		}
	}

	if record.TLSDetails != nil {
		if tlsVersion, ok := record.TLSDetails["tlsVersion"].(string); ok {
			attrs.PutStr(string(conventions.TLSProtocolVersionKey), tlsVersion)
		}
		if cipherSuite, ok := record.TLSDetails["cipherSuite"].(string); ok {
			attrs.PutStr(string(conventions.TLSCipherKey), cipherSuite)
		}
		if hostHeader, ok := record.TLSDetails["clientProvidedHostHeader"].(string); ok {
			attrs.PutStr(string(conventions.ServerAddressKey), hostHeader)
		}
	}

	if record.RequestParameters != nil {
		if userName, ok := record.RequestParameters["userName"].(string); ok {
			attrs.PutStr("aws.target_user.name", userName)
		}
	}

	if record.ResponseElements != nil {
		if user, ok := record.ResponseElements["user"].(map[string]any); ok {
			if arn, ok := user["arn"].(string); ok {
				attrs.PutStr("aws.target_user.arn", arn)
			}
			if userID, ok := user["userId"].(string); ok {
				attrs.PutStr("aws.target_user.id", userID)
			}
			if path, ok := user["path"].(string); ok {
				attrs.PutStr("aws.target_user.path", path)
			}
		}
	}

	if isEC2InstanceOperation(record.EventName) {
		instanceIDs := extractInstanceIDs(record.RequestParameters)
		if len(instanceIDs) > 0 {
			instancesSlice := attrs.PutEmptySlice("aws.request.parameters.instances")
			for _, id := range instanceIDs {
				instancesSlice.AppendEmpty().SetStr(id)
			}
		}

		instanceDetails := extractInstanceDetails(record.ResponseElements)
		if len(instanceDetails) > 0 {
			respInstances := attrs.PutEmptySlice("aws.response.instances")

			for _, details := range instanceDetails {
				kvListItem := respInstances.AppendEmpty()
				kvListValue := kvListItem.SetEmptyMap()
				for k, v := range details {
					kvListValue.PutStr(k, v)
				}
			}
		}
	}
}

func extractInstanceDetails(responseElements map[string]any) []map[string]string {
	var instanceDetails []map[string]string

	if instancesSet, ok := responseElements["instancesSet"].(map[string]any); ok {
		if items, ok := instancesSet["items"].([]any); ok {
			for _, item := range items {
				if itemMap, ok := item.(map[string]any); ok {
					details := make(map[string]string)

					if instanceID, ok := itemMap["instanceId"].(string); ok {
						details["instanceId"] = instanceID
					}

					if cs, ok := itemMap["currentState"].(map[string]any); ok {
						if name, ok := cs["name"].(string); ok {
							details["currentState"] = name
						}
					}

					if ps, ok := itemMap["previousState"].(map[string]any); ok {
						if name, ok := ps["name"].(string); ok {
							details["previousState"] = name
						}
					}

					if len(details) > 0 {
						instanceDetails = append(instanceDetails, details)
					}
				}
			}
		}
	}

	return instanceDetails
}
