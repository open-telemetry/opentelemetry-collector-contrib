// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package jmxreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/jmxreceiver"

import (
	"context"
	"fmt"
	"net"
	"os"
	"strconv"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/otlpreceiver"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/jmxreceiver/internal/subprocess"
)

var _ receiver.Metrics = (*jmxMetricReceiver)(nil)

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

	javaConfig, err := jmx.config.buildJMXConfig()
	if err != nil {
		return err
	}

	tmpFile, err := os.CreateTemp(os.TempDir(), "jmx-config-*.properties")
	if err != nil {
		return fmt.Errorf("failed to get tmp file for jmxreceiver config: %w", err)
	}

	if _, err = tmpFile.WriteString(javaConfig); err != nil {
		return fmt.Errorf("failed to write config file for jmxreceiver config: %w", err)
	}

	// Close the file
	if err = tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to write config file for jmxreceiver config: %w", err)
	}

	jmx.configFile = tmpFile.Name()
	subprocessConfig := subprocess.Config{
		ExecutablePath: "java",
		Args:           append(jmx.config.parseProperties(jmx.logger), jmx.config.jarMainClass(), "-config", jmx.configFile),
		EnvironmentVariables: map[string]string{
			"CLASSPATH": jmx.config.parseClasspath(),
			// Overwrite these environment variables to reduce attack surface
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
				// ensure stdout/stderr buffer is read from.
				// these messages are already debug logged when captured.
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

// insertDefault is a helper function to insert a default value for a configoptional.Optional type.
func insertDefault[T any](opt *configoptional.Optional[T]) error {
	if opt.HasValue() {
		return nil
	}

	empty := confmap.NewFromStringMap(map[string]any{})
	return empty.Unmarshal(opt)
}

func (jmx *jmxMetricReceiver) buildOTLPReceiver() (receiver.Metrics, error) {
	endpoint := jmx.config.OTLPExporterConfig.Endpoint
	host, port, err := net.SplitHostPort(endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to parse OTLPExporterConfig.Endpoint %s: %w", jmx.config.OTLPExporterConfig.Endpoint, err)
	}
	if port == "0" {
		// We need to know the port OTLP receiver will use to specify w/ java properties and not
		// rely on gRPC server's connection.
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
	if err := insertDefault(&config.GRPC); err != nil {
		return nil, err
	}
	config.GRPC.Get().NetAddr = confignet.AddrConfig{Endpoint: endpoint, Transport: confignet.TransportTypeTCP}

	params := receiver.Settings{
		ID:                component.NewIDWithName(factory.Type(), jmx.params.ID.String()),
		TelemetrySettings: jmx.params.TelemetrySettings,
		BuildInfo:         jmx.params.BuildInfo,
	}
	return factory.CreateMetrics(context.Background(), params, config, jmx.nextConsumer)
}
