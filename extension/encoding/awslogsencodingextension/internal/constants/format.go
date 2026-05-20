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

	FileFormatPlainText     = "plain-text"
	FileFormatParquet       = "parquet"
	FormatIdentificationTag = "encoding.format"

	// Feature gate for VPC flow start field ISO-8601 format
	VPCFlowStartISO8601FormatID = "extension.awslogsencoding.vpcflow.start.iso8601"

	// CloudTrailEnableUserIdentityPrefixID defines the feature gate ID to control `aws.user_identity` prefix for decoded CloudTrail logs.
	CloudTrailEnableUserIdentityPrefixID = "extension.awslogsencoding.cloudtrail.enable.user.identity.prefix"
)
