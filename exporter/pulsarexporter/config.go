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

	"github.com/apache/pulsar-client-go/pulsar"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

// Config defines configuration for Pulsar exporter.
type Config struct {
	config.ExporterSettings        `mapstructure:",squash"`
	exporterhelper.TimeoutSettings `mapstructure:",squash"`
	exporterhelper.QueueSettings   `mapstructure:"sending_queue"`
	exporterhelper.RetrySettings   `mapstructure:"retry_on_failure"`

	ServiceUrl string `mapstructure:"service_url"`
	Topic      string `mapstructure:"topic"`

	EnableBatch bool `mapstructure:"enable_batch"`
	// Encoding of messages (default "otlp_proto")
	Encoding              string `mapstructure:"encoding"`
	TLSTrustCertsFilePath string `mapstructure:"tls_trust_certs_file_path"`
	Insecure              bool   `mapstructure:"insecure"`
	AuthName              string `mapstructure:"auth_name"`
	AuthParam             string `mapstructure:"auth_param"`
}

var _ config.Exporter = (*Config)(nil)

// Validate checks if the exporter configuration is valid
func (cfg *Config) Validate() error {

	return nil
}

func (cfg *Config) ClientOptions() (pulsar.ClientOptions, error) {
	duration, _ := time.ParseDuration("20s")
	options := pulsar.ClientOptions{
		URL:               cfg.ServiceUrl,
		ConnectionTimeout: duration,
		OperationTimeout:  duration,
	}

	options.TLSAllowInsecureConnection = cfg.Insecure
	if len(cfg.TLSTrustCertsFilePath) > 0 {
		options.TLSTrustCertsFilePath = cfg.TLSTrustCertsFilePath
	}

	if len(cfg.AuthName) > 0 && len(cfg.AuthParam) > 0 {
		auth, err := pulsar.NewAuthentication(cfg.AuthName, cfg.AuthParam)
		if err != nil {
			return pulsar.ClientOptions{}, err
		}
		options.Authentication = auth
	}

	return options, nil
}
