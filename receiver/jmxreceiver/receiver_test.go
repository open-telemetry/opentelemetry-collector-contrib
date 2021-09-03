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

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/testutil"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
	"google.golang.org/grpc/codes"
)

func TestReceiver(t *testing.T) {
	params := componenttest.NewNopReceiverCreateSettings()
	config := &Config{
		Endpoint: "service:jmx:protocol:sap",
		OTLPExporterConfig: otlpExporterConfig{
			Endpoint: fmt.Sprintf("localhost:%d", testutil.GetAvailablePort(t)),
		},
	}

	receiver, err := newJMXMetricReceiver(params, config, consumertest.NewNop())
	require.NoError(t, err)

	require.NotNil(t, receiver)
	require.Same(t, params.Logger, receiver.logger)
	require.Same(t, config, receiver.config)

	require.NoError(t, receiver.Start(context.Background(), componenttest.NewNopHost()))
	require.NoError(t, receiver.Shutdown(context.Background()))
}

func TestReceiverStatusCodes(t *testing.T) {
	params := componenttest.NewNopReceiverCreateSettings()
	config := &Config{
		Endpoint: "service:jmx:protocol:sap",
		OTLPExporterConfig: otlpExporterConfig{
			Endpoint: fmt.Sprintf("localhost:%d", testutil.GetAvailablePort(t)),
		},
	}

	tcs := []struct {
		name       string
		statusCode string
		errorStr   string
	}{
		{
			name:       "positive interval",
			statusCode: codes.InvalidArgument.String(),
			errorStr:   "`interval` must be positive:",
		},
		{
			name:       "positive timeout",
			statusCode: codes.InvalidArgument.String(),
			errorStr:   "`otlp.timeout` must be positive:",
		},
		{
			name:       "jmx required fields",
			statusCode: codes.InvalidArgument.String(),
			errorStr:   "jmx missing required fields:",
		},
		{
			name:       "endpoint parsing",
			statusCode: codes.InvalidArgument.String(),
			errorStr:   "failed to parse OTLPExporterConfig.Endpoint",
		},
		{
			name:       "missing cancel",
			statusCode: codes.Internal.String(),
			errorStr:   "no subprocess.cancel().",
		},
		{
			name:       "passed timeout",
			statusCode: codes.OutOfRange.String(),
			errorStr:   "subprocess hasn't returned within shutdown timeout.",
		},
		{
			name:       "unexpected shutdown",
			statusCode: codes.Aborted.String(),
			errorStr:   "unexpected shutdown:",
		},
		{
			name:       "can't create subprocess input pipe",
			statusCode: codes.Internal.String(),
			errorStr:   "Input pipe could not be created for subprocess",
		},
		{
			name:       "can't create subprocess output pipe",
			statusCode: codes.Internal.String(),
			errorStr:   "Output pipe could not be created for subprocess",
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			obs, logs := observer.New(zap.ErrorLevel)
			params.Logger = zap.New(obs)
			receiver, err := newJMXMetricReceiver(params, config, consumertest.NewNop())
			require.NoError(t, err)
			receiver.subprocess = NewMockSubprocess(receiver.logger, tc.errorStr)
			require.NotNil(t, receiver)

			require.NoError(t, receiver.Start(context.Background(), componenttest.NewNopHost()))
			require.NoError(t, receiver.Shutdown(context.Background()))

			require.NotEqual(t, 0, len(logs.AllUntimed()))
			logMap := logs.AllUntimed()[0].ContextMap()
			require.Contains(t, tc.errorStr, logs.AllUntimed()[0].Message)
			require.Equal(t, tc.statusCode, logMap["status_code"])
		})
	}
}

