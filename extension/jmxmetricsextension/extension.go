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

package jmxmetricsextension

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/jmxmetricsextension/subprocess"
)

var _ component.ServiceExtension = (*jmxMetricsExtension)(nil)
var _ component.PipelineWatcher = (*jmxMetricsExtension)(nil)

type jmxMetricsExtension struct {
	logger     *zap.Logger
	config     *config
	subprocess *subprocess.Subprocess
}

func newJmxMetricsExtension(
	logger *zap.Logger,
	config *config,
) *jmxMetricsExtension {
	return &jmxMetricsExtension{
		logger: logger,
		config: config,
	}
}

func (jmx *jmxMetricsExtension) Start(ctx context.Context, host component.Host) error {
	jmx.logger.Debug("Starting JMX Metric Extension")
	javaConfig, err := jmx.buildJmxMetricGathererConfig()
	if err != nil {
		return err
	}
	subprocessConfig := subprocess.Config{
		ExecutablePath: "java",
		Args:           []string{"-Dorg.slf4j.simpleLogger.defaultLogLevel=debug", "-jar", jmx.config.JarPath, "-config", "-"},
		StdInContents:  javaConfig,
	}

	jmx.subprocess = subprocess.NewSubprocess(&subprocessConfig, jmx.logger)
	return nil
}

func (jmx *jmxMetricsExtension) Shutdown(ctx context.Context) error {
	jmx.logger.Debug("Shutting down JMX Metric Extension")
	return jmx.subprocess.Shutdown(ctx)
}

func (jmx *jmxMetricsExtension) Ready() error {
	jmx.logger.Debug("JMX Metric Extension is ready.  Starting subprocess.")
	return jmx.subprocess.Start(context.Background())
}

func (jmx *jmxMetricsExtension) NotReady() error {
	return nil
}

func (jmx *jmxMetricsExtension) buildJmxMetricGathererConfig() (string, error) {
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
`, jmx.config.OtlpEndpoint, jmx.config.OtlpTimeout.Milliseconds())
	} else if jmx.config.Exporter == prometheusExporter {
		javaConfig += fmt.Sprintf(`otel.exporter = prometheus
otel.prometheus.host = %v
otel.prometheus.port = %v
`, jmx.config.PromethusHost, jmx.config.PromethusPort)
	}

	if jmx.config.Username != "" {
		javaConfig += fmt.Sprintf("otel.jmx.username = %v\n", jmx.config.Username)
	}

	if jmx.config.Password != "" {
		javaConfig += fmt.Sprintf("otel.jmx.password = %v\n", jmx.config.Password)
	}

	return javaConfig, nil
}
