// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cloudtraillog // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/unmarshaler/cloudtraillog"

import (
	"fmt"
	"io"
	"strconv"
	"time"

	gojson "github.com/goccy/go-json"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	conventions "go.opentelemetry.io/otel/semconv/v1.38.0"

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
	Type             string          `json:"type"`
	PrincipalID      string          `json:"principalId"`
	ARN              string          `json:"arn"`
	AccountID        string          `json:"accountId"`
	AccessKeyID      string          `json:"accessKeyId"`
	UserName         string          `json:"userName"`
	UserID           string          `json:"userId"`
	IdentityStoreARN string          `json:"identityStoreArn"`
	InvokedBy        string          `json:"invokedBy"`
	SessionContext   *SessionContext `json:"sessionContext"`
}

// SessionContext if request was made with temporary security credentials,
// provides information about the session created for credentials.
type SessionContext struct {
	Attributes    *SessionContextAttributes `json:"attributes"`
	SessionIssuer *SessionIssuer            `json:"sessionIssuer"`
}

// SessionContextAttributes provides additional attributes for the session.
type SessionContextAttributes struct {
	MFAAuthenticated string `json:"mfaAuthenticated"`
	CreationDate     string `json:"creationDate"`
}

// SessionIssuer provides information about how the user obtained credentials.
type SessionIssuer struct {
	Type        string `json:"type"`
	PrincipalID string `json:"principalId"`
	ARN         string `json:"arn"`
	AccountID   string `json:"accountId"`
	UserName    string `json:"userName"`
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
	APIVersion                   string         `json:"apiVersion"`
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
	AdditionalEventData          map[string]any `json:"additionalEventData"`
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

// logFiles represents log file information in a CloudTrail digest file
type logFiles struct {
	S3Bucket        string `json:"s3Bucket"`
	S3Object        string `json:"s3Object"`
	NewestEventTime string `json:"newestEventTime"`
	OldestEventTime string `json:"oldestEventTime"`
}

// CloudTrailDigest represents a CloudTrail digest file
type CloudTrailDigest struct {
	AWSAccountID           string     `json:"awsAccountId"`
	DigestStartTime        string     `json:"digestStartTime"`
	DigestEndTime          string     `json:"digestEndTime"`
	DigestS3Bucket         string     `json:"digestS3Bucket"`
	DigestS3Object         string     `json:"digestS3Object"`
	NewestEventTime        string     `json:"newestEventTime"`
	OldestEventTime        string     `json:"oldestEventTime"`
	PreviousDigestS3Bucket string     `json:"previousDigestS3Bucket"`
	PreviousDigestS3Object string     `json:"previousDigestS3Object"`
	LogFiles               []logFiles `json:"logFiles"`
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
		return plog.Logs{}, fmt.Errorf("failed to unmarshal payload as CloudTrail logs: %w", err)
	}

	if cloudTrailLog.Records != nil {
		return u.processRecords(cloudTrailLog.Records)
	}

	// Try to parse as a CloudTrail digest record
	var cloudTrailDigest CloudTrailDigest
	if err := gojson.Unmarshal(decompressedBuf, &cloudTrailDigest); err != nil {
		return plog.Logs{}, fmt.Errorf("failed to unmarshal payload as a CloudTrail digest: %w", err)
	}

	return u.processDigestRecord(cloudTrailDigest)
}

