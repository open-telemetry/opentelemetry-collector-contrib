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
)
