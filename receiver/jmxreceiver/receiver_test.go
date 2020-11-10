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

package jmxreceiver

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/testutil"
	"go.uber.org/zap"
)

func TestReceiver(t *testing.T) {
	params := component.ReceiverCreateParams{Logger: zap.NewNop()}
	config := &config{
		OTLPExporterConfig: otlpExporterConfig{
			Endpoint: fmt.Sprintf("localhost:%d", testutil.GetAvailablePort(t)),
		},
	}

	receiver := newJMXMetricReceiver(params, config, consumertest.NewMetricsNop())
	require.NotNil(t, receiver)
	require.Same(t, params.Logger, receiver.logger)
	require.Same(t, config, receiver.config)

	require.Nil(t, receiver.Start(context.Background(), componenttest.NewNopHost()))
	require.Nil(t, receiver.Shutdown(context.Background()))
}

func TestBuildJMXMetricGathererConfig(t *testing.T) {
	tests := []struct {
		name           string
		config         config
		expectedConfig string
	}{
		{
			"uses target system",
			config{
				ServiceURL:         "myserviceurl",
				TargetSystem:       "mytargetsystem",
				GroovyScript:       "mygroovyscript",
				CollectionInterval: 123 * time.Second,
				OTLPExporterConfig: otlpExporterConfig{
					Endpoint: "myotlpendpoint",
					TimeoutSettings: exporterhelper.TimeoutSettings{
						Timeout: 234 * time.Second,
					},
				},
			},
			`otel.jmx.service.url = myserviceurl
otel.jmx.interval.milliseconds = 123000
otel.jmx.target.system = mytargetsystem
otel.exporter = otlp
otel.exporter.otlp.endpoint = myotlpendpoint
otel.exporter.otlp.metric.timeout = 234000
`,
		},
		{
			"uses groovy script",
			config{
				ServiceURL:         "myserviceurl",
				GroovyScript:       "mygroovyscript",
				CollectionInterval: 123 * time.Second,
				OTLPExporterConfig: otlpExporterConfig{
					Endpoint: "myotlpendpoint",
					TimeoutSettings: exporterhelper.TimeoutSettings{
						Timeout: 234 * time.Second,
					},
				},
			},
			`otel.jmx.service.url = myserviceurl
otel.jmx.interval.milliseconds = 123000
otel.jmx.groovy.script = mygroovyscript
otel.exporter = otlp
otel.exporter.otlp.endpoint = myotlpendpoint
otel.exporter.otlp.metric.timeout = 234000
`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(tt *testing.T) {
			params := component.ReceiverCreateParams{Logger: zap.NewNop()}
			receiver := newJMXMetricReceiver(params, &test.config, consumertest.NewMetricsNop())
			jmxConfig, err := receiver.buildJMXMetricGathererConfig()
			require.NoError(t, err)
			require.Equal(t, test.expectedConfig, jmxConfig)
		})
	}
}

func TestBuildOTLPReceiverInvalidEndpoints(t *testing.T) {
	tests := []struct {
		name        string
		config      config
		expectedErr string
	}{
		{
			"missing OTLPExporterConfig.Endpoint",
			config{},
			"failed to parse OTLPExporterConfig.Endpoint : missing port in address",
		},
		{
			"invalid OTLPExporterConfig.Endpoint host with 0 port",
			config{OTLPExporterConfig: otlpExporterConfig{Endpoint: ".:0"}},
			"failed determining desired port from OTLPExporterConfig.Endpoint .:0: listen tcp: lookup .: no such host",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(tt *testing.T) {
			params := component.ReceiverCreateParams{Logger: zap.NewNop()}
			jmxReceiver := newJMXMetricReceiver(params, &test.config, consumertest.NewMetricsNop())
			otlpReceiver, err := jmxReceiver.buildOTLPReceiver()
			require.Error(t, err)
			require.EqualError(t, err, test.expectedErr)
			require.Nil(t, otlpReceiver)
		})
	}
}