func TestBuildJMXMetricGathererConfig(t *testing.T) {
	tests := []struct {
		name           string
		config         Config
		expectedConfig string
		expectedError  string
	}{
		{
			"uses target system",
			Config{
				Endpoint:           "service:jmx:rmi///jndi/rmi://myservice:12345/jmxrmi/",
				TargetSystem:       "mytargetsystem",
				GroovyScript:       "mygroovyscript",
				CollectionInterval: 123 * time.Second,
				OTLPExporterConfig: otlpExporterConfig{
					Endpoint: "myotlpendpoint:900",
					TimeoutSettings: exporterhelper.TimeoutSettings{
						Timeout: 234 * time.Second,
					},
				},
			},
			`otel.jmx.service.url = service:jmx:rmi///jndi/rmi://myservice:12345/jmxrmi/
otel.jmx.interval.milliseconds = 123000
otel.jmx.target.system = mytargetsystem
otel.metrics.exporter = otlp
otel.exporter.otlp.endpoint = http://myotlpendpoint:900
otel.exporter.otlp.timeout = 234000
`, "",
		},
		{
			"uses groovy script",
			Config{
				Endpoint:           "service:jmx:rmi///jndi/rmi://myservice:12345/jmxrmi/",
				GroovyScript:       "mygroovyscript",
				CollectionInterval: 123 * time.Second,
				OTLPExporterConfig: otlpExporterConfig{
					Endpoint: "http://myotlpendpoint",
					TimeoutSettings: exporterhelper.TimeoutSettings{
						Timeout: 234 * time.Second,
					},
				},
			},
			`otel.jmx.service.url = service:jmx:rmi///jndi/rmi://myservice:12345/jmxrmi/
otel.jmx.interval.milliseconds = 123000
otel.jmx.groovy.script = mygroovyscript
otel.metrics.exporter = otlp
otel.exporter.otlp.endpoint = http://myotlpendpoint
otel.exporter.otlp.timeout = 234000
`, "",
		},
		{
			"uses endpoint as service url",
			Config{
				Endpoint:           "myhost:12345",
				TargetSystem:       "mytargetsystem",
				GroovyScript:       "mygroovyscript",
				CollectionInterval: 123 * time.Second,
				OTLPExporterConfig: otlpExporterConfig{
					Endpoint: "https://myotlpendpoint",
					TimeoutSettings: exporterhelper.TimeoutSettings{
						Timeout: 234 * time.Second,
					},
					Headers: map[string]string{
						"one":   "two",
						"three": "four",
					},
				},
			},
			`otel.jmx.service.url = service:jmx:rmi:///jndi/rmi://myhost:12345/jmxrmi
otel.jmx.interval.milliseconds = 123000
otel.jmx.target.system = mytargetsystem
otel.metrics.exporter = otlp
otel.exporter.otlp.endpoint = https://myotlpendpoint
otel.exporter.otlp.timeout = 234000
otel.exporter.otlp.headers = one=two,three=four
`, "",
		},
		{
			"errors on portless endpoint",
			Config{
				Endpoint:           "myhostwithoutport",
				TargetSystem:       "mytargetsystem",
				GroovyScript:       "mygroovyscript",
				CollectionInterval: 123 * time.Second,
				OTLPExporterConfig: otlpExporterConfig{
					Endpoint: "myotlpendpoint",
					TimeoutSettings: exporterhelper.TimeoutSettings{
						Timeout: 234 * time.Second,
					},
				},
			}, "",
			`failed to parse OTLPExporterConfig.Endpoint myotlpendpoint: address myotlpendpoint: missing port in address`,
		},
		{
			"errors on invalid port in endpoint",
			Config{
				Endpoint:           "myhost:withoutvalidport",
				TargetSystem:       "mytargetsystem",
				GroovyScript:       "mygroovyscript",
				CollectionInterval: 123 * time.Second,
				OTLPExporterConfig: otlpExporterConfig{
					Endpoint: "myotlpendpoint",
					TimeoutSettings: exporterhelper.TimeoutSettings{
						Timeout: 234 * time.Second,
					},
				},
			}, "",
			`failed to parse OTLPExporterConfig.Endpoint myotlpendpoint: address myotlpendpoint: missing port in address`,
		},
		{
			"errors on invalid endpoint",
			Config{
				Endpoint:           ":::",
				TargetSystem:       "mytargetsystem",
				GroovyScript:       "mygroovyscript",
				CollectionInterval: 123 * time.Second,
				OTLPExporterConfig: otlpExporterConfig{
					Endpoint: "myotlpendpoint",
					TimeoutSettings: exporterhelper.TimeoutSettings{
						Timeout: 234 * time.Second,
					},
				},
			}, "",
			`failed to parse OTLPExporterConfig.Endpoint myotlpendpoint: address myotlpendpoint: missing port in address`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(tt *testing.T) {
			params := componenttest.NewNopReceiverCreateSettings()
			receiver, err := newJMXMetricReceiver(params, &test.config, consumertest.NewNop())
			var jmxConfig string
			if test.expectedError == "" {
				require.NoError(t, err)
				jmxConfig, _ = receiver.buildJMXMetricGathererConfig()
			} else {
				require.Error(t, err)
				require.EqualError(t, err, test.expectedError)
			}

			require.Equal(t, test.expectedConfig, jmxConfig)
		})
	}
}

func TestBuildOTLPReceiverInvalidEndpoints(t *testing.T) {
	tests := []struct {
		name        string
		config      Config
		expectedErr string
	}{
		{
			"missing OTLPExporterConfig.Endpoint",
			Config{},
			"failed to parse OTLPExporterConfig.Endpoint : missing port in address",
		},
		{
			"invalid OTLPExporterConfig.Endpoint host with 0 port",
			Config{OTLPExporterConfig: otlpExporterConfig{Endpoint: ".:0"}},
			"failed determining desired port from OTLPExporterConfig.Endpoint .:0: listen tcp: lookup .: no such host",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(tt *testing.T) {
			params := componenttest.NewNopReceiverCreateSettings()
			_, err := newJMXMetricReceiver(params, &test.config, consumertest.NewNop())
			require.Error(t, err)
		})
	}
}
