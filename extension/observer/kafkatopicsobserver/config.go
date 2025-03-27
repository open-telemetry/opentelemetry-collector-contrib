// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkatopicsobserver // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/kafkatopicsobserver"

import (
	"fmt"
	"time"

	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/kafka/configkafka"
)

// Config defines configuration for docker observer
type Config struct {
	configkafka.ClientConfig `mapstructure:",squash"`
	TopicRegex               string        `mapstructure:"topic_regex"`
	TopicsSyncInterval       time.Duration `mapstructure:"topics_sync_interval"`
}

func (config *Config) Validate() (errs error) {
	if len(config.TopicRegex) == 0 {
		errs = multierr.Append(errs, fmt.Errorf("topic_regex must be specified"))
	}
	if config.TopicsSyncInterval <= 0 {
		errs = multierr.Append(errs, fmt.Errorf("topics_sync_interval must be greater than 0"))
	}
	return errs
}
