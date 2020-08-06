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
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver/prometheusreceiver"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusexecreceiver/subprocessmanager"
)

const (
	// template for port in strings
	portTemplate string = "{{port}}"
	// healthyProcessTime is the default time a process needs to stay alive to be considered healthy
	healthyProcessTime time.Duration = 30 * time.Minute
	// healthyCrashCount is the amount of times a process can crash (within the healthyProcessTime) before being considered unstable - it may be trying to find a port
	healthyCrashCount int = 3
)

var random *rand.Rand = rand.New(rand.NewSource(time.Now().UnixNano()))

type prometheusExecReceiver struct {
	logger   *zap.Logger
	config   *Config
	consumer consumer.MetricsConsumerOld

	// Prometheus receiver config
	receiverConfig *prometheusreceiver.Config

	// Subprocess data
	subprocessConfig *subprocessmanager.SubprocessConfig

	// Receiver data
	originalPort       int
	prometheusReceiver component.MetricsReceiver
}

// new returns a prometheusExecReceiver
func new(logger *zap.Logger, config *Config, consumer consumer.MetricsConsumerOld) *prometheusExecReceiver {
	return &prometheusExecReceiver{logger: logger, config: config, consumer: consumer}
}

// Start creates the configs and calls the function that handles the prometheus_exec receiver
func (wrapper *prometheusExecReceiver) Start(ctx context.Context, host component.Host) error {
	factory := &prometheusreceiver.Factory{}

	subprocessConfig, ok := getSubprocessConfig(wrapper.config)
	if ok != nil {
		return fmt.Errorf("unable to generate the subprocess config: %w", ok)
	}

	receiverConfig := getReceiverConfig(wrapper.config)

	receiver, ok := factory.CreateMetricsReceiver(ctx, wrapper.logger, receiverConfig, wrapper.consumer)
	if ok != nil {
		return fmt.Errorf("unable to create Prometheus receiver: %w", ok)
	}

	wrapper.originalPort = wrapper.config.Port
	wrapper.subprocessConfig = subprocessConfig
	wrapper.receiverConfig = receiverConfig
	wrapper.prometheusReceiver = receiver

	// Start the process with the built config
	go wrapper.manageProcess(ctx, host)

	return nil
}

// getReceiverConfig returns the Prometheus receiver config
func getReceiverConfig(cfg *Config) *prometheusreceiver.Config {
	scrapeConfig := &config.ScrapeConfig{}

	scrapeConfig.ScrapeInterval = model.Duration(cfg.ScrapeInterval)
	scrapeConfig.ScrapeTimeout = model.Duration(defaultScrapeTimeout)
	scrapeConfig.Scheme = "http"
	scrapeConfig.MetricsPath = defaultMetricsPath
	scrapeConfig.JobName = extractName(cfg)
	scrapeConfig.HonorLabels = false
	scrapeConfig.HonorTimestamps = true

	// Set the proper target by creating one target inside a single target group (this is how Prometheus wants its scrape config)
	scrapeConfig.ServiceDiscoveryConfig = sdconfig.ServiceDiscoveryConfig{
		StaticConfigs: []*targetgroup.Group{
			{
				Targets: []model.LabelSet{
					{model.AddressLabel: model.LabelValue(fmt.Sprintf("localhost:%v", cfg.Port))},
				},
			},
		},
	}

	receiverSettings := &configmodels.ReceiverSettings{}
	receiverSettings.SetType(typeStr)
	receiverSettings.SetName(cfg.Name())

	return &prometheusreceiver.Config{
		ReceiverSettings: *receiverSettings,
		PrometheusConfig: &config.Config{
			ScrapeConfigs: []*config.ScrapeConfig{scrapeConfig},
		},
	}
}

// getSubprocessConfig returns the subprocess config after the correct logic is made
func getSubprocessConfig(cfg *Config) (*subprocessmanager.SubprocessConfig, error) {
	if cfg.SubprocessConfig.Command == "" {
		return nil, fmt.Errorf("no command to execute entered in config file for %v", cfg.Name())
	}

	subprocessConfig := &subprocessmanager.SubprocessConfig{}

	subprocessConfig.Command = cfg.SubprocessConfig.Command
	subprocessConfig.Env = cfg.SubprocessConfig.Env

	return subprocessConfig, nil
}

// extractName will return the receiver's given custom name (prometheus_exec/custom_name)
func extractName(cfg *Config) string {
	splitName := strings.SplitN(cfg.Name(), "/", 2)
	if len(splitName) > 1 && splitName[1] != "" {
		return splitName[1]
	}
	// fall back to the first part of the string, prometheus_exec
	return splitName[0]
}

