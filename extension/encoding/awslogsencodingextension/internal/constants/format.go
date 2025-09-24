// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package constants // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/constants"

const (
	FormatCloudWatchLogsSubscriptionFilter = "cloudwatch_subscription_filter"
	FormatVPCFlowLog                       = "vpcflow"
	FormatS3AccessLog                      = "s3access"
	FormatWAFLog                           = "waf"
	FormatCloudTrailLog                    = "cloudtrail"
	FormatELBAccessLog                     = "elbaccess"

	FileFormatPlainText     = "plain-text"
	FileFormatParquet       = "parquet"
	FormatIdentificationTag = "awslogs_encoding.format"
)
