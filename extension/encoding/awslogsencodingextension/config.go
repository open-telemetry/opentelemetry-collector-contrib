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
		constants.FormatCloudWatchLogsSubscriptionFilter,
		constants.FormatVPCFlowLog,
		constants.FormatS3AccessLog,
		constants.FormatWAFLog,
		constants.FormatCloudTrailLog,
		constants.FormatELBAccessLog,
		constants.FormatNetworkFirewallLog,
	}
	supportedVPCFlowLogFileFormat = []string{constants.FileFormatPlainText, constants.FileFormatParquet}
)

type Config struct {
	// Format selects the AWS logs format. See supportedLogFormats for valid values.
	Format string `mapstructure:"format"`

	VPCFlowLogConfig vpcflowlog.Config `mapstructure:"vpcflow"`

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
	case constants.FormatCloudWatchLogsSubscriptionFilter: // valid
	case constants.FormatVPCFlowLog: // valid
	case constants.FormatS3AccessLog: // valid
	case constants.FormatWAFLog: // valid
	case constants.FormatCloudTrailLog: // valid
	case constants.FormatELBAccessLog: // valid
	case constants.FormatNetworkFirewallLog: // valid
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

	if len(cfg.CloudWatch.Streams) > 0 {
		if cfg.Format != constants.FormatCloudWatchLogsSubscriptionFilter {
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
