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

package prometheusexecreceiver

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/config"
	sdconfig "github.com/prometheus/prometheus/discovery/config"
	"github.com/prometheus/prometheus/discovery/targetgroup"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver/prometheusreceiver"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusexecreceiver/subprocessmanager"
)

const (
	minPortRange int    = 10000      // Minimum of the port generation range
	maxPortRange int    = 11000      // Maximum of the port generation range
	portTemplate string = "{{port}}" // Template for port in strings
)

// Local random seed to not override anything being used globally
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

// Start creates and calls the function that handles the prometheus_exec receiver
func (wrapper *prometheusReceiverWrapper) Start(ctx context.Context, host component.Host) error {
	factory := &prometheusreceiver.Factory{}

	customName := getCustomName(wrapper.config)

	subprocessConfig, ok := getSubprocessConfig(wrapper.config, customName)
	if ok != nil {
		return fmt.Errorf("unable to generate the subprocess config: %w", ok)
	}

	receiverConfig := getReceiverConfig(wrapper.config, customName)

	receiver, ok := factory.CreateMetricsReceiver(wrapper.context, wrapper.logger, receiverConfig, wrapper.consumer)
	if ok != nil {
		return fmt.Errorf("unable to create Prometheus receiver: %w", ok)
	}

	wrapper.subprocessConfig = subprocessConfig
	wrapper.receiverConfig = receiverConfig
	wrapper.prometheusReceiver = receiver
	wrapper.context = ctx
	wrapper.host = host

	// Start the process with the built config
	go wrapper.manageProcess()

	return nil
}

// getReceiverConfig returns the Prometheus receiver config
func getReceiverConfig(cfg *Config, customName string) *prometheusreceiver.Config {
	scrapeConfig := &config.ScrapeConfig{}

	scrapeConfig.ScrapeInterval = model.Duration(cfg.ScrapeInterval)
	scrapeConfig.ScrapeTimeout = model.Duration(defaultScrapeTimeout)
	scrapeConfig.Scheme = "http"
	scrapeConfig.MetricsPath = defaultMetricsPath
	scrapeConfig.JobName = customName

	// This is a default Prometheus scrape config value, which indicates that the scraped metrics can be modified
	scrapeConfig.HonorLabels = false
	// This is a default Prometheus scrape config value, which indicates that timestamps of the scrape should be respected
	scrapeConfig.HonorTimestamps = true

	// Set the proper target by creating one target inside a single target group (this is how Prometheus wants its scrape config)
	scrapeConfig.ServiceDiscoveryConfig = sdconfig.ServiceDiscoveryConfig{
		StaticConfigs: []*targetgroup.Group{
			{
				Targets: []model.LabelSet{
					{model.AddressLabel: model.LabelValue(fmt.Sprintf("localhost:%v", cfg.SubprocessConfig.Port))},
				},
			},
		},
	}

	return &prometheusreceiver.Config{
		PrometheusConfig: &config.Config{
			ScrapeConfigs: []*config.ScrapeConfig{scrapeConfig},
		},
	}
}

// getSubprocessConfig returns the subprocess config after the correct logic is made
func getSubprocessConfig(cfg *Config, customName string) (*subprocessmanager.Process, error) {
	if cfg.SubprocessConfig.Command == "" {
		return nil, fmt.Errorf("no command to execute entered in config file for %v", cfg.Name())
	}

	subprocessConfig := &subprocessmanager.Process{}

	subprocessConfig.Command = cfg.SubprocessConfig.Command
	subprocessConfig.Port = cfg.SubprocessConfig.Port
	subprocessConfig.Env = cfg.SubprocessConfig.Env

	return subprocessConfig, nil
}

// getCustomName will return the receiver's given custom name or try to generate one if none was given
func getCustomName(cfg *Config) string {
	// Try to get a custom name from the config (receivers should be named prometheus_exec/customName)
	splitName := strings.SplitN(cfg.Name(), "/", 2)
	if len(splitName) > 1 && splitName[1] != "" {
		return splitName[1]
	}
	// fall back to the first part of the string, prometheus_exec, which should only happen if there is a single prometheus_exec receiver configured
	return splitName[0]
}

