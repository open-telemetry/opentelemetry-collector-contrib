// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cloudtraillog // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/unmarshaler/cloudtraillog"

import (
	"fmt"
	"io"
	"time"

	gojson "github.com/goccy/go-json"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	conventions "go.opentelemetry.io/otel/semconv/v1.27.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/constants"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/unmarshaler"
)

type CloudTrailLogUnmarshaler struct {
	buildInfo component.BuildInfo
}

var _ unmarshaler.AWSUnmarshaler = (*CloudTrailLogUnmarshaler)(nil)

// UserIdentity represents the user identity information in CloudTrail logs
type UserIdentity struct {
	Type             string `json:"type"`
	PrincipalID      string `json:"principalId"`
	ARN              string `json:"arn"`
	AccountID        string `json:"accountId"`
	AccessKeyID      string `json:"accessKeyId"`
	UserName         string `json:"userName"`
	UserID           string `json:"userId"`
	IdentityStoreARN string `json:"identityStoreArn"`
}

// TLSDetails represents the TLS connection details in CloudTrail logs
type TLSDetails struct {
	TLSVersion               string `json:"tlsVersion"`
	CipherSuite              string `json:"cipherSuite"`
	ClientProvidedHostHeader string `json:"clientProvidedHostHeader"`
}

// Resource represents a resource referenced in CloudTrail logs
type Resource struct {
	AccountID string `json:"accountId"`
	Type      string `json:"type"`
	ARN       string `json:"ARN"`
}

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
	UserIdentity                 *UserIdentity  `json:"userIdentity"`
	ResponseElements             map[string]any `json:"responseElements"`
	RequestParameters            map[string]any `json:"requestParameters"`
	Resources                    []*Resource    `json:"resources"`
	ReadOnly                     *bool          `json:"readOnly"`
	ManagementEvent              *bool          `json:"managementEvent"`
	TLSDetails                   *TLSDetails    `json:"tlsDetails"`
	SessionCredentialFromConsole string         `json:"sessionCredentialFromConsole"`
	ErrorCode                    string         `json:"errorCode"`
	ErrorMessage                 string         `json:"errorMessage"`
	InsightDetails               map[string]any `json:"insightDetails"`
	SharedEventID                string         `json:"sharedEventID"`
}

type CloudTrailLog struct {
	Records []CloudTrailRecord `json:"Records"`
}

func NewCloudTrailLogUnmarshaler(buildInfo component.BuildInfo) *CloudTrailLogUnmarshaler {
	return &CloudTrailLogUnmarshaler{
		buildInfo: buildInfo,
	}
}

func (u *CloudTrailLogUnmarshaler) UnmarshalAWSLogs(reader io.Reader) (plog.Logs, error) {
	decompressedBuf, err := io.ReadAll(reader)
	if err != nil {
		return plog.Logs{}, fmt.Errorf("failed to read CloudTrail logs: %w", err)
	}

	var cloudTrailLog CloudTrailLog
	if err := gojson.Unmarshal(decompressedBuf, &cloudTrailLog); err != nil {
		return plog.Logs{}, fmt.Errorf("failed to unmarshal CloudTrail logs: %w", err)
	}

	return u.processRecords(cloudTrailLog.Records)
}

func (u *CloudTrailLogUnmarshaler) processRecords(records []CloudTrailRecord) (plog.Logs, error) {
	logs := plog.NewLogs()

	if len(records) == 0 {
		return logs, nil
	}

	// Create a single resource logs entry for all records
	resourceLogs := logs.ResourceLogs().AppendEmpty()
	scopeLogs := resourceLogs.ScopeLogs().AppendEmpty()
	scopeLogs.Scope().SetName(metadata.ScopeName)
	scopeLogs.Scope().SetVersion(u.buildInfo.Version)
	scopeLogs.Scope().Attributes().PutStr(constants.FormatIdentificationTag, constants.FormatCloudTrailLog)

	// Set resource attributes based on the first record
	// (all records have the same account ID and region)
	u.setResourceAttributes(resourceLogs.Resource().Attributes(), records[0])

	for i := range records {
		record := &records[i]
		logRecord := scopeLogs.LogRecords().AppendEmpty()
		if err := u.setLogRecord(logRecord, record); err != nil {
			return plog.Logs{}, err
		}
	}

	return logs, nil
}

func (*CloudTrailLogUnmarshaler) setResourceAttributes(attrs pcommon.Map, record CloudTrailRecord) {
	attrs.PutStr(string(conventions.CloudProviderKey), conventions.CloudProviderAWS.Value.AsString())
	attrs.PutStr(string(conventions.CloudRegionKey), record.AwsRegion)
	attrs.PutStr(string(conventions.CloudAccountIDKey), record.RecipientAccountID)
}

func (u *CloudTrailLogUnmarshaler) setLogRecord(logRecord plog.LogRecord, record *CloudTrailRecord) error {
	t, err := time.Parse(time.RFC3339, record.EventTime)
	if err != nil {
		return fmt.Errorf("failed to parse timestamp of log: %w", err)
	}
	logRecord.SetTimestamp(pcommon.NewTimestampFromTime(t))
	u.setLogAttributes(logRecord.Attributes(), record)
	return nil
}

