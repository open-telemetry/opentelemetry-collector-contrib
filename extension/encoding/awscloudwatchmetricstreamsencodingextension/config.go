// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awscloudwatchmetricstreamsencodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awscloudwatchmetricstreamsencodingextension"
import (
	"fmt"

	"go.opentelemetry.io/collector/confmap/xconfmap"
)

var _ xconfmap.Validator = (*Config)(nil)

const (
	formatJSON            = "json"
	formatOpenTelemetry10 = "opentelemetry1.0"
)

var supportedFormats = []string{formatOpenTelemetry10, formatJSON}

type Config struct {
	// Format defines the CloudWatch Metric Streams format.
	//
	// Valid values are "json" and "opentelemetry1.0"; opentelemetry0.7 is unsupported.
	//
	// See https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-cloudwatch-metricstream.html#cfn-cloudwatch-metricstream-outputformat
	Format string `mapstructure:"format"`

	// prevent unkeyed literal initialization
	_ struct{}
}

func (cfg *Config) Validate() error {
	switch cfg.Format {
	case "":
		return fmt.Errorf("format unspecified, expected one of %q", supportedFormats)
	case formatJSON, formatOpenTelemetry10:
		// valid
	default:
		return fmt.Errorf("unsupported format %q, expected one of %q", cfg.Format, supportedFormats)
	}
	return nil
}
