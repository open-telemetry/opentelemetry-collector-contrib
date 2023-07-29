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

var _ receiver.Metrics = (*jmxMetricReceiver)(nil)

type jmxMetricReceiver struct {
	logger       *zap.Logger
	config       *Config
	subprocess   *subprocess.Subprocess
	params       receiver.CreateSettings
	otlpReceiver receiver.Metrics
	nextConsumer consumer.Metrics
	configFile   string
}

func newJMXMetricReceiver(
	params receiver.CreateSettings,
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

	var err error
	jmx.otlpReceiver, err = jmx.buildOTLPReceiver()
	if err != nil {
		return err
	}

	javaConfig, err := jmx.buildJMXMetricGathererConfig()
	if err != nil {
		return err
	}

	tmpFile, err := os.CreateTemp(os.TempDir(), "jmx-config-*.properties")
	if err != nil {
		return fmt.Errorf("failed to get tmp file for jmxreceiver config: %w", err)
	}

	if _, err = tmpFile.Write([]byte(javaConfig)); err != nil {
		return fmt.Errorf("failed to write config file for jmxreceiver config: %w", err)
	}

	// Close the file
	if err = tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to write config file for jmxreceiver config: %w", err)
	}

	jmx.configFile = tmpFile.Name()
	subprocessConfig := subprocess.Config{
		ExecutablePath: "java",
		Args:           append(jmx.config.parseProperties(jmx.logger), jmxMainClass, "-config", jmx.configFile),
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
		for range jmx.subprocess.Stdout { // nolint
			// ensure stdout/stderr buffer is read from.
			// these messages are already debug logged when captured.
		}
	}()

	return jmx.subprocess.Start(context.Background())
}

func (jmx *jmxMetricReceiver) Shutdown(ctx context.Context) error {
	if jmx.subprocess == nil {
		return nil
	}
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
		config["otel.jmx.password"] = string(jmx.config.Password)
	}

	if jmx.config.RemoteProfile != "" {
		config["otel.jmx.remote.profile"] = jmx.config.RemoteProfile
	}

	if jmx.config.Realm != "" {
		config["otel.jmx.realm"] = jmx.config.Realm
	}

	if jmx.config.KeystorePath != "" {
		config["javax.net.ssl.keyStore"] = jmx.config.KeystorePath
	}
	if jmx.config.KeystorePassword != "" {
		config["javax.net.ssl.keyStorePassword"] = string(jmx.config.KeystorePassword)
	}
	if jmx.config.KeystoreType != "" {
		config["javax.net.ssl.keyStoreType"] = jmx.config.KeystoreType
	}
	if jmx.config.TruststorePath != "" {
		config["javax.net.ssl.trustStore"] = jmx.config.TruststorePath
	}
	if jmx.config.TruststorePassword != "" {
		config["javax.net.ssl.trustStorePassword"] = string(jmx.config.TruststorePassword)
	}
	if jmx.config.TruststoreType != "" {
		config["javax.net.ssl.trustStoreType"] = jmx.config.TruststoreType
	}

	if len(jmx.config.ResourceAttributes) > 0 {
		attributes := make([]string, 0, len(jmx.config.ResourceAttributes))
		for k, v := range jmx.config.ResourceAttributes {
			attributes = append(attributes, fmt.Sprintf("%s=%s", k, v))
		}
		sort.Strings(attributes)
		config["otel.resource.attributes"] = strings.Join(attributes, ",")
	}

	var content []string
	for k, v := range config {
		// Documentation of Java Properties format & escapes: https://docs.oracle.com/javase/7/docs/api/java/util/Properties.html#load(java.io.Reader)

		// Keys are receiver-defined so this escape should be unnecessary but in case that assumption
		// breaks in the future this will ensure keys are properly escaped
		safeKey := strings.ReplaceAll(k, "=", "\\=")
		safeKey = strings.ReplaceAll(safeKey, ":", "\\:")
		// Any whitespace must be removed from keys
		safeKey = strings.ReplaceAll(safeKey, " ", "")
		safeKey = strings.ReplaceAll(safeKey, "\t", "")
		safeKey = strings.ReplaceAll(safeKey, "\n", "")

		// Unneeded escape tokens will be removed by the properties file loader, so it should be pre-escaped to ensure
		// the values provided reach the metrics gatherer as provided. Also in case a user attempts to provide multiline
		// values for one of the available fields, we need to escape the newlines
		safeValue := strings.ReplaceAll(v, "\\", "\\\\")
		safeValue = strings.ReplaceAll(safeValue, "\n", "\\n")
		content = append(content, fmt.Sprintf("%s = %s", safeKey, safeValue))
	}
	sort.Strings(content)

	return strings.Join(content, "\n"), nil
}