func (*CloudTrailLogUnmarshaler) setLogAttributes(attrs pcommon.Map, record *CloudTrailRecord) {
	attrs.PutStr("aws.cloudtrail.event_version", record.EventVersion)

	attrs.PutStr("aws.cloudtrail.event_id", record.EventID)

	if record.EventName != "" {
		attrs.PutStr(string(conventions.RPCMethodKey), record.EventName)
	}

	attrs.PutStr(string(conventions.RPCSystemKey), record.EventType)

	if record.EventSource != "" {
		attrs.PutStr(string(conventions.RPCServiceKey), record.EventSource)
	}

	if record.RequestID != "" {
		attrs.PutStr(string(conventions.AWSRequestIDKey), record.RequestID)
	}

	attrs.PutStr("aws.event.category", record.EventCategory)

	if record.ReadOnly != nil {
		attrs.PutBool("aws.event.read_only", *record.ReadOnly)
	}

	if record.ManagementEvent != nil {
		attrs.PutBool("aws.event.management", *record.ManagementEvent)
	}

	if record.SourceIPAddress != "" {
		attrs.PutStr(string(conventions.SourceAddressKey), record.SourceIPAddress)
	}

	if record.UserAgent != "" {
		attrs.PutStr(string(conventions.UserAgentOriginalKey), record.UserAgent)
	}

	if record.SessionCredentialFromConsole == "true" {
		attrs.PutBool("aws.session.console", true)
	}

	if record.UserIdentity != nil {
		if record.UserIdentity.UserID != "" {
			attrs.PutStr(string(conventions.UserIDKey), record.UserIdentity.UserID)
		}

		if record.UserIdentity.UserName != "" {
			attrs.PutStr(string(conventions.UserNameKey), record.UserIdentity.UserName)
		}

		if record.UserIdentity.AccessKeyID != "" {
			attrs.PutStr("aws.access_key.id", record.UserIdentity.AccessKeyID)
		}

		// Store the Identity Store ARN and others as custom attributes
		// since there are no standard conventions for them
		if record.UserIdentity.IdentityStoreARN != "" {
			attrs.PutStr("aws.identity_store.arn", record.UserIdentity.IdentityStoreARN)
		}

		if record.UserIdentity.PrincipalID != "" {
			attrs.PutStr("aws.principal.id", record.UserIdentity.PrincipalID)
		}

		if record.UserIdentity.ARN != "" {
			attrs.PutStr("aws.principal.arn", record.UserIdentity.ARN)
		}

		if record.UserIdentity.Type != "" {
			attrs.PutStr("aws.principal.type", record.UserIdentity.Type)
		}
	}

	if record.TLSDetails != nil {
		if record.TLSDetails.TLSVersion != "" {
			// Extract only the version number from TLSv1.2 format
			version := extractTLSVersion(record.TLSDetails.TLSVersion)
			attrs.PutStr(string(conventions.TLSProtocolVersionKey), version)
		}
		if record.TLSDetails.CipherSuite != "" {
			attrs.PutStr(string(conventions.TLSCipherKey), record.TLSDetails.CipherSuite)
		}
		if record.TLSDetails.ClientProvidedHostHeader != "" {
			attrs.PutStr(string(conventions.ServerAddressKey), record.TLSDetails.ClientProvidedHostHeader)
		}
	}

	if record.ErrorCode != "" {
		attrs.PutStr("aws.error.code", record.ErrorCode)
	}

	if record.ErrorMessage != "" {
		attrs.PutStr("aws.error.message", record.ErrorMessage)
	}

	if record.SharedEventID != "" {
		attrs.PutStr("aws.shared_event_id", record.SharedEventID)
	}

	if record.InsightDetails != nil {
		insightDetailsMap := attrs.PutEmptyMap("aws.insight_details")
		_ = insightDetailsMap.FromRaw(record.InsightDetails)
	}

	if record.RequestParameters != nil {
		requestParamsMap := attrs.PutEmptyMap("aws.request.parameters")
		_ = requestParamsMap.FromRaw(record.RequestParameters)
	}

	if record.ResponseElements != nil {
		responseElementsMap := attrs.PutEmptyMap("aws.response.elements")
		_ = responseElementsMap.FromRaw(record.ResponseElements)
	}

	if len(record.Resources) > 0 {
		resourcesArray := attrs.PutEmptySlice("aws.resources")
		for _, resource := range record.Resources {
			resourceMap := resourcesArray.AppendEmpty().SetEmptyMap()
			if resource.AccountID != "" {
				resourceMap.PutStr("account.id", resource.AccountID)
			}
			if resource.Type != "" {
				resourceMap.PutStr("type", resource.Type)
			}
			if resource.ARN != "" {
				resourceMap.PutStr("arn", resource.ARN)
			}
		}
	}
}

// extract the version number from a TLS version string (e.g. "TLSv1.2" becomes "1.2")
func extractTLSVersion(tlsVersion string) string {
	if len(tlsVersion) > 4 && tlsVersion[:4] == "TLSv" {
		return tlsVersion[4:]
	}
	return tlsVersion
}
