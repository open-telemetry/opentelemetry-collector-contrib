// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkametricsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkametricsreceiver"

import (
	"time"

	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/kafka/configkafka"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkametricsreceiver/internal/metadata"
)

// Config represents user settings for kafkametrics receiver
type Config struct {
	scraperhelper.ControllerConfig `mapstructure:",squash"`

	// Alias name of the kafka cluster
	ClusterAlias string `mapstructure:"cluster_alias"`

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
	Authentication configkafka.AuthenticationConfig `mapstructure:"auth"`

	// Cluster metadata refresh frequency
	// Configures the refresh frequency to update cached cluster metadata
	// Defaults to 10 minutes from Sarama library
	RefreshFrequency time.Duration `mapstructure:"refresh_frequency"`

	// Scrapers defines which metric data points to be captured from kafka
	Scrapers []string `mapstructure:"scrapers"`

	// ClientID is the id associated with the consumer that reads from topics in kafka.
	ClientID string `mapstructure:"client_id"`

	// MetricsBuilderConfig allows customizing scraped metrics/attributes representation.
	metadata.MetricsBuilderConfig `mapstructure:",squash"`
}
