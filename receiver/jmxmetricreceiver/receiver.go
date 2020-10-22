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

package jmxmetricextension

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/jmxmetricextension/subprocess"
)

var _ component.ServiceExtension = (*jmxMetricExtension)(nil)
var _ component.PipelineWatcher = (*jmxMetricExtension)(nil)

type jmxMetricExtension struct {
	logger     *zap.Logger
	config     *config
	subprocess *subprocess.Subprocess
}

func newJMXMetricExtension(
	logger *zap.Logger,
	config *config,
) *jmxMetricExtension {
	return &jmxMetricExtension{
		logger: logger,
		config: config,
	}
}

func (jmx *jmxMetricExtension) Start(ctx context.Context, host component.Host) error {
	jmx.logger.Debug("Starting JMX Metric Extension")
	javaConfig, err := jmx.buildJMXMetricGathererConfig()
	if err != nil {
		return err
	}
	subprocessConfig := subprocess.Config{
		ExecutablePath: "java",
		Args:           []string{"-Dorg.slf4j.simpleLogger.defaultLogLevel=debug", "-jar", jmx.config.JARPath, "-config", "-"},
		StdInContents:  javaConfig,
	}

	jmx.subprocess = subprocess.NewSubprocess(&subprocessConfig, jmx.logger)
	return nil
}

func (jmx *jmxMetricExtension) Shutdown(ctx context.Context) error {
	jmx.logger.Debug("Shutting down JMX Metric Extension")
	return jmx.subprocess.Shutdown(ctx)
}

func (jmx *jmxMetricExtension) Ready() error {
	jmx.logger.Debug("JMX Metric Extension is ready.  Starting subprocess.")
	return jmx.subprocess.Start(context.Background())
}

func (jmx *jmxMetricExtension) NotReady() error {
	return nil
}

func (jmx *jmxMetricExtension) buildJMXMetricGathererConfig() (string, error) {
	javaConfig := fmt.Sprintf(`otel.jmx.service.url = %v
otel.jmx.interval.milliseconds = %v
`, jmx.config.ServiceURL, jmx.config.Interval.Milliseconds())

	if jmx.config.TargetSystem != "" {
		javaConfig += fmt.Sprintf("otel.jmx.target.system = %v\n", jmx.config.TargetSystem)
	} else if jmx.config.GroovyScript != "" {
		javaConfig += fmt.Sprintf("otel.jmx.groovy.script = %v\n", jmx.config.GroovyScript)
	}

	if jmx.config.Exporter == otlpExporter {
		javaConfig += fmt.Sprintf(`otel.exporter = otlp
otel.otlp.endpoint = %v
otel.otlp.metric.timeout = %v
`, jmx.config.OTLPEndpoint, jmx.config.OTLPTimeout.Milliseconds())
	} else if jmx.config.Exporter == prometheusExporter {
		javaConfig += fmt.Sprintf(`otel.exporter = prometheus
otel.prometheus.host = %v
otel.prometheus.port = %v
`, jmx.config.PrometheusHost, jmx.config.PrometheusPort)
	}

	if jmx.config.Username != "" {
		javaConfig += fmt.Sprintf("otel.jmx.username = %v\n", jmx.config.Username)
	}

	if jmx.config.Password != "" {
		javaConfig += fmt.Sprintf("otel.jmx.password = %v\n", jmx.config.Password)
	}

	return javaConfig, nil
}
