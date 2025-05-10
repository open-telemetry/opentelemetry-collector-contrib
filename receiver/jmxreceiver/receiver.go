// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package jmxreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/jmxreceiver"

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/otlpreceiver"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/jmxreceiver/internal/subprocess"
)

// jmxMainClass the class containing the main function for the JMX Metric Gatherer JAR
const jmxMainClass = "io.opentelemetry.contrib.jmxmetrics.JmxMetrics"

type jmxMetricReceiver struct {
	logger       *zap.Logger
	config       *Config
	subprocess   *subprocess.Subprocess
	params       receiver.Settings
	otlpReceiver receiver.Metrics
	nextConsumer consumer.Metrics
	configFile   string
	cancel       context.CancelFunc
}

func newJMXMetricReceiver(
	params receiver.Settings,
	config *Config,
	nextConsumer consumer.Metrics,
) *jmxMetricReceiver {
	return &jmxMetricReceiver{
		logger:       params.Logger,
		params:       params,
		config:       config,
		nextConsumer: nextConsumer,
	}
}

func (jmx *jmxMetricReceiver) Start(ctx context.Context, host component.Host) error {
	jmx.logger.Debug("starting JMX Receiver")

	ctx, jmx.cancel = context.WithCancel(ctx)

	var err error
	jmx.otlpReceiver, err = jmx.buildOTLPReceiver()
	if err != nil {
		return err
	}

	javaConfig, err := jmx.buildJMXMetricGathererConfig()
	jmx.logger.Info("Java Config: ", zap.Any("javaConfig", javaConfig))
	if err != nil {
		return err
	}

	tmpFile, err := os.Create(os.TempDir() + "/config.properties")
	if err != nil {
		return fmt.Errorf("failed to get tmp file for jmxreceiver config: %w", err)
	}

	if _, err = tmpFile.Write([]byte(javaConfig)); err != nil {
		return fmt.Errorf("failed to write config file for jmxreceiver config: %w", err)
	}

	if err = tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to write config file for jmxreceiver config: %w", err)
	}

	jmx.configFile = tmpFile.Name()

	var properties []string
	classpath := ""
	for _, appConfig := range jmx.config.Applications {
		classpath = appConfig.parseClasspath()
		properties = appConfig.parseProperties(jmx.logger)
		break
	}

	subprocessConfig := subprocess.Config{
		ExecutablePath: "java",
		Args:           append(properties, jmxMainClass, "-config", jmx.configFile),
		EnvironmentVariables: map[string]string{
			"CLASSPATH":         classpath,
			"JAVA_TOOL_OPTIONS": "",
			"LD_PRELOAD":        "",
		},
	}

	jmx.subprocess = subprocess.NewSubprocess(&subprocessConfig, jmx.logger)

	err = jmx.otlpReceiver.Start(ctx, host)
	if err != nil {
		return err
	}
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-jmx.subprocess.Stdout:
				continue
			}
		}
	}()

	return jmx.subprocess.Start(ctx)
}

func (jmx *jmxMetricReceiver) Shutdown(ctx context.Context) error {
	if jmx.subprocess == nil {
		return nil
	}
	jmx.logger.Debug("Shutting down JMX Receiver")
	subprocessErr := jmx.subprocess.Shutdown(ctx)
	otlpErr := jmx.otlpReceiver.Shutdown(ctx)

	if jmx.cancel != nil {
		jmx.cancel()
	}

	removeErr := os.Remove(jmx.configFile)
	if subprocessErr != nil {
		return subprocessErr
	}
	if otlpErr != nil {
		return otlpErr
	}
	return removeErr
}

