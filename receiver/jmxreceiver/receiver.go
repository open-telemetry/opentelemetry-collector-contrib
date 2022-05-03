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

package jmxreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/jmxreceiver"

import (
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver/otlpreceiver"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/jmxreceiver/internal/subprocess"
)

// jmxMainClass the class containing the main function for the JMX Metric Gatherer JAR
const jmxMainClass = "io.opentelemetry.contrib.jmxmetrics.JmxMetrics"

var _ component.MetricsReceiver = (*jmxMetricReceiver)(nil)

type jmxMetricReceiver struct {
	logger       *zap.Logger
	config       *Config
	subprocess   *subprocess.Subprocess
	params       component.ReceiverCreateSettings
	otlpReceiver component.MetricsReceiver
	nextConsumer consumer.Metrics
	configFile   string
}

func newJMXMetricReceiver(
	params component.ReceiverCreateSettings,
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

func (jmx *jmxMetricReceiver) Start(ctx context.Context, host component.Host) (err error) {
	jmx.logger.Debug("starting JMX Receiver")

	jmx.otlpReceiver, err = jmx.buildOTLPReceiver()
	if err != nil {
		return err
	}

	javaConfig, err := jmx.buildJMXMetricGathererConfig()
	if err != nil {
		return err
	}

	tmpFile, err := ioutil.TempFile(os.TempDir(), "jmx-config-*.properties")
	if err != nil {
		return fmt.Errorf("failed to get tmp file for jmxreceiver config: %w", err)
	}

	if _, err = tmpFile.Write([]byte(javaConfig)); err != nil {
		return fmt.Errorf("failed to write config file for jmxreceiver config: %w", err)
	}

	// Close the file
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to write config file for jmxreceiver config: %w", err)
	}

	jmx.configFile = tmpFile.Name()
	subprocessConfig := subprocess.Config{
		ExecutablePath: "java",
		Args:           append(jmx.config.parseProperties(), jmxMainClass, "-config", jmx.configFile),
		EnvironmentVariables: map[string]string{
			"CLASSPATH": jmx.config.parseClasspath(),
		},
	}

	jmx.subprocess = subprocess.NewSubprocess(&subprocessConfig, jmx.logger)

	err = jmx.otlpReceiver.Start(ctx, host)
	if err != nil {
		return err
	}
	go func() {
		for range jmx.subprocess.Stdout {
			// ensure stdout/stderr buffer is read from.
			// these messages are already debug logged when captured.
		}
	}()

	return jmx.subprocess.Start(context.Background())
}

func (jmx *jmxMetricReceiver) Shutdown(ctx context.Context) error {
	jmx.logger.Debug("Shutting down JMX Receiver")
	subprocessErr := jmx.subprocess.Shutdown(ctx)
	otlpErr := jmx.otlpReceiver.Shutdown(ctx)
	removeErr := os.Remove(jmx.configFile)
	if subprocessErr != nil {
		return subprocessErr
	}
	if otlpErr != nil {
		return otlpErr
	}
	return removeErr
}

func (jmx *jmxMetricReceiver) buildOTLPReceiver() (component.MetricsReceiver, error) {
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
		port = fmt.Sprintf("%d", addr.Port)
		endpoint = fmt.Sprintf("%s:%s", host, port)
		jmx.config.OTLPExporterConfig.Endpoint = endpoint
	}

	factory := otlpreceiver.NewFactory()
	config := factory.CreateDefaultConfig().(*otlpreceiver.Config)
	config.GRPC.NetAddr = confignet.NetAddr{Endpoint: endpoint, Transport: "tcp"}
	config.HTTP = nil

	return factory.CreateMetricsReceiver(context.Background(), jmx.params, config, jmx.nextConsumer)
}

func (jmx *jmxMetricReceiver) buildJMXMetricGathererConfig() (string, error) {
	config := map[string]string{}
	failedToParse := `failed to parse Endpoint "%s": %w`
	parsed, err := url.Parse(jmx.config.Endpoint)
	if err != nil {
		return "", fmt.Errorf(failedToParse, jmx.config.Endpoint, err)
	}

	if !(parsed.Scheme == "service" && strings.HasPrefix(parsed.Opaque, "jmx:")) {
		host, portStr, err := net.SplitHostPort(jmx.config.Endpoint)
		if err != nil {
			return "", fmt.Errorf(failedToParse, jmx.config.Endpoint, err)
		}
		port, err := strconv.ParseInt(portStr, 10, 0)
		if err != nil {
			return "", fmt.Errorf(failedToParse, jmx.config.Endpoint, err)
		}
		jmx.config.Endpoint = fmt.Sprintf("service:jmx:rmi:///jndi/rmi://%v:%d/jmxrmi", host, port)
	}

	config["otel.jmx.service.url"] = jmx.config.Endpoint
	config["otel.jmx.interval.milliseconds"] = strconv.FormatInt(jmx.config.CollectionInterval.Milliseconds(), 10)
	config["otel.jmx.target.system"] = jmx.config.TargetSystem

	endpoint := jmx.config.OTLPExporterConfig.Endpoint
	if !strings.HasPrefix(endpoint, "http") {
		endpoint = fmt.Sprintf("http://%s", endpoint)
	}

	config["otel.metrics.exporter"] = "otlp"
	config["otel.exporter.otlp.endpoint"] = endpoint
	config["otel.exporter.otlp.timeout"] = strconv.FormatInt(jmx.config.OTLPExporterConfig.Timeout.Milliseconds(), 10)

	if len(jmx.config.OTLPExporterConfig.Headers) > 0 {
		config["otel.exporter.otlp.headers"] = jmx.config.OTLPExporterConfig.headersToString()
	}

	if jmx.config.Username != "" {
		config["otel.jmx.username"] = jmx.config.Username
	}

	if jmx.config.Password != "" {
		config["otel.jmx.password"] = jmx.config.Password
	}

	content := []string{}
	for k, v := range config {
		// Documentation of Java Properties format & escapes: https://docs.oracle.com/javase/7/docs/api/java/util/Properties.html#load(java.io.Reader)
		// As all of the keys are receiver-defined we don't need to escape the reserved characters of ':', '=', or
		// non-newline white space characters. However, we do want to ensure that we handle multiline values
		// correctly just in case a user attempts to provide multiline values for one of the available fields
		safeValue := strings.ReplaceAll(v, "\n", "\\\n")
		content = append(content, fmt.Sprintf("%s = %s", k, safeValue))
	}
	sort.Strings(content)

	return strings.Join(content, "\n"), nil
}
