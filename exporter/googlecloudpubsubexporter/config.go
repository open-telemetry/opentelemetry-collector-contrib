// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlecloudpubsubexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/googlecloudpubsubexporter"

import (
	"errors"
	"fmt"
	"regexp"
	"time"

	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.uber.org/multierr"
)

var topicMatcher = regexp.MustCompile(`^projects/[a-z][a-z0-9\-]*/topics/`)

type Config struct {
	// Timeout for all API calls. If not set, defaults to 12 seconds.
	TimeoutSettings           exporterhelper.TimeoutConfig    `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct.
	QueueSettings             exporterhelper.QueueBatchConfig `mapstructure:"sending_queue"`
	configretry.BackOffConfig `mapstructure:"retry_on_failure"`
	// Google Cloud Project ID where the Pubsub client will connect to
	ProjectID string `mapstructure:"project"`
	// User agent that will be used by the Pubsub client to connect to the service
	UserAgent string `mapstructure:"user_agent"`
	// Override of the Pubsub Endpoint, leave empty for the default endpoint
	Endpoint string `mapstructure:"endpoint"`
	// Only has effect if Endpoint is not ""
	Insecure bool `mapstructure:"insecure"`

	// The fully qualified resource name of the Pubsub topic
	Topic string `mapstructure:"topic"`
	// Compression of the payload (only gzip or is supported, no compression is the default)
	Compression string `mapstructure:"compression"`
	// Watermark defines the watermark (the ce-time attribute on the message) behavior
	Watermark WatermarkConfig `mapstructure:"watermark"`
	// Ordering configures the ordering keys
	Ordering OrderingConfig `mapstructure:"ordering"`
}

// WatermarkConfig customizes the behavior of the watermark
type WatermarkConfig struct {
	// Behavior of the watermark. Currently, only of the message (none, earliest and current, current being the default)
	// will set the timestamp on pubsub based on timestamps of the events inside the message
	Behavior string `mapstructure:"behavior"`
	// Indication on how much the timestamp can drift from the current time, the timestamp will be capped to the allowed
	// maximum. A duration of 0 is the same as maximum duration
	AllowedDrift time.Duration `mapstructure:"allowed_drift"`
}

// OrderingConfig customizes the behavior of the ordering
type OrderingConfig struct {
	// Enabled indicates if ordering is enabled
	Enabled bool `mapstructure:"enabled"`
	// FromResourceAttribute is a resource attribute that will be used as the ordering key.
	FromResourceAttribute string `mapstructure:"from_resource_attribute"`
	// RemoveResourceAttribute indicates if the ordering key should be removed from the resource attributes.
	RemoveResourceAttribute bool `mapstructure:"remove_resource_attribute"`
}

func (config *Config) Validate() error {
	var errors error
	if !topicMatcher.MatchString(config.Topic) {
		errors = multierr.Append(errors, fmt.Errorf("topic '%s' is not a valid format, use 'projects/<project_id>/topics/<name>'", config.Topic))
	}
	if _, err := config.parseCompression(); err != nil {
		errors = multierr.Append(errors, err)
	}
	errors = multierr.Append(errors, config.Watermark.validate())
	errors = multierr.Append(errors, config.Ordering.validate())
	return errors
}

func (config *WatermarkConfig) validate() error {
	if config.AllowedDrift == 0 {
		config.AllowedDrift = 1<<63 - 1
	}
	_, err := config.parseWatermarkBehavior()
	return err
}

func (cfg *OrderingConfig) validate() error {
	if cfg.Enabled && cfg.FromResourceAttribute == "" {
		return errors.New("'from_resource_attribute' is required if ordering is enabled")
	}
	return nil
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
