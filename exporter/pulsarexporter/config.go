// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package pulsarexporter

import (
	"time"

	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

// Config defines configuration for Pulsar exporter.
type Config struct {
	config.ExporterSettings        `mapstructure:"-"`
	exporterhelper.TimeoutSettings `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct.
	exporterhelper.QueueSettings   `mapstructure:"sending_queue"`
	exporterhelper.RetrySettings   `mapstructure:"retry_on_failure"`

	// client config
	// URL is client The url of pulsar cluster (default pulsar://localhost:6650)
	URL string `mapstructure:"url"`
	// ConnectionTimeout is the client connection timeout
	ConnectionTimeout time.Duration `mapstructure:"connection_timeout"`
	// OperationTimeout is the client operation timeout
	OperationTimeout time.Duration `mapstructure:"operation_timeout"`
	// Authentication defines used authentication mechanism.
	Authentication Authentication `mapstructure:"auth"`
	// MaxConnectionsPerBroker is max connections num connect to broker
	MaxConnectionsPerBroker int `mapstructure:"max_connections_per_broker"`

	// producer config
	// Topic The name of the pulsar topic to export to (default otlp_traces for traces, otlp_metrics for metrics,
	// otlp_logs for logs)
	Topic string `mapstructure:"topic"`
	// Encoding of messages (default "otlp_proto")
	Encoding string `mapstructure:"encoding"`
	// SendTimeout is producer timeout
	SendTimeout time.Duration `mapstructure:"send_timeout"`
	// MaxPendingMessages is max pending messages
	MaxPendingMessages int `mapstructure:"max_pending_messages"`
	// CompressionType compress type
	CompressionType int `mapstructure:"compression_type"`
	// CompressionType compress level
	CompressionLevel int `mapstructure:"compression_level"`
	// BatchingMaxPublishDelay batching max publish delay duration
	BatchingMaxPublishDelay time.Duration `mapstructure:"batching_max_publish_delay"`
	// BatchingMaxMessages batching max messages
	BatchingMaxMessages uint `mapstructure:"batching_max_messages"`
	// BatchingMaxSize batching max size bytes
	BatchingMaxSize uint `mapstructure:"batching_max_size"`
	// AttributeAsSendKey attribute name as producer send key
	AttributeAsSendKey string `mapstructure:"attribute_as_send_key"`
}

// Authentication defines used authentication mechanism.
type Authentication struct {
	// Name is pulsar auth name, ref: pulsar.NewAuthentication(name string, params string)
	Name string `mapstructure:"name"`
	// Name is pulsar auth params, ref: pulsar.NewAuthentication(name string, params string). params is map json string.
	Params map[string]string `mapstructure:"params"`
}

var _ config.Exporter = (*Config)(nil)

// Validate checks if the exporter configuration is valid
func (cfg *Config) Validate() error {
	return nil
}
