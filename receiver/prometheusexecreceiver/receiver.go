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

type prometheusExecReceiver struct {
	logger   *zap.Logger
	config   *Config
	consumer consumer.MetricsConsumerOld

	// Prometheus receiver config
	promReceiverConfig *prometheusreceiver.Config

	// Subprocess data
	subprocessConfig *subprocessmanager.SubprocessConfig
	originalPort     int

	// Underlying receiver data
	prometheusReceiver component.MetricsReceiver
}

// new returns a prometheusExecReceiver
func new(logger *zap.Logger, config *Config, consumer consumer.MetricsConsumerOld) *prometheusExecReceiver {
	subprocessConfig := getSubprocessConfig(config)
	promReceiverConfig := getPromReceiverConfig(config)

	return &prometheusExecReceiver{
		logger:             logger,
		config:             config,
		consumer:           consumer,
		subprocessConfig:   subprocessConfig,
		promReceiverConfig: promReceiverConfig,
		originalPort:       config.Port,
	}
}

// Start creates the configs and calls the function that handles the prometheus_exec receiver
func (per *prometheusExecReceiver) Start(ctx context.Context, host component.Host) error {
	if per.subprocessConfig.Command == "" {
		return fmt.Errorf("no command to execute entered in config file for %v", per.config.Name())
	}

	factory := &prometheusreceiver.Factory{}
	receiver, ok := factory.CreateMetricsReceiver(ctx, per.logger, per.promReceiverConfig, per.consumer)
	if ok != nil {
		return fmt.Errorf("unable to create Prometheus receiver: %w", ok)
	}
	per.prometheusReceiver = receiver

	// Start the process with the built config
	go per.manageProcess(ctx, host)

	return nil
}

// getPromReceiverConfig returns the Prometheus receiver config
func getPromReceiverConfig(cfg *Config) *prometheusreceiver.Config {
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

// getSubprocessConfig returns the subprocess config
func getSubprocessConfig(cfg *Config) *subprocessmanager.SubprocessConfig {
	subprocessConfig := &subprocessmanager.SubprocessConfig{}

	subprocessConfig.Command = cfg.SubprocessConfig.Command
	subprocessConfig.Env = cfg.SubprocessConfig.Env

	return subprocessConfig
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
func (per *prometheusExecReceiver) manageProcess(ctx context.Context, host component.Host) {
	var (
		newPort int = per.originalPort

		elapsed    time.Duration
		crashCount int
	)

	for {

		// Generate a port if none was specified and if process had a low runtime (Receiver was stopped)
		generatePort := per.originalPort == 0 && elapsed <= healthyProcessTime
		if generatePort {
			var err error
			newPort, err = per.assignNewRandomPort(ctx)
			if err != nil {
				per.logger.Error("assignNewRandomPort() error - killing this single process/receiver", zap.String("error", err.Error()))
				return
			}
		}

		per.subprocessConfig = per.fillPortPlaceholders(newPort)

		// Start the receiver if it's the first pass, or if the process is unhealthy/a newport was generated meaning the Receiver was stopped last pass
		firstRun := elapsed == 0
		unhealthyProcess := elapsed <= healthyProcessTime && crashCount > healthyCrashCount

		if firstRun || unhealthyProcess || generatePort {
			err := per.prometheusReceiver.Start(ctx, host)
			if err != nil {
				per.logger.Error("Start() error, could not start receiver - killing this single process/receiver", zap.String("error", err.Error()))
				return
			}
		}

		// Start the process
		var subprocessErr error
		elapsed, subprocessErr = per.subprocessConfig.Run(per.logger)
		if subprocessErr != nil {
			per.logger.Info("Subprocess error", zap.String("error", subprocessErr.Error()))
		}

		var shutdownReceiver bool
		crashCount, shutdownReceiver = per.computeHealthAndCrashCount(ctx, elapsed, crashCount)

		if shutdownReceiver {
			err := per.prometheusReceiver.Shutdown(ctx)
			if err != nil {
				per.logger.Error("could not stop receiver associated to process, killing it", zap.String("error", err.Error()))
				return
			}
		}

		sleepTime := subprocessmanager.GetDelay(elapsed, healthyProcessTime, crashCount, healthyCrashCount)
		per.logger.Info("Subprocess start delay", zap.String("time until process restarts", sleepTime.String()))
		time.Sleep(sleepTime)
	}
}

// computeHealthAndCrashCount will decide whether the process is healthy, set the crashCount accordingly and shutdown the receiver if unhealthy
func (per *prometheusExecReceiver) computeHealthAndCrashCount(ctx context.Context, elapsed time.Duration, crashCount int) (int, bool) {
	shutdownReceiver := false

	if elapsed > healthyProcessTime {
		return 1, shutdownReceiver
	}
	crashCount++

	// Stop the associated receiver until process starts back up again since it is unhealthy (too little elapsed time and high crashCount)
	// or if port is generated, to allow for new port
	if per.originalPort == 0 || crashCount > healthyCrashCount {
		shutdownReceiver = true
	}

	return crashCount, shutdownReceiver
}

// assignNewRandomPort generates a new port, creates a new metrics receiver with that port and assigns it to the per
func (per *prometheusExecReceiver) assignNewRandomPort(ctx context.Context) (int, error) {
	var err error
	newPort, err := generateRandomPort()
	if err != nil {
		return 0, err
	}

	per.promReceiverConfig.PrometheusConfig.ScrapeConfigs[0].ServiceDiscoveryConfig.StaticConfigs[0].Targets = []model.LabelSet{
		{model.AddressLabel: model.LabelValue(fmt.Sprintf("localhost:%v", newPort))},
	}

	factory := &prometheusreceiver.Factory{}
	per.prometheusReceiver, err = factory.CreateMetricsReceiver(ctx, per.logger, per.promReceiverConfig, per.consumer)
	if err != nil {
		return 0, fmt.Errorf("unable to create Prometheus receiver. Killing this process")
	}

	return newPort, nil
}

// fillPortPlaceholders will check if any of the strings in the process data have the {{port}} placeholder, and replace it if necessary
func (per *prometheusExecReceiver) fillPortPlaceholders(newPort int) *subprocessmanager.SubprocessConfig {
	port := strconv.Itoa(newPort)

	newConfig := *per.subprocessConfig

	newConfig.Command = strings.ReplaceAll(per.config.SubprocessConfig.Command, portTemplate, port)

	for i, env := range per.config.SubprocessConfig.Env {
		newConfig.Env[i].Value = strings.ReplaceAll(env.Value, portTemplate, port)
	}

	return &newConfig
}

// generateRandomPort will try to generate a random port until it is different than the last one generated
func generateRandomPort() (int, error) {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		return 0, err
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port, nil
}

// Shutdown stops the underlying Prometheus receiver.
func (per *prometheusExecReceiver) Shutdown(ctx context.Context) error {
	return per.prometheusReceiver.Shutdown(ctx)
}
