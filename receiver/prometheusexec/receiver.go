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
	"errors"
	"fmt"
	"strings"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusexec/subprocessmanager"
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

	config, ok := getPrometheusConfig(wrapper.config, wrapper.logger)
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

// getPrometheusConfig returns the config after the correct logic is made
// All the scrape/subprocess configs are looped over and the proper attributes are assigned into the struct
func getPrometheusConfig(cfg *Config, logger *zap.Logger) (*prometheusreceiver.Config, error) {
	scrapeConfigs := []*config.ScrapeConfig{}
	subprocessConfigs := []*subprocessmanager.Process{}

	// Loop over the configurations and create the appropriate entries in the above slices
	for i, configuration := range cfg.ScrapeConfigs {
		// If there is no command to execute, throw error since this is a required field
		if configuration.SubprocessConfig.CommandString == "" {
			return nil, errors.New("no command to execute entered in config file")
		}

		scrapeConfig := &config.ScrapeConfig{}
		subprocessConfig := &subprocessmanager.Process{}

		scrapeConfig.ScrapeInterval = model.Duration(configuration.ScrapeInterval)
		scrapeConfig.ScrapeTimeout = model.Duration(configuration.ScrapeInterval)
		scrapeConfig.HonorTimestamps = true
		scrapeConfig.Scheme = "http"
		scrapeConfig.MetricsPath = defaultMetricsPath

		if configuration.SubprocessConfig.CustomName == "" {
			// If there is no customName, try to simply generate one by using the first word in the exec string, assuming it's the binary (i.e. ./mysqld_exporter ...)
			defaultName := strings.Split(configuration.SubprocessConfig.CommandString, " ")[0]
			scrapeConfig.JobName = defaultName
			subprocessConfig.CustomName = defaultName
		} else {
			scrapeConfig.JobName = configuration.SubprocessConfig.CustomName
			subprocessConfig.CustomName = configuration.SubprocessConfig.CustomName
		}

		// Set the proper target
		scrapeConfig.ServiceDiscoveryConfig = sdconfig.ServiceDiscoveryConfig{
			StaticConfigs: []*targetgroup.Group{
				{
					Targets: []model.LabelSet{
						{model.AddressLabel: model.LabelValue(fmt.Sprintf("localhost:%v", configuration.SubprocessConfig.Port))},
					},
				},
			},
		}

		// Append the resulting scrapeConfig into the aforedeclared slice of scrapeConfigs
		scrapeConfigs = append(scrapeConfigs, scrapeConfig)

		subprocessConfig.Command = configuration.SubprocessConfig.CommandString
		subprocessConfig.Port = configuration.SubprocessConfig.Port
		subprocessConfig.Index = i

		// Append the resulting subprocessConfig to the aforedeclared slice of subprocessConfigs
		subprocessConfigs = append(subprocessConfigs, subprocessConfig)
	}

	subprocessmanager.DistributeProcesses(subprocessConfigs)

	return &prometheusreceiver.Config{
		PrometheusConfig: &config.Config{
			ScrapeConfigs: scrapeConfigs,
		},
	}, nil
}

// Shutdown stops the underlying Prometheus receiver.
func (wrapper *prometheusReceiverWrapper) Shutdown(ctx context.Context) error {
	return wrapper.prometheusReceiver.Shutdown(ctx)
}
