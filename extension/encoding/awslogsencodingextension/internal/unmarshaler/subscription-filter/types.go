// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package subscriptionfilter // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/unmarshaler/subscription-filter"

// extractedFields contains the extracted fields from CloudWatch Logs events.
// These fields are present when emitSystemFields is enabled in the CloudWatch Logs
// subscription filter, which is useful for centralized logging scenarios where logs
// from multiple AWS accounts and regions are consolidated.
type extractedFields struct {
	AccountID string `json:"@aws.account,omitempty"`
	Region    string `json:"@aws.region,omitempty"`
}

// cloudwatchLogsLogEvent represents a single log event from CloudWatch Logs.
// This is a custom type that extends the AWS SDK's CloudwatchLogsLogEvent
// to include extractedFields support.
type cloudwatchLogsLogEvent struct {
	ID              string           `json:"id"`
	Timestamp       int64            `json:"timestamp"`
	Message         string           `json:"message"`
	ExtractedFields *extractedFields `json:"extractedFields,omitempty"`
}

// cloudwatchLogsData represents the CloudWatch Logs data structure.
// This is a custom type that replaces the AWS SDK's CloudwatchLogsData
// to support extractedFields in log events.
type cloudwatchLogsData struct {
	Owner               string                   `json:"owner"`
	LogGroup            string                   `json:"logGroup"`
	LogStream           string                   `json:"logStream"`
	SubscriptionFilters []string                 `json:"subscriptionFilters"`
	MessageType         string                   `json:"messageType"`
	LogEvents           []cloudwatchLogsLogEvent `json:"logEvents"`
}

// resourceGroupKey represents the combination of account ID, region,
// log group, and log stream used for grouping log events into separate ResourceLogs.
type resourceGroupKey struct {
	accountID string
	region    string
	logGroup  string
	logStream string
}
