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
)

var _ encoding.LogsUnmarshalerExtension = (*encodingExtension)(nil)

type encodingExtension struct {
	unmarshaler plog.Unmarshaler
	format      string
}

func newExtension(cfg *Config, _ extension.Settings) (*encodingExtension, error) {
	switch cfg.Format {
	case formatCloudWatchLogsSubscriptionFilter:
		return &encodingExtension{
			unmarshaler: cloudWatchLogsSubscriptionFilterUnmarshaler{},
			format:      cfg.Format,
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

func (e *encodingExtension) UnmarshalLogs(record []byte) (plog.Logs, error) {
	logs, err := e.unmarshaler.UnmarshalLogs(record)
	if err != nil {
		return plog.Logs{}, fmt.Errorf("failed to unmarshal logs as '%s' format: %w", e.format, err)
	}
	return logs, nil
}
