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
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.uber.org/zap"
)

func TestExtension(t *testing.T) {
	logger := zap.NewNop()
	config := &config{}

	extension := newJMXMetricExtension(logger, config)
	require.NotNil(t, extension)
	require.Same(t, logger, extension.logger)
	require.Same(t, config, extension.config)

	require.Nil(t, extension.Start(context.Background(), componenttest.NewNopHost()))
	require.Nil(t, extension.Ready())
	require.Nil(t, extension.Shutdown(context.Background()))
	require.Nil(t, extension.NotReady())
}

func TestBuildJMXMetricGathererOTLPConfig(t *testing.T) {
	logger := zap.NewNop()
	config := &config{
		ServiceURL:     "myserviceurl",
		TargetSystem:   "mytargetsystem",
		GroovyScript:   "mygroovyscript",
		Interval:       123 * time.Second,
		Exporter:       "otlp",
		OTLPEndpoint:   "myotlpendpoint",
		OTLPTimeout:    234 * time.Second,
		PrometheusHost: "myprometheushost",
		PrometheusPort: 12345,
	}

	expectedConfig := `otel.jmx.service.url = myserviceurl
otel.jmx.interval.milliseconds = 123000
otel.jmx.target.system = mytargetsystem
otel.exporter = otlp
otel.otlp.endpoint = myotlpendpoint
otel.otlp.metric.timeout = 234000
`
	extension := newJMXMetricExtension(logger, config)
	jmxConfig, err := extension.buildJMXMetricGathererConfig()
	require.NoError(t, err)
	require.Equal(t, expectedConfig, jmxConfig)
}

func TestBuildJMXMetricGathererPrometheusConfig(t *testing.T) {
	logger := zap.NewNop()
	config := &config{
		ServiceURL:     "myserviceurl",
		GroovyScript:   "mygroovyscript",
		Interval:       123 * time.Second,
		Exporter:       "prometheus",
		OTLPEndpoint:   "myotlpendpoint",
		OTLPTimeout:    234 * time.Second,
		PrometheusHost: "myprometheushost",
		PrometheusPort: 12345,
	}

	expectedConfig := `otel.jmx.service.url = myserviceurl
otel.jmx.interval.milliseconds = 123000
otel.jmx.groovy.script = mygroovyscript
otel.exporter = prometheus
otel.prometheus.host = myprometheushost
otel.prometheus.port = 12345
`
	extension := newJMXMetricExtension(logger, config)
	jmxConfig, err := extension.buildJMXMetricGathererConfig()
	require.NoError(t, err)
	require.Equal(t, expectedConfig, jmxConfig)
}
