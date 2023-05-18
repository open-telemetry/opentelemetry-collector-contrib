// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlecloudpubsubexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/googlecloudpubsubexporter"

import (
	"fmt"
	"regexp"
	"time"

	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

var topicMatcher = regexp.MustCompile(`^projects/[a-z][a-z0-9\-]*/topics/`)

type Config struct {
	// Timeout for all API calls. If not set, defaults to 12 seconds.
	exporterhelper.TimeoutSettings `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct.
	exporterhelper.QueueSettings   `mapstructure:"sending_queue"`
	exporterhelper.RetrySettings   `mapstructure:"retry_on_failure"`
	// Google Cloud Project ID where the Pubsub client will connect to
	ProjectID string `mapstructure:"project"`
	// User agent that will be used by the Pubsub client to connect to the service
	UserAgent string `mapstructure:"user_agent"`
	// Override of the Pubsub endpoint, for testing only
	endpoint string
	// Only has effect if Endpoint is not ""
	insecure bool

	// The fully qualified resource name of the Pubsub topic
	Topic string `mapstructure:"topic"`
	// Compression of the payload (only gzip or is supported, no compression is the default)
	Compression string `mapstructure:"compression"`
	// Watermark defines the watermark (the ce-time attribute on the message) behavior
	Watermark WatermarkConfig `mapstructure:"watermark"`
}

// WatermarkConfig customizes the behavior of the watermark
type WatermarkConfig struct {
	// Behavior of the watermark. Currently, only  of the message (none, earliest and current, current being the default)
	// will set the timestamp on pubsub based on timestamps of the events inside the message
	Behavior string `mapstructure:"behavior"`
	// Indication on how much the timestamp can drift from the current time, the timestamp will be capped to the allowed
	// maximum. A duration of 0 is the same as maximum duration
	AllowedDrift time.Duration `mapstructure:"allowed_drift"`
}

func (config *Config) Validate() error {
	if !topicMatcher.MatchString(config.Topic) {
		return fmt.Errorf("topic '%s' is not a valid format, use 'projects/<project_id>/topics/<name>'", config.Topic)
	}
	_, err := config.parseCompression()
	if err != nil {
		return err
	}
	return config.Watermark.validate()
}

func (config *WatermarkConfig) validate() error {
	if config.AllowedDrift == 0 {
		config.AllowedDrift = 1<<63 - 1
	}
	_, err := config.parseWatermarkBehavior()
	return err
}

func (config *Config) parseCompression() (compression, error) {
	switch config.Compression {
	case "gzip":
		return gZip, nil
	case "":
		return uncompressed, nil
	}
	return uncompressed, fmt.Errorf("compression %v is not supported.  supported compression formats include [gzip]", config.Compression)
}

func (config *WatermarkConfig) parseWatermarkBehavior() (WatermarkBehavior, error) {
	switch config.Behavior {
	case "earliest":
		return earliest, nil
	case "current":
		return current, nil
	case "":
		return current, nil
	}
	return current, fmt.Errorf("behavior %v is not supported.  supported compression formats include [current,earliest]", config.Behavior)
}