func (jmx *jmxMetricReceiver) buildOTLPReceiver() (receiver.Metrics, error) {
	endpoint := jmx.config.OTLPExporterConfig.Endpoint
	host, port, err := net.SplitHostPort(endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to parse OTLPExporterConfig.Endpoint %s: %w", jmx.config.OTLPExporterConfig.Endpoint, err)
	}
	if port == "0" {
		listener, err := net.Listen("tcp", endpoint)
		if err != nil {
			return nil, fmt.Errorf(
				"failed determining desired port from OTLPExporterConfig.Endpoint %s: %w", jmx.config.OTLPExporterConfig.Endpoint, err,
			)
		}
		defer listener.Close()
		addr := listener.Addr().(*net.TCPAddr)
		port = strconv.Itoa(addr.Port)
		endpoint = fmt.Sprintf("%s:%s", host, port)
		jmx.config.OTLPExporterConfig.Endpoint = endpoint
	}

	factory := otlpreceiver.NewFactory()
	config := factory.CreateDefaultConfig().(*otlpreceiver.Config)
	config.GRPC.NetAddr = confignet.AddrConfig{Endpoint: endpoint, Transport: confignet.TransportTypeTCP}
	config.HTTP = nil

	params := receiver.Settings{
		ID:                component.NewIDWithName(factory.Type(), jmx.params.ID.String()),
		TelemetrySettings: jmx.params.TelemetrySettings,
		BuildInfo:         jmx.params.BuildInfo,
	}
	return factory.CreateMetrics(context.Background(), params, config, jmx.nextConsumer)
}

func (jmx *jmxMetricReceiver) buildJMXMetricGathererConfig() (string, error) {
	config := make(map[string]map[string]string)
	var errors []error

	for appName, appConfig := range jmx.config.Applications {
		if _, exists := config[appName]; !exists {
			config[appName] = make(map[string]string)
		}
		parsed, err := url.Parse(appConfig.Endpoint)
		if err != nil {
			errors = append(errors, fmt.Errorf("failed to parse endpoint %q: %w", appConfig.Endpoint, err))
			continue
		}

		if !(parsed.Scheme == "service" && strings.HasPrefix(parsed.Opaque, "jmx:")) {
			host, portStr, err := net.SplitHostPort(appConfig.Endpoint)
			if err != nil {
				errors = append(errors, fmt.Errorf("failed to parse endpoint %q: %w", appConfig.Endpoint, err))
				continue
			}
			port, err := strconv.ParseInt(portStr, 10, 0)
			if err != nil {
				errors = append(errors, fmt.Errorf("failed to parse port in endpoint %q: %w", appConfig.Endpoint, err))
				continue
			}
			appConfig.Endpoint = fmt.Sprintf("service:jmx:rmi:///jndi/rmi://%v:%d/jmxrmi", host, port)
		}

		config[appName]["otel.jmx.service.url"] = appConfig.Endpoint
		config[appName]["otel.jmx.interval.milliseconds"] = strconv.FormatInt(appConfig.CollectionInterval.Milliseconds(), 10)
		config[appName]["otel.jmx.target.system"] = appConfig.TargetSystem

		endpoint := jmx.config.OTLPExporterConfig.Endpoint
		if !strings.HasPrefix(endpoint, "http") {
			endpoint = "http://" + endpoint
		}

		config[appName]["otel.metrics.exporter"] = "otlp"
		config[appName]["otel.exporter.otlp.endpoint"] = endpoint
		config[appName]["otel.exporter.otlp.timeout"] = strconv.FormatInt(jmx.config.OTLPExporterConfig.TimeoutSettings.Timeout.Milliseconds(), 10)

		if len(appConfig.ResourceAttributes) > 0 {
			attributes := make([]string, 0, len(appConfig.ResourceAttributes))
			for k, v := range appConfig.ResourceAttributes {
				attributes = append(attributes, fmt.Sprintf("%s=%s", k, v))
			}
			sort.Strings(attributes)
			config[appName]["otel.resource.attributes"] = strings.Join(attributes, ",")
		}
	}

	jmx.logger.Info("Errors parsing JMX endpoints", zap.Errors("errors", errors))

	var content []string
	for appName, appConfig := range config {
		content = append(content, fmt.Sprintf("%s:", appName))
		var appContent []string
		for k, v := range appConfig {
			safeKey := strings.ReplaceAll(k, "=", "\\=")
			safeKey = strings.ReplaceAll(safeKey, ":", "\\:")
			safeKey = strings.ReplaceAll(safeKey, " ", "")
			safeKey = strings.ReplaceAll(safeKey, "\t", "")
			safeKey = strings.ReplaceAll(safeKey, "\n", "")

			safeValue := strings.ReplaceAll(v, "\\", "\\\\")
			safeValue = strings.ReplaceAll(safeValue, "\n", "\\n")
			appContent = append(appContent, fmt.Sprintf("  %s = %s", safeKey, safeValue))
		}
		sort.Strings(appContent)
		content = append(content, strings.Join(appContent, "\n"))
	}

	return strings.Join(content, "\n"), nil
}
