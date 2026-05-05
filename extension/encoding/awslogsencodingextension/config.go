// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awslogsencodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension"

import (
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/confmap/xconfmap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/constants"
	subscriptionfilter "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/unmarshaler/subscription-filter"
	vpcflowlog "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/unmarshaler/vpc-flow-log"
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
	// Format selects the AWS logs format. See supportedLogFormats for valid values.
	Format string `mapstructure:"format"`

	VPCFlowLogConfig vpcflowlog.Config `mapstructure:"vpcflow"`
	// Deprecated: use VPCFlowLogConfig. Will be removed in v0.138.0.
	VPCFlowLogConfigV1 vpcflowlog.Config `mapstructure:"vpc_flow_log"`

	// CloudWatch is consulted only when Format is a CloudWatch subscription-filter format.
	CloudWatch CloudWatchConfig `mapstructure:"cloudwatch"`

	// prevent unkeyed literal initialization
	_ struct{}
}

type CloudWatchConfig struct {
	// Streams routes subscription-filter events to inner encoding extensions
	// by logGroup/logStream pattern or service name. Empty means no routing.
	Streams []subscriptionfilter.CloudWatchStream `mapstructure:"streams"`

	// prevent unkeyed literal initialization
	_ struct{}
}

func (cfg *Config) Validate() error {
	var errs []error

	switch cfg.Format {
	case "":
		errs = append(errs, fmt.Errorf("format unspecified, expected one of %q", supportedLogFormats))
	case constants.FormatCloudWatchLogsSubscriptionFilter,
		constants.FormatCloudWatchLogsSubscriptionFilterV1,
		constants.FormatVPCFlowLogV1,
		constants.FormatVPCFlowLog,
		constants.FormatS3AccessLogV1,
		constants.FormatS3AccessLog,
		constants.FormatWAFLogV1,
		constants.FormatWAFLog,
		constants.FormatCloudTrailLogV1,
		constants.FormatCloudTrailLog,
		constants.FormatELBAccessLogV1,
		constants.FormatELBAccessLog,
		constants.FormatNetworkFirewallLog:
	default:
		errs = append(errs, fmt.Errorf("unsupported format %q, expected one of %q", cfg.Format, supportedLogFormats))
	}

	switch cfg.VPCFlowLogConfig.FileFormat {
	case constants.FileFormatParquet, constants.FileFormatPlainText:
	default:
		errs = append(errs, fmt.Errorf(
			"unsupported file format %q for VPC flow log, expected one of %q",
			cfg.VPCFlowLogConfig.FileFormat,
			supportedVPCFlowLogFileFormat,
		))
	}

	// to be deprecated in v0.138.0
	switch cfg.VPCFlowLogConfigV1.FileFormat {
	case constants.FileFormatParquet, constants.FileFormatPlainText:
	default:
		errs = append(errs, fmt.Errorf(
			"unsupported file format %q for VPC flow log, expected one of %q",
			cfg.VPCFlowLogConfigV1.FileFormat,
			supportedVPCFlowLogFileFormat,
		))
	}

	if len(cfg.CloudWatch.Streams) > 0 {
		switch cfg.Format {
		case constants.FormatCloudWatchLogsSubscriptionFilter,
			constants.FormatCloudWatchLogsSubscriptionFilterV1:
		default:
			errs = append(errs, fmt.Errorf(
				"'cloudwatch.streams' is only valid with format %q; got %q",
				constants.FormatCloudWatchLogsSubscriptionFilter, cfg.Format,
			))
		}
		if err := subscriptionfilter.ValidateStreams(cfg.CloudWatch.Streams); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}
