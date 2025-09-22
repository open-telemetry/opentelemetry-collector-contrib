// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awslogsencodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension"

import (
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/confmap/xconfmap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/constants"
)

var _ xconfmap.Validator = (*Config)(nil)

var (
	supportedLogFormats = []string{
		constants.FormatCloudWatchLogsSubscriptionFilter,
		constants.FormatVPCFlowLog,
		constants.FormatS3AccessLog,
		constants.FormatWAFLog,
		constants.FormatCloudTrailLog,
		constants.FormatELBAccessLog,
	}
	supportedVPCFlowLogFileFormat = []string{constants.FileFormatPlainText, constants.FileFormatParquet}
)

type Config struct {
	// Format defines the AWS logs format.
	//
	// Current valid values are:
	// - cloudwatch_logs_subscription_filter
	// - vpc_flow_log
	// - s3_access_log
	// - waf_log
	// - cloudtrail_log
	// - elb_access_log
	//
	Format string `mapstructure:"format"`

	VPCFlowLogConfig VPCFlowLogConfig `mapstructure:"vpc_flow_log"`

	// prevent unkeyed literal initialization
	_ struct{}
}

type VPCFlowLogConfig struct {
	// VPC flow logs sent to S3 have support
	// for file format in plain text or
	// parquet. Default is plain text.
	//
	// See https://docs.aws.amazon.com/vpc/latest/userguide/flow-logs-s3-path.html.
	FileFormat string `mapstructure:"file_format"`
	// prevent unkeyed literal initialization
	_ struct{}
}

func (cfg *Config) Validate() error {
	var errs []error

	switch cfg.Format {
	case "":
		errs = append(errs, fmt.Errorf("format unspecified, expected one of %q", supportedLogFormats))
	case constants.FormatCloudWatchLogsSubscriptionFilter: // valid
	case constants.FormatVPCFlowLog: // valid
	case constants.FormatS3AccessLog: // valid
	case constants.FormatWAFLog: // valid
	case constants.FormatCloudTrailLog: // valid
	case constants.FormatELBAccessLog: // valid
	default:
		errs = append(errs, fmt.Errorf("unsupported format %q, expected one of %q", cfg.Format, supportedLogFormats))
	}

	switch cfg.VPCFlowLogConfig.FileFormat {
	case constants.FileFormatParquet: // valid
	case constants.FileFormatPlainText: // valid
	default:
		errs = append(errs, fmt.Errorf(
			"unsupported file format %q for VPC flow log, expected one of %q",
			cfg.VPCFlowLogConfig.FileFormat,
			supportedVPCFlowLogFileFormat,
		))
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}
