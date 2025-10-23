// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package constants // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/constants"

const (
	FormatCloudWatchLogsSubscriptionFilter = "cloudwatch"
	FormatVPCFlowLog                       = "vpcflow"
	FormatS3AccessLog                      = "s3access"
	FormatWAFLog                           = "waf"
	FormatCloudTrailLog                    = "cloudtrail"
	FormatELBAccessLog                     = "elbaccess"
	FormatNetworkFirewallLog               = "networkfirewall"

	// Legacy format values (v1) - kept for backward compatibility
	FormatVPCFlowLogV1                       = "vpc_flow_log"
	FormatELBAccessLogV1                     = "elb_access_log"
	FormatS3AccessLogV1                      = "s3_access_log"
	FormatCloudTrailLogV1                    = "cloudtrail_log"
	FormatWAFLogV1                           = "waf_log"
	FormatCloudWatchLogsSubscriptionFilterV1 = "cloudwatch_logs_subscription_filter"

	FileFormatPlainText     = "plain-text"
	FileFormatParquet       = "parquet"
	FormatIdentificationTag = "encoding.format"

	// Feature gate for VPC flow start field ISO-8601 format
	VPCFlowStartISO8601FormatID = "extension.awslogsencoding.vpcflow.start.iso8601"
)
