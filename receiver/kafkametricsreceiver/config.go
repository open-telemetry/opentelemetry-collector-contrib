// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkametricsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkametricsreceiver"

import (
	"go.opentelemetry.io/collector/receiver/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/kafka"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkametricsreceiver/internal/metadata"
)

// Config represents user settings for kafkametrics receiver
type Config struct {
	scraperhelper.ScraperControllerSettings `mapstructure:",squash"`

	// The list of kafka brokers (default localhost:9092)
	Brokers []string `mapstructure:"brokers"`

	// ResolveCanonicalBootstrapServersOnly makes Sarama do a DNS lookup for
	// each of the provided brokers. It will then do a PTR lookup for each
	// returned IP, and that set of names becomes the broker list. This can be
	// required in SASL environments.
	ResolveCanonicalBootstrapServersOnly bool `mapstructure:"resolve_canonical_bootstrap_servers_only"`

	// ProtocolVersion Kafka protocol version
	ProtocolVersion string `mapstructure:"protocol_version"`

	// TopicMatch topics to collect metrics on
	TopicMatch string `mapstructure:"topic_match"`

	// GroupMatch consumer groups to collect on
	GroupMatch string `mapstructure:"group_match"`

	// Authentication data
	Authentication kafka.Authentication `mapstructure:"auth"`

	// Scrapers defines which metric data points to be captured from kafka
	Scrapers []string `mapstructure:"scrapers"`

	// ClientID is the id associated with the consumer that reads from topics in kafka.
	ClientID string `mapstructure:"client_id"`

	// MetricsBuilderConfig allows customizing scraped metrics/attributes representation.
	metadata.MetricsBuilderConfig `mapstructure:",squash"`
}
