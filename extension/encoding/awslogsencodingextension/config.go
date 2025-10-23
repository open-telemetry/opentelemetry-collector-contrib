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
		// New format values
		constants.FormatCloudWatchLogsSubscriptionFilter,
		constants.FormatVPCFlowLog,
		constants.FormatS3AccessLog,
		constants.FormatWAFLog,
		constants.FormatCloudTrailLog,
		constants.FormatELBAccessLog,
		constants.FormatNetworkFirewallLog,
		// Legacy format values (for backward compatibility)
		constants.FormatCloudWatchLogsSubscriptionFilterV1,
		constants.FormatVPCFlowLogV1,
		constants.FormatS3AccessLogV1,
		constants.FormatWAFLogV1,
		constants.FormatCloudTrailLogV1,
		constants.FormatELBAccessLogV1,
	}
	supportedVPCFlowLogFileFormat = []string{constants.FileFormatPlainText, constants.FileFormatParquet}
)

type Config struct {
	// Format defines the AWS logs format.
	//
	// Current valid values are:
	// - cloudwatch
	// - vpcflow
	// - s3access
	// - waf
	// - cloudtrail
	// - elbaccess
	// - networkfirewall
	//
	Format string `mapstructure:"format"`

	VPCFlowLogConfig VPCFlowLogConfig `mapstructure:"vpcflow"`
	// Deprecated: use VPCFlowLogConfig instead. It will be removed in v0.138.0
	VPCFlowLogConfigV1 VPCFlowLogConfig `mapstructure:"vpc_flow_log"`

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
	case constants.FormatCloudWatchLogsSubscriptionFilterV1: // valid
	case constants.FormatVPCFlowLogV1: // valid
	case constants.FormatVPCFlowLog: // valid
	case constants.FormatS3AccessLogV1: // valid
	case constants.FormatS3AccessLog: // valid
	case constants.FormatWAFLogV1: // valid
	case constants.FormatWAFLog: // valid
	case constants.FormatCloudTrailLogV1: // valid
	case constants.FormatCloudTrailLog: // valid
	case constants.FormatELBAccessLogV1: // valid
	case constants.FormatELBAccessLog: // valid
	case constants.FormatNetworkFirewallLog: // valid
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

	// to be deprecated in v0.138.0
	switch cfg.VPCFlowLogConfigV1.FileFormat {
	case constants.FileFormatParquet: // valid
	case constants.FileFormatPlainText: // valid
	default:
		errs = append(errs, fmt.Errorf(
			"unsupported file format %q for VPC flow log, expected one of %q",
			cfg.VPCFlowLogConfigV1.FileFormat,
			supportedVPCFlowLogFileFormat,
		))
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}
