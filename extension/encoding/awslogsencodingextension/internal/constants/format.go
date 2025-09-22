// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package constants // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/constants"

const (
	FormatCloudWatchLogsSubscriptionFilter = "cloudwatch_logs_subscription_filter"
	FormatVPCFlowLog                       = "vpc_flow_log"
	FormatS3AccessLog                      = "s3_access_log"
	FormatWAFLog                           = "waf_log"
	FormatCloudTrailLog                    = "cloudtrail_log"
	FormatELBAccessLog                     = "elb_access_log"

	FileFormatPlainText     = "plain-text"
	FileFormatParquet       = "parquet"
	FormatIdentificationTag = "awslogs_encoding.format"
)
