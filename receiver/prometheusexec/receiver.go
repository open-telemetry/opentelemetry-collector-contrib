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
	"math/rand"
	"regexp"
	"strconv"
	"strings"
	"time"

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

const (
	minPortRange   int    = 10000      // Minimum of the port generation range
	maxPortRange   int    = 10100      // Maximum of the port generation range
	stringTemplate string = "{{port}}" // Template for port in strings
)

// Local random seed
var random *rand.Rand = rand.New(rand.NewSource(time.Now().UnixNano()))

type prometheusReceiverWrapper struct {
	logger   *zap.Logger
	config   *Config
	consumer consumer.MetricsConsumerOld
	// Prometheus receiver config
	receiverConfig *prometheusreceiver.Config
	// Subprocess data
	subprocessConfig *subprocessmanager.Process
	// Receiver data
	prometheusReceiver component.MetricsReceiver
	context            context.Context
	host               component.Host
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

	wrapper.receiverConfig = receiverConfig
	wrapper.prometheusReceiver = receiver
	wrapper.context = ctx
	wrapper.host = host

	// Start the process with the built config
	wrapper.subprocessConfig = subprocessConfig
	go wrapper.manageProcess()

	return nil
}

// manageProcess will put the process in an infinite starting loop
func (wrapper *prometheusReceiverWrapper) manageProcess() error {
	var (
		elapsed    time.Duration
		newPort    int
		crashCount int
		err        error
	)

	for true {
		// Generate a port if none was specified, and if process is unhealthy (Receiver has been stopped)
		if wrapper.subprocessConfig.Port == 0 && elapsed <= subprocessmanager.HealthyProcessTime {
			newPort = generateRandomPort(newPort)

			// TODO: refactor
			// Assign the new port in the config
			wrapper.receiverConfig.PrometheusConfig.ScrapeConfigs[0].ServiceDiscoveryConfig = sdconfig.ServiceDiscoveryConfig{
				StaticConfigs: []*targetgroup.Group{
					{
						Targets: []model.LabelSet{
							{model.AddressLabel: model.LabelValue(fmt.Sprintf("localhost:%v", newPort))},
						},
					},
				},
			}

			// Create new Prometheus receiver with new config and keep pointer to it in wrapper
			factory := &prometheusreceiver.Factory{}
			wrapper.prometheusReceiver, err = factory.CreateMetricsReceiver(wrapper.logger, wrapper.receiverConfig, wrapper.consumer)
			if err != nil {
				return fmt.Errorf("unable to create Prometheus receiver: %v. Killing this process (%v)", err, wrapper.subprocessConfig.CustomName)
			}
		}

		// Replace the templating in the strings of the process data
		wrapper.stringTemplating(newPort)

		// Start the receiver if it's the first pass, or if the process in unhealthy and Receiver was stopped
		if elapsed <= subprocessmanager.HealthyProcessTime {
			err := wrapper.prometheusReceiver.Start(wrapper.context, wrapper.host)
			if err != nil {
				return fmt.Errorf("could not start receiver associated to %v process, killing this single process (%v)", wrapper.subprocessConfig.CustomName, wrapper.subprocessConfig.CustomName)
			}
		}

		elapsed, err = subprocessmanager.StartProcess(wrapper.subprocessConfig)
		if err != nil {
			return err // ??
		}

		// Reset crash count to 1 if the process seems to be healthy now, else increase crashCount
		if elapsed > subprocessmanager.HealthyProcessTime {
			crashCount = 1
		} else {
			crashCount++

			// Stop the associated receiver until process starts back up again since it is unhealthy
			err := wrapper.prometheusReceiver.Shutdown(wrapper.context)
			if err != nil {
				return fmt.Errorf("could not stop receiver associated to %v process, killing this single process(%v)", wrapper.subprocessConfig.CustomName, wrapper.subprocessConfig.CustomName)
			}
		}

		// Sleep this goroutine for a certain amount of time, computed by exponential backoff
		time.Sleep(subprocessmanager.GetDelay(elapsed, crashCount))
	}

	return nil
}

// stringTemplating will check if any of the strings in the process data have a certain templating, and replace it if necessary
func (wrapper *prometheusReceiverWrapper) stringTemplating(newPort int) {
	var port string
	if wrapper.subprocessConfig.Port == 0 {
		port = strconv.Itoa(newPort)
	} else {
		port = strconv.Itoa(wrapper.subprocessConfig.Port)
	}

	r, _ := regexp.Compile(stringTemplate)

	if r.MatchString(wrapper.config.SubprocessConfig.CommandString) {
		wrapper.subprocessConfig.Command = strings.ReplaceAll(wrapper.config.SubprocessConfig.CommandString, stringTemplate, port)
	}

	for i, env := range wrapper.config.SubprocessConfig.Env {
		if r.MatchString(env.Value) {
			wrapper.subprocessConfig.Env[i].Value = strings.ReplaceAll(env.Value, stringTemplate, port)
		}
	}
}

// generateRandomPort will try to generate a random port until it is different than the last one generated
func generateRandomPort(lastPort int) int {
	for true {
		newPort := random.Intn(maxPortRange-minPortRange) + minPortRange
		if newPort != lastPort {
			return newPort
		}
	}
	return 0
}

// TODO: refactor
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
	splitName := strings.SplitN(cfg.Name(), "/", 2)
	customName := strings.TrimSpace(splitName[1])
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
