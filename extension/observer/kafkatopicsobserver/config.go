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
	// The list of kafka brokers (default localhost:9092)
	Brokers []string `mapstructure:"brokers"`
	// ResolveCanonicalBootstrapServersOnly makes Sarama do a DNS lookup for
	// each of the provided brokers. It will then do a PTR lookup for each
	// returned IP, and that set of names becomes the broker list. This can be
	// required in SASL environments.
	ResolveCanonicalBootstrapServersOnly bool `mapstructure:"resolve_canonical_bootstrap_servers_only"`
	// Kafka protocol version
	ProtocolVersion    string                           `mapstructure:"protocol_version"`
	Authentication     configkafka.AuthenticationConfig `mapstructure:"auth"`
	TopicRegex         string                           `mapstructure:"topic_regex"`
	TopicsSyncInterval time.Duration                    `mapstructure:"topics_sync_interval"`
}

func (config *Config) Validate() (errs error) {
	if len(config.Brokers) == 0 {
		errs = multierr.Append(errs, fmt.Errorf("brokers list must be specified"))
	}
	if len(config.ProtocolVersion) == 0 {
		errs = multierr.Append(errs, fmt.Errorf("protocol_version must be specified"))
	}
	if len(config.TopicRegex) == 0 {
		errs = multierr.Append(errs, fmt.Errorf("topic_regex must be specified"))
	}
	if config.TopicsSyncInterval <= 0 {
		errs = multierr.Append(errs, fmt.Errorf("topics_sync_interval must be greater than 0"))
	}
	return errs
}
