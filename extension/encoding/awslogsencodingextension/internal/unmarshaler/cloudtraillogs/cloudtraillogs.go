// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cloudtraillogs

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

// CloudTrailLogsUnmarshaler is an unmarshaler for CloudTrail logs.
type CloudTrailLogsUnmarshaler struct {
	buildInfo component.BuildInfo
}

var _ plog.Unmarshaler = (*CloudTrailLogsUnmarshaler)(nil)

// CloudTrailRecord represents a CloudTrail log record
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

const (
	cloudProvider = "aws"
	scopeName     = "cloudtrail.processor"
)

func NewCloudTrailLogsUnmarshaler(buildInfo component.BuildInfo) plog.Unmarshaler {
	return &CloudTrailLogsUnmarshaler{
		buildInfo: buildInfo,
	}
}

func (u *CloudTrailLogsUnmarshaler) Unmarshal(buf []byte) (plog.Logs, error) {
	return u.UnmarshalLogs(buf)
}

func (u *CloudTrailLogsUnmarshaler) UnmarshalLogs(buf []byte) (plog.Logs, error) {
	var record CloudTrailRecord
	if err := json.Unmarshal(buf, &record); err != nil {
		return plog.Logs{}, fmt.Errorf("failed to unmarshal CloudTrail logs: %w", err)
	}

	logs, resourceLogs, scopeLogs := u.createLogs()

	u.setResourceAttributes(resourceLogs.Resource().Attributes(), record)

	logRecord := scopeLogs.LogRecords().AppendEmpty()
	u.setLogRecord(logRecord, record)

	return logs, nil
}

func (u *CloudTrailLogsUnmarshaler) createLogs() (plog.Logs, plog.ResourceLogs, plog.ScopeLogs) {
	logs := plog.NewLogs()
	resourceLogs := logs.ResourceLogs().AppendEmpty()
	scopeLogs := resourceLogs.ScopeLogs().AppendEmpty()
	scopeLogs.Scope().SetName(scopeName)
	return logs, resourceLogs, scopeLogs
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
	serviceName := "aws-api"
	if record.EventSource != "" {
		parts := strings.Split(record.EventSource, ".")
		if len(parts) > 0 {
			serviceName = "aws-" + parts[0] + "-api"
		}
	}

	attrs.PutStr("service.name", serviceName)
	attrs.PutStr("cloud.provider", cloudProvider)
	attrs.PutStr("cloud.region", record.AwsRegion)
	attrs.PutStr("cloud.account.id", record.RecipientAccountID)
}

func (u *CloudTrailLogsUnmarshaler) setLogRecord(logRecord plog.LogRecord, record CloudTrailRecord) {
	if t, err := time.Parse(time.RFC3339, record.EventTime); err == nil {
		logRecord.SetTimestamp(pcommon.NewTimestampFromTime(t))
	}
	logRecord.SetSeverityText("INFO")
	u.setLogBody(logRecord, record)
	u.setLogAttributes(logRecord.Attributes(), record)
}

func (u *CloudTrailLogsUnmarshaler) setLogBody(logRecord plog.LogRecord, record CloudTrailRecord) {
	var bodyMsg string

	service := record.EventSource
	if parts := strings.Split(record.EventSource, "."); len(parts) > 0 {
		service = strings.ToUpper(parts[0])
	}

	targetUser := ""
	if record.RequestParameters != nil {
		if userName, ok := record.RequestParameters["userName"].(string); ok {
			targetUser = fmt.Sprintf(" for user \"%s\"", userName)
		}
	}

	bodyMsg = fmt.Sprintf("%s %s API call%s", service, record.EventName, targetUser)
	logRecord.Body().SetStr(bodyMsg)
}

func (u *CloudTrailLogsUnmarshaler) setLogAttributes(attrs pcommon.Map, record CloudTrailRecord) {
	attrs.PutStr("event.name", record.EventName)
	attrs.PutStr("event.id", record.EventID)
	attrs.PutStr("event.type", record.EventType)
	attrs.PutStr("event.category", record.EventCategory)

	attrs.PutStr("aws.service", record.EventSource)
	attrs.PutStr("aws.operation", record.EventName)
	attrs.PutStr("aws.request_id", record.RequestID)

	if record.SessionCredentialFromConsole == "true" {
		attrs.PutBool("aws.session.console", true)
	}
	attrs.PutBool("aws.event.read_only", record.ReadOnly)

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

	attrs.PutStr("net.peer.ip", record.SourceIPAddress)
	attrs.PutStr("user_agent.original", record.UserAgent)

	if record.TLSDetails != nil {
		if tlsVersion, ok := record.TLSDetails["tlsVersion"].(string); ok {
			attrs.PutStr("tls.version", tlsVersion)
		}
		if cipherSuite, ok := record.TLSDetails["cipherSuite"].(string); ok {
			attrs.PutStr("tls.cipher", cipherSuite)
		}
		if hostHeader, ok := record.TLSDetails["clientProvidedHostHeader"].(string); ok {
			attrs.PutStr("server.address", hostHeader)
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
