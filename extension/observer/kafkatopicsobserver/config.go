// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkatopicsobserver // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/kafkatopicsobserver"

import (
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/kafka"

	"go.opentelemetry.io/collector/confmap"
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
	ProtocolVersion string `mapstructure:"protocol_version"`
	// Session interval for the Kafka consumer
	SessionTimeout time.Duration `mapstructure:"session_timeout"`
	// Heartbeat interval for the Kafka consumer
	HeartbeatInterval time.Duration        `mapstructure:"heartbeat_interval"`
	Authentication    kafka.Authentication `mapstructure:"auth"`
	TopicRegex        string               `mapstructure:"topic_regex"`
}

func (config Config) Validate() error {
	return nil
}

func (config *Config) Unmarshal(conf *confmap.Conf) error {
	err := conf.Unmarshal(config)
	if err != nil {
		return err
	}

	return err
}