// manageProcess will put the process in an infinite starting loop which goesd through the following steps
// If the port is not defined by the user, one is generated and a new metrics receiver is built with the new port
// All instances of {{port}} are replaced with the actual port (either defined by the user or generated)
// Start the receiver if it is stopped (either first iteration or if it was previously shutdown)
// We then start the subprocess, get its runtime and decide if the process is considered healthy or not -> the receiver is shutdown if it was deemed unhealthy
// Finally, the wait time before the subprocess is restarted is computed and this goroutine sleeps for that amount of time, before restarting the loop from the start
func (wrapper *prometheusReceiverWrapper) manageProcess() {
	var (
		elapsed       time.Duration
		newPort       int
		crashCount    int
		err           error
		subprocessErr error
	)

	for {

		// Generate a port if none was specified and if process is unhealthy (Receiver has been stopped)
		generatePort := wrapper.subprocessConfig.Port == 0 && elapsed <= subprocessmanager.HealthyProcessTime
		if generatePort {
			newPort, err = wrapper.assignNewRandomPort(newPort)
			if err != nil {
				wrapper.logger.Info("assignNewRandomPort() error - killing this single process/receiver", zap.String("error", err.Error()))
				return
			}
		}

		// Replace the templating in the strings of the process data
		wrapper.subprocessConfig = wrapper.fillPortPlaceholders(newPort)

		// Start the receiver if it's the first pass, or if the process is unhealthy/a newport was generated meaning the Receiver was stopped last pass
		firstRun := elapsed == 0
		unhealthyProcess := elapsed <= subprocessmanager.HealthyProcessTime && crashCount > subprocessmanager.HealthyCrashCount

		if firstRun || unhealthyProcess || generatePort {
			err = wrapper.prometheusReceiver.Start(wrapper.context, wrapper.host)
			if err != nil {
				wrapper.logger.Info("Start() error, could not start receiver - killing this single process/receiver", zap.String("error", err.Error()))
				return
			}
		}

		// Start the process, error is handled a little later
		elapsed, subprocessErr = wrapper.subprocessConfig.Run(wrapper.logger)

		// Compute the crashCount depending on subprocess health
		crashCount, err = wrapper.computeHealthAndCrashCount(elapsed, crashCount)
		if err != nil {
			wrapper.logger.Info("computeHealthAndCrashCount() error - killing this single process/receiver", zap.String("error", err.Error()))
			return
		}

		// Compute how long this process will wait before restarting
		sleepTime := subprocessmanager.GetDelay(elapsed, crashCount)

		// Now we can log the process error, no need to escalate the error, simply log and restart the subprocess
		if subprocessErr != nil {
			wrapper.logger.Info("Subprocess error", zap.String("time until process restarts", sleepTime.String()), zap.String("error", subprocessErr.Error()))
		}

		// Sleep this goroutine for a certain amount of time, computed by exponential backoff
		time.Sleep(sleepTime)
	}
}

// computeHealthAndCrashCount will decide whether the process is healthy, set the crashCount accordingly and shutdown the receiver if unhealthy
func (wrapper *prometheusReceiverWrapper) computeHealthAndCrashCount(elapsed time.Duration, crashCount int) (int, error) {
	// Reset crash count to 1 if the process seems to be healthy now, else increase crashCount
	if elapsed > subprocessmanager.HealthyProcessTime {
		return 1, nil
	}
	crashCount++

	// Stop the associated receiver until process starts back up again since it is unhealthy (too little elapsed time and high crashCount) or if port is generated, to allow for new port
	if wrapper.subprocessConfig.Port == 0 || crashCount > subprocessmanager.HealthyCrashCount {
		err := wrapper.Shutdown(wrapper.context)
		if err != nil {
			return crashCount, fmt.Errorf("could not stop receiver associated to process, killing it")
		}
	}

	return crashCount, nil
}

// assignNewRandomPort generates a new port, creates a new metrics receiver with that port and assigns it to the wrapper
func (wrapper *prometheusReceiverWrapper) assignNewRandomPort(oldPort int) (int, error) {
	var err error
	newPort := generateRandomPort(oldPort)

	// Assign the new port in the config
	wrapper.receiverConfig.PrometheusConfig.ScrapeConfigs[0].ServiceDiscoveryConfig.StaticConfigs[0].Targets = []model.LabelSet{
		{model.AddressLabel: model.LabelValue(fmt.Sprintf("localhost:%v", newPort))},
	}

	// Create new Prometheus receiver with new config and replace pointer to it in wrapper
	factory := &prometheusreceiver.Factory{}
	wrapper.prometheusReceiver, err = factory.CreateMetricsReceiver(wrapper.context, wrapper.logger, wrapper.receiverConfig, wrapper.consumer)
	if err != nil {
		return 0, fmt.Errorf("unable to create Prometheus receiver. Killing this process")
	}

	return newPort, nil
}

// fillPortPlaceholders will check if any of the strings in the process data have the {{port}} placeholder, and replace it if necessary
func (wrapper *prometheusReceiverWrapper) fillPortPlaceholders(newPort int) *subprocessmanager.Process {
	var port string
	if wrapper.subprocessConfig.Port == 0 {
		port = strconv.Itoa(newPort)
	} else {
		port = strconv.Itoa(wrapper.subprocessConfig.Port)
	}

	newConfig := *wrapper.subprocessConfig
	// ReplaceAll runs much faster (about 5x according to my tests) than checking for a regex match, therefore no checks are made and ReplaceAll simply returns the original string if no match is found
	// ReplaceAll is run on the original strings of the config, which remain unchanged
	newConfig.Command = strings.ReplaceAll(wrapper.config.SubprocessConfig.Command, portTemplate, port)

	for i, env := range wrapper.config.SubprocessConfig.Env {
		newConfig.Env[i].Value = strings.ReplaceAll(env.Value, portTemplate, port)
	}

	return &newConfig
}

// generateRandomPort will try to generate a random port until it is different than the last one generated
func generateRandomPort(lastPort int) int {
	var newPort int
	for {
		newPort = random.Intn(maxPortRange-minPortRange) + minPortRange

		// Generate another port if it's the same
		if newPort == lastPort {
			continue
		}

		// Test the port to see if it's available
		portTest, err := net.Listen("tcp", fmt.Sprintf(":%v", newPort))
		if err != nil {
			continue
		}

		portTest.Close()
		break
	}
	return newPort
}

// Shutdown stops the underlying Prometheus receiver.
func (wrapper *prometheusReceiverWrapper) Shutdown(ctx context.Context) error {
	return wrapper.prometheusReceiver.Shutdown(ctx)
}