func (u *CloudTrailLogUnmarshaler) processRecords(records []CloudTrailRecord) (plog.Logs, error) {
	logs := plog.NewLogs()

	if len(records) == 0 {
		return logs, nil
	}

	// Create a single resource logs entry for all records
	resourceLogs := logs.ResourceLogs().AppendEmpty()
	scopeLogs := resourceLogs.ScopeLogs().AppendEmpty()
	u.setCommonScopeAttributes(scopeLogs)

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

func (u *CloudTrailLogUnmarshaler) processDigestRecord(record CloudTrailDigest) (plog.Logs, error) {
	logs := plog.NewLogs()

	resourceLog := logs.ResourceLogs().AppendEmpty()
	scopeLog := resourceLog.ScopeLogs().AppendEmpty()
	u.setCommonScopeAttributes(scopeLog)

	logRecord := scopeLog.LogRecords().AppendEmpty()
	t, err := time.Parse(time.RFC3339, record.DigestStartTime)
	if err != nil {
		return plog.Logs{}, fmt.Errorf("failed to parse start time: %w", err)
	}
	logRecord.SetTimestamp(pcommon.NewTimestampFromTime(t))

	attributes := logRecord.Attributes()
	attributes.PutStr(string(conventions.CloudAccountIDKey), record.AWSAccountID)
	attributes.PutStr("aws.cloudtrail.digest.end_time", record.DigestEndTime)
	attributes.PutStr("aws.cloudtrail.digest.s3_bucket", record.DigestS3Bucket)
	attributes.PutStr("aws.cloudtrail.digest.s3_object", record.DigestS3Object)

	// following attributes may be empty (null)
	if record.NewestEventTime != "" {
		attributes.PutStr("aws.cloudtrail.digest.newest_event", record.NewestEventTime)
	}

	if record.OldestEventTime != "" {
		attributes.PutStr("aws.cloudtrail.digest.oldest_event", record.OldestEventTime)
	}

	if record.PreviousDigestS3Bucket != "" {
		attributes.PutStr("aws.cloudtrail.digest.previous_s3_bucket", record.PreviousDigestS3Bucket)
	}

	if record.PreviousDigestS3Object != "" {
		attributes.PutStr("aws.cloudtrail.digest.previous_s3_object", record.PreviousDigestS3Object)
	}

	if len(record.LogFiles) > 0 {
		logFilesArray := attributes.PutEmptySlice("aws.cloudtrail.digest.log_files")
		for _, logFile := range record.LogFiles {
			logFileMap := logFilesArray.AppendEmpty().SetEmptyMap()
			logFileMap.PutStr("s3_bucket", logFile.S3Bucket)
			logFileMap.PutStr("s3_object", logFile.S3Object)
			logFileMap.PutStr("newest_event_time", logFile.NewestEventTime)
			logFileMap.PutStr("oldest_event_time", logFile.OldestEventTime)
		}
	}

	return logs, nil
}

func (u *CloudTrailLogUnmarshaler) setCommonScopeAttributes(scope plog.ScopeLogs) {
	scope.Scope().SetName(metadata.ScopeName)
	scope.Scope().SetVersion(u.buildInfo.Version)
	scope.Scope().Attributes().PutStr(constants.FormatIdentificationTag, "aws."+constants.FormatCloudTrailLog)
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

	if record.APIVersion != "" {
		attrs.PutStr("aws.cloudtrail.api_version", record.APIVersion)
	}

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

		if record.UserIdentity.AccountID != "" {
			attrs.PutStr("aws.user_identity.account_id", record.UserIdentity.AccountID)
		}

		if record.UserIdentity.AccessKeyID != "" {
			attrs.PutStr("aws.access_key.id", record.UserIdentity.AccessKeyID)
		}

		// Store the Identity Store ARN and others as custom attributes
		// since there are no standard conventions for them
		if record.UserIdentity.IdentityStoreARN != "" {
			attrs.PutStr("aws.identity_store.arn", record.UserIdentity.IdentityStoreARN)
		}

		if record.UserIdentity.InvokedBy != "" {
			attrs.PutStr("aws.user_identity.invoked_by", record.UserIdentity.InvokedBy)
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

		// Add session context details if available
		if record.UserIdentity.SessionContext != nil {
			enrichWithSessionContext(attrs, record.UserIdentity.SessionContext)
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

	if record.AdditionalEventData != nil {
		additionalDataMap := attrs.PutEmptyMap("aws.cloudtrail.additional_event_data")
		_ = additionalDataMap.FromRaw(record.AdditionalEventData)
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

// enrichWithSessionContext is a helper to add SessionContext details to log attributes.
// Root level Attributes will be added with aws.user_identity.session_context prefix.
// SessionContextAttributes will be added with aws.user_identity.session_context.attributes prefix.
// SessionIssuer details will be added with aws.user_identity.session_context.issuer prefix.
func enrichWithSessionContext(attrs pcommon.Map, sessionContext *SessionContext) {
	if sessionContext.Attributes != nil {
		if sessionContext.Attributes.MFAAuthenticated != "" {
			b, err := strconv.ParseBool(sessionContext.Attributes.MFAAuthenticated)
			if err == nil {
				// only append boolean converted value if no error in conversion
				attrs.PutBool("aws.user_identity.session_context.attributes.mfa_authenticated", b)
			}
		}

		if sessionContext.Attributes.CreationDate != "" {
			attrs.PutStr("aws.user_identity.session_context.attributes.creation_date", sessionContext.Attributes.CreationDate)
		}
	}

	if sessionContext.SessionIssuer != nil {
		if sessionContext.SessionIssuer.Type != "" {
			attrs.PutStr("aws.user_identity.session_context.issuer.type", sessionContext.SessionIssuer.Type)
		}

		if sessionContext.SessionIssuer.PrincipalID != "" {
			attrs.PutStr("aws.user_identity.session_context.issuer.principal_id", sessionContext.SessionIssuer.PrincipalID)
		}

		if sessionContext.SessionIssuer.ARN != "" {
			attrs.PutStr("aws.user_identity.session_context.issuer.arn", sessionContext.SessionIssuer.ARN)
		}

		if sessionContext.SessionIssuer.AccountID != "" {
			attrs.PutStr("aws.user_identity.session_context.issuer.account_id", sessionContext.SessionIssuer.AccountID)
		}

		if sessionContext.SessionIssuer.UserName != "" {
			attrs.PutStr("aws.user_identity.session_context.issuer.user_name", sessionContext.SessionIssuer.UserName)
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
