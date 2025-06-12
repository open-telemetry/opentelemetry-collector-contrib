// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awslogsencodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding"
	s3accesslog "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/unmarshaler/s3-access-log"
	subscriptionfilter "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/unmarshaler/subscription-filter"
	vpcflowlog "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/unmarshaler/vpc-flow-log"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/unmarshaler/waf"
)

var _ encoding.LogsUnmarshalerExtension = (*encodingExtension)(nil)

type encodingExtension struct {
	unmarshaler plog.Unmarshaler
	format      string
}

func newExtension(cfg *Config, settings extension.Settings) (*encodingExtension, error) {
	switch cfg.Format {
	case formatCloudWatchLogsSubscriptionFilter:
		return &encodingExtension{
			unmarshaler: subscriptionfilter.NewSubscriptionFilterUnmarshaler(settings.BuildInfo),
			format:      formatCloudWatchLogsSubscriptionFilter,
		}, nil
	case formatVPCFlowLog:
		unmarshaler, err := vpcflowlog.NewVPCFlowLogUnmarshaler(
			cfg.VPCFlowLogConfig.FileFormat,
			settings.BuildInfo,
			settings.Logger,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create encoding extension for %q format: %w", formatVPCFlowLog, err)
		}
		return &encodingExtension{
			unmarshaler: unmarshaler,
			format:      formatVPCFlowLog,
		}, nil
	case formatS3AccessLog:
		return &encodingExtension{
			unmarshaler: s3accesslog.NewS3AccessLogUnmarshaler(settings.BuildInfo),
			format:      formatS3AccessLog,
		}, nil
	case formatWAFLog:
		return &encodingExtension{
			unmarshaler: waf.NewWAFLogUnmarshaler(settings.BuildInfo),
			format:      formatWAFLog,
		}, nil
	default:
		// Format will have been validated by Config.Validate,
		// so we'll only get here if we haven't handled a valid
		// format.
		return nil, fmt.Errorf("unimplemented format %q", cfg.Format)
	}
}

func (*encodingExtension) Start(_ context.Context, _ component.Host) error {
	return nil
}

func (*encodingExtension) Shutdown(_ context.Context) error {
	return nil
}

func (e *encodingExtension) UnmarshalLogs(buf []byte) (plog.Logs, error) {
	logs, err := e.unmarshaler.UnmarshalLogs(buf)
	if err != nil {
		return plog.Logs{}, fmt.Errorf("failed to unmarshal logs as %q format: %w", e.format, err)
	}
	return logs, nil
}