// manageProcess will put the process in an infinite starting loop which goes through the following steps
// If the port is not defined by the user, one is generated and a new metrics receiver is built with the new port
// All instances of {{port}} are replaced with the actual port (either defined by the user or generated)
// Start the receiver if it is stopped (either first iteration or if it was previously shutdown)
// We then start the subprocess, get its runtime and decide if the process is considered healthy or not -> the receiver is shutdown if it was deemed unhealthy
// Finally, the wait time before the subprocess is restarted is computed and this goroutine sleeps for that amount of time, before restarting the loop from the start
func (wrapper *prometheusExecReceiver) manageProcess(ctx context.Context, host component.Host) {
	var (
		newPort int = wrapper.originalPort

		elapsed       time.Duration
		crashCount    int
		err           error
		subprocessErr error
	)

	for {

		// Generate a port if none was specified and if process had a low runtime (Receiver was stopped)
		generatePort := wrapper.originalPort == 0 && elapsed <= healthyProcessTime
		if generatePort {
			newPort, err = wrapper.assignNewRandomPort(ctx)
			if err != nil {
				wrapper.logger.Error("assignNewRandomPort() error - killing this single process/receiver", zap.String("error", err.Error()))
				return
			}
		}

		wrapper.subprocessConfig = wrapper.fillPortPlaceholders(newPort)

		// Start the receiver if it's the first pass, or if the process is unhealthy/a newport was generated meaning the Receiver was stopped last pass
		firstRun := elapsed == 0
		unhealthyProcess := elapsed <= healthyProcessTime && crashCount > healthyCrashCount

		if firstRun || unhealthyProcess || generatePort {
			err = wrapper.prometheusReceiver.Start(ctx, host)
			if err != nil {
				wrapper.logger.Error("Start() error, could not start receiver - killing this single process/receiver", zap.String("error", err.Error()))
				return
			}
		}

		// Start the process, error is handled a little later
		elapsed, subprocessErr = wrapper.subprocessConfig.Run(wrapper.logger)

		crashCount, err = wrapper.computeHealthAndCrashCount(ctx, elapsed, crashCount)
		if err != nil {
			wrapper.logger.Error("computeHealthAndCrashCount() error - killing this single process/receiver", zap.String("error", err.Error()))
			return
		}

		sleepTime := subprocessmanager.GetDelay(elapsed, healthyProcessTime, crashCount, healthyCrashCount)

		// Now we can log the process exit error, no need to escalate the error
		if subprocessErr != nil {
			wrapper.logger.Error("Subprocess error", zap.String("time until process restarts", sleepTime.String()), zap.String("error", subprocessErr.Error()))
		}

		time.Sleep(sleepTime)
	}
}

// computeHealthAndCrashCount will decide whether the process is healthy, set the crashCount accordingly and shutdown the receiver if unhealthy
func (wrapper *prometheusExecReceiver) computeHealthAndCrashCount(ctx context.Context, elapsed time.Duration, crashCount int) (int, error) {
	if elapsed > healthyProcessTime {
		return 1, nil
	}
	crashCount++

	// Stop the associated receiver until process starts back up again since it is unhealthy (too little elapsed time and high crashCount)
	// or if port is generated, to allow for new port
	if wrapper.originalPort == 0 || crashCount > healthyCrashCount {
		err := wrapper.Shutdown(ctx)
		if err != nil {
			return crashCount, fmt.Errorf("could not stop receiver associated to process, killing it")
		}
	}

	return crashCount, nil
}

// assignNewRandomPort generates a new port, creates a new metrics receiver with that port and assigns it to the wrapper
func (wrapper *prometheusExecReceiver) assignNewRandomPort(ctx context.Context) (int, error) {
	var err error
	newPort, err := generateRandomPort()
	if err != nil {
		return 0, err
	}

	wrapper.receiverConfig.PrometheusConfig.ScrapeConfigs[0].ServiceDiscoveryConfig.StaticConfigs[0].Targets = []model.LabelSet{
		{model.AddressLabel: model.LabelValue(fmt.Sprintf("localhost:%v", newPort))},
	}

	factory := &prometheusreceiver.Factory{}
	wrapper.prometheusReceiver, err = factory.CreateMetricsReceiver(ctx, wrapper.logger, wrapper.receiverConfig, wrapper.consumer)
	if err != nil {
		return 0, fmt.Errorf("unable to create Prometheus receiver. Killing this process")
	}

	return newPort, nil
}

// fillPortPlaceholders will check if any of the strings in the process data have the {{port}} placeholder, and replace it if necessary
func (wrapper *prometheusExecReceiver) fillPortPlaceholders(newPort int) *subprocessmanager.SubprocessConfig {
	port := strconv.Itoa(newPort)

	newConfig := *wrapper.subprocessConfig

	newConfig.Command = strings.ReplaceAll(wrapper.config.SubprocessConfig.Command, portTemplate, port)

	for i, env := range wrapper.config.SubprocessConfig.Env {
		newConfig.Env[i].Value = strings.ReplaceAll(env.Value, portTemplate, port)
	}

	return &newConfig
}

// generateRandomPort will try to generate a random port until it is different than the last one generated
func generateRandomPort() (int, error) {
	findPort, err := net.Listen("tcp", ":0")
	if err != nil {
		return 0, fmt.Errorf("unable to generate a new port, error: %w", err)
	}

	newPortString := findPort.Addr().String()
	newPort, AtoiErr := strconv.Atoi(newPortString[strings.LastIndex(newPortString, ":")+1:])
	if AtoiErr != nil {
		return 0, fmt.Errorf("unable to parse string to int, error: %w", AtoiErr)
	}

	findPort.Close()

	return newPort, nil
}

// Shutdown stops the underlying Prometheus receiver.
func (wrapper *prometheusExecReceiver) Shutdown(ctx context.Context) error {
	return wrapper.prometheusReceiver.Shutdown(ctx)
}
