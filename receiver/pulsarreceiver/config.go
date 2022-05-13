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

package pulsarreceiver

import (
	"errors"

	"github.com/apache/pulsar-client-go/pulsar"
	"go.opentelemetry.io/collector/config"
)

type Config struct {
	config.ReceiverSettings `mapstructure:",squash"`
	// Configure the service URL for the Pulsar service.
	ServiceUrl string `mapstructure:"service_url"`
	// The topic of pulsar to consume logs,metrics,traces. (default = "otlp_traces" for traces,
	//"otlp_metrics" for metrics, "otlp_logs" for logs)
	Topic string `mapstructure:"topic"`
	// The Subscription that receiver will be consuming messages from (default "otlp_subscription")
	Subscription string `mapstructure:"subscription"`
	// Encoding of the messages (default "otlp_proto")
	Encoding string `mapstructure:"encoding"`
	// Name specifies the consumer name.
	ConsumerName string `mapstructure:"consumer_name"`
	// Set the path to the trusted TLS certificate file
	TLSTrustCertsFilePath string `mapstructure:"tls_trust_certs_file_path"`
	// Configure whether the Pulsar client accept untrusted TLS certificate from broker (default: false)
	Insecure bool `mapstructure:"insecure"`
	//AuthName to create an authentication
	AuthName string `mapstructure:"auth_name"`
	//AuthParam to create an authentication
	AuthParam string `mapstructure:"auth_param"`
}

var _ config.Receiver = (*Config)(nil)

// Validate checks the receiver configuration is valid
func (cfg *Config) Validate() error {
	return nil
}

func (cfg *Config) clientOptions() (pulsar.ClientOptions, error) {
	url := cfg.ServiceUrl
	if len(url) <= 0 {
		url = defaultServiceUrl
	}
	options := pulsar.ClientOptions{
		URL: url,
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

func (cfg *Config) consumerOptions() (pulsar.ConsumerOptions, error) {
	options := pulsar.ConsumerOptions{
		Type:             pulsar.Failover,
		Topic:            cfg.Topic,
		SubscriptionName: cfg.Subscription,
	}

	if len(cfg.ConsumerName) > 0 {
		options.Name = cfg.ConsumerName
	}

	if options.SubscriptionName == "" || options.Topic == "" {
		return options, errors.New("topic and subscription is required")
	}

	return options, nil
}
