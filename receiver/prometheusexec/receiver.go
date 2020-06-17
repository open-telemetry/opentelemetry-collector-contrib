// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package prometheusexec

import (
	"context"
	"fmt"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/config"
	sdconfig "github.com/prometheus/prometheus/discovery/config"
	"github.com/prometheus/prometheus/discovery/targetgroup"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver/prometheusreceiver"
	"go.uber.org/zap"
)

type prometheusReceiverWrapper struct {
	logger             *zap.Logger
	config             *Config
	consumer           consumer.MetricsConsumerOld
	prometheusReceiver component.MetricsReceiver
}

// new returns a prometheusReceiverWrapper
func new(logger *zap.Logger, config *Config, consumer consumer.MetricsConsumerOld) *prometheusReceiverWrapper {
	return &prometheusReceiverWrapper{logger: logger, config: config, consumer: consumer}
}

// Start creates and starts the prometheus receiver
func (wrapper *prometheusReceiverWrapper) Start(ctx context.Context, host component.Host) error {
	factory := &prometheusreceiver.Factory{}

	config, ok := getPrometheusConfig(wrapper.config)
	if ok != nil {
		return fmt.Errorf("unable to generate the prometheusexec receiver config: %v", ok)
	}

	receiver, ok := factory.CreateMetricsReceiver(wrapper.logger, config, wrapper.consumer)
	if ok != nil {
		return fmt.Errorf("unable to create Prometheus receiver: %v", ok)
	}

	wrapper.prometheusReceiver = receiver
	return wrapper.prometheusReceiver.Start(ctx, host)
}

//getPrometheusConfig returns the config after the correct logic is made
// TODO: update with ACTUAL logic
func getPrometheusConfig(cfg *Config) (*prometheusreceiver.Config, error) {
	return &prometheusreceiver.Config{
		PrometheusConfig: &config.Config{
			ScrapeConfigs: []*config.ScrapeConfig{
				&config.ScrapeConfig{
					ScrapeInterval:  model.Duration(cfg.ScrapeConfigs[0].ScrapeInterval),
					ScrapeTimeout:   model.Duration(cfg.ScrapeConfigs[0].ScrapeInterval),
					JobName:         cfg.ScrapeConfigs[0].SubprocessConfig.CommandString,
					HonorTimestamps: true,
					ServiceDiscoveryConfig: sdconfig.ServiceDiscoveryConfig{
						StaticConfigs: []*targetgroup.Group{
							{
								Targets: []model.LabelSet{
									{model.AddressLabel: model.LabelValue(cfg.Endpoint)},
								},
							},
						},
					},
				},
			},
		},
	}, nil
}

// Shutdown stops the underlying Prometheus receiver.
func (wrapper *prometheusReceiverWrapper) Shutdown(ctx context.Context) error {
	return wrapper.prometheusReceiver.Shutdown(ctx)
}
