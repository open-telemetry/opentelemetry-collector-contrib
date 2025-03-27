// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awslogsencodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension"

import (
	"fmt"

	"go.opentelemetry.io/collector/confmap/xconfmap"
)

var _ xconfmap.Validator = (*Config)(nil)

const (
	formatCloudWatchLogsSubscriptionFilter = "cloudwatch_logs_subscription_filter"
)

var supportedFormats = []string{formatCloudWatchLogsSubscriptionFilter}

type Config struct {
	// Format defines the AWS logs format.
	//
	// Valid values are:
	// - cloudwatch_logs_subscription_filter
	//
	Format string `mapstructure:"format"`
}

func (cfg *Config) Validate() error {
	switch cfg.Format {
	case "":
		return fmt.Errorf("format unspecified, expected one of %q", supportedFormats)
	case formatCloudWatchLogsSubscriptionFilter:
		// valid
	default:
		return fmt.Errorf("unsupported format %q, expected one of %q", cfg.Format, supportedFormats)
	}
	return nil
}
