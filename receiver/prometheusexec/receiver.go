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

	receiverConfig, subprocessConfig, ok := getPrometheusConfig(wrapper.config, wrapper.logger)
	if ok != nil {
		return fmt.Errorf("unable to generate the prometheusexec receiver config: %v", ok)
	}

	receiver, ok := factory.CreateMetricsReceiver(wrapper.logger, receiverConfig, wrapper.consumer)
	if ok != nil {
		return fmt.Errorf("unable to create Prometheus receiver: %v", ok)
	}

	wrapper.prometheusReceiver = receiver

	// Start the process with the built config
	subprocessConfig.Receiver = receiver
	subprocessConfig.Context = ctx
	subprocessConfig.Host = host
	go subprocessmanager.StartProcess(subprocessConfig)

	return wrapper.prometheusReceiver.Start(ctx, host)
}

// getPrometheusConfig returns the config after the correct logic is made
// All the scrape/subprocess configs are looped over and the proper attributes are assigned into the struct
func getPrometheusConfig(cfg *Config, logger *zap.Logger) (*prometheusreceiver.Config, *subprocessmanager.Process, error) {
	if cfg.SubprocessConfig.CommandString == "" {
		return nil, nil, fmt.Errorf("no command to execute entered in config file for %v", cfg.Name())
	}

	scrapeConfig := &config.ScrapeConfig{}
	subprocessConfig := &subprocessmanager.Process{}

	scrapeConfig.ScrapeInterval = model.Duration(cfg.ScrapeInterval)
	scrapeConfig.ScrapeTimeout = model.Duration(cfg.ScrapeInterval)
	scrapeConfig.Scheme = "http"
	scrapeConfig.MetricsPath = defaultMetricsPath

	// This is a default Prometheus scrape config value, which indicates that the scraped metrics can be modified
	scrapeConfig.HonorLabels = false
	// This is a default Prometheus scrape config value, which indicates that timestamps of the scrape should be respected
	scrapeConfig.HonorTimestamps = true

	// Try to get a custom name from the config (receivers should be named prometheus_exec/customName)
	splitName := strings.Split(cfg.Name(), "/")
	customName := strings.TrimSpace(splitName[len(splitName)-1])
	if customName == "" || len(splitName) < 2 {
		// If there is no customName, try to simply generate one by using the first word in the exec string, assuming it's the binary (i.e. ./mysqld_exporter ...)
		defaultName := strings.Split(cfg.SubprocessConfig.CommandString, " ")[0]
		scrapeConfig.JobName = defaultName
		subprocessConfig.CustomName = defaultName
	} else {
		scrapeConfig.JobName = customName
		subprocessConfig.CustomName = customName
	}

	// Set the proper target
	scrapeConfig.ServiceDiscoveryConfig = sdconfig.ServiceDiscoveryConfig{
		StaticConfigs: []*targetgroup.Group{
			{
				Targets: []model.LabelSet{
					{model.AddressLabel: model.LabelValue(fmt.Sprintf("localhost:%v", cfg.SubprocessConfig.Port))},
				},
			},
		},
	}

	subprocessConfig.Command = cfg.SubprocessConfig.CommandString
	subprocessConfig.Port = cfg.SubprocessConfig.Port
	subprocessConfig.Env = cfg.SubprocessConfig.Env

	return &prometheusreceiver.Config{
		PrometheusConfig: &config.Config{
			ScrapeConfigs: []*config.ScrapeConfig{scrapeConfig},
		},
	}, subprocessConfig, nil
}

// Shutdown stops the underlying Prometheus receiver.
func (wrapper *prometheusReceiverWrapper) Shutdown(ctx context.Context) error {
	return wrapper.prometheusReceiver.Shutdown(ctx)
}
