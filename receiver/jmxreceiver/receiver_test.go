// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package jmxreceiver

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
)

func TestReceiver(t *testing.T) {
	params := receivertest.NewNopSettings()
	config := &Config{
		Endpoint: "service:jmx:protocol:sap",
		OTLPExporterConfig: otlpExporterConfig{
			Endpoint: testutil.GetAvailableLocalAddress(t),
		},
	}

	receiver := newJMXMetricReceiver(params, config, consumertest.NewNop())
	require.NotNil(t, receiver)
	require.Same(t, params.Logger, receiver.logger)
	require.Same(t, config, receiver.config)

	require.NoError(t, receiver.Start(context.Background(), componenttest.NewNopHost()))
	require.NoError(t, receiver.Shutdown(context.Background()))
}

func TestBuildJMXMetricGathererConfig(t *testing.T) {
	passwordFileContents := `
myusername mypassword
keystore: keypass
truststore = trustpass
`
	passwordFilePath := filepath.Join(t.TempDir(), "test.properties")
	require.NoError(t, os.WriteFile(passwordFilePath, []byte(passwordFileContents), 0o600))
	tests := []struct {
		name           string
		config         *Config
		expectedConfig string
		expectedError  string
	}{
		{
			"handles all relevant input appropriately",
			&Config{
				Endpoint:           "myhost:12345",
				TargetSystem:       "mytargetsystem",
				CollectionInterval: 123 * time.Second,
				OTLPExporterConfig: otlpExporterConfig{
					Endpoint: "https://myotlpendpoint",
					TimeoutSettings: exporterhelper.TimeoutConfig{
						Timeout: 234 * time.Second,
					},
					Headers: map[string]string{
						"one":   "two",
						"three": "four",
					},
				},
				// While these aren't realistic usernames/passwords, we want to test the
				// multiline handling in place to reduce the attack surface of the
				// interface to the JMX metrics gatherer
				Username:           "myuser\nname",
				Password:           "mypass \nword",
				Realm:              "myrealm",
				RemoteProfile:      "myprofile",
				TruststorePath:     "/1/2/3",
				TruststorePassword: "trustpass",
				TruststoreType:     "ASCII",
				KeystorePath:       "/my/keystore",
				KeystorePassword:   "keypass",
				KeystoreType:       "JKS",
				ResourceAttributes: map[string]string{
					"abc": "123",
					"one": "two",
				},
			},
			`javax.net.ssl.keyStore = /my/keystore
javax.net.ssl.keyStorePassword = keypass
javax.net.ssl.keyStoreType = JKS
javax.net.ssl.trustStore = /1/2/3
javax.net.ssl.trustStorePassword = trustpass
javax.net.ssl.trustStoreType = ASCII
otel.exporter.otlp.endpoint = https://myotlpendpoint
otel.exporter.otlp.headers = one=two,three=four
otel.exporter.otlp.timeout = 234000
otel.jmx.interval.milliseconds = 123000
otel.jmx.password = mypass \nword
otel.jmx.realm = myrealm
otel.jmx.remote.profile = myprofile
otel.jmx.remote.registry.ssl = false
otel.jmx.service.url = service:jmx:rmi:///jndi/rmi://myhost:12345/jmxrmi
otel.jmx.target.system = mytargetsystem
otel.jmx.username = myuser\nname
otel.metrics.exporter = otlp
otel.resource.attributes = abc=123,one=two`,
			"",
		},
		{
			"handles password file",
			&Config{
				Endpoint:     "myhost:12345",
				TargetSystem: "mytargetsystem",
				OTLPExporterConfig: otlpExporterConfig{
					Endpoint: "https://myotlpendpoint",
				},
				Username:     "myusername",
				PasswordFile: passwordFilePath,
			},
			`javax.net.ssl.keyStorePassword = keypass
javax.net.ssl.trustStorePassword = trustpass
otel.exporter.otlp.endpoint = https://myotlpendpoint
otel.exporter.otlp.timeout = 0
otel.jmx.interval.milliseconds = 0
otel.jmx.password = mypassword
otel.jmx.remote.registry.ssl = false
otel.jmx.service.url = service:jmx:rmi:///jndi/rmi://myhost:12345/jmxrmi
otel.jmx.target.system = mytargetsystem
otel.jmx.username = myusername
otel.metrics.exporter = otlp`,
			"",
		},
		{
			"errors on portless endpoint",
			&Config{
				Endpoint:           "myhostwithoutport",
				TargetSystem:       "mytargetsystem",
				CollectionInterval: 123 * time.Second,
				OTLPExporterConfig: otlpExporterConfig{
					Endpoint: "myotlpendpoint",
					TimeoutSettings: exporterhelper.TimeoutConfig{
						Timeout: 234 * time.Second,
					},
				},
			}, "",
			`failed to parse Endpoint "myhostwithoutport": address myhostwithoutport: missing port in address`,
		},
		{
			"errors on invalid port in endpoint",
			&Config{
				Endpoint:           "myhost:withoutvalidport",
				TargetSystem:       "mytargetsystem",
				CollectionInterval: 123 * time.Second,
				OTLPExporterConfig: otlpExporterConfig{
					Endpoint: "myotlpendpoint",
					TimeoutSettings: exporterhelper.TimeoutConfig{
						Timeout: 234 * time.Second,
					},
				},
			}, "",
			`failed to parse Endpoint "myhost:withoutvalidport": strconv.ParseInt: parsing "withoutvalidport": invalid syntax`,
		},
		{
			"errors on invalid endpoint",
			&Config{
				Endpoint:           ":::",
				TargetSystem:       "mytargetsystem",
				CollectionInterval: 123 * time.Second,
				OTLPExporterConfig: otlpExporterConfig{
					Endpoint: "myotlpendpoint",
					TimeoutSettings: exporterhelper.TimeoutConfig{
						Timeout: 234 * time.Second,
					},
				},
			}, "",
			`failed to parse Endpoint ":::": parse ":::": missing protocol scheme`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(*testing.T) {
			params := receivertest.NewNopSettings()
			receiver := newJMXMetricReceiver(params, test.config, consumertest.NewNop())
			jmxConfig, err := receiver.buildJMXMetricGathererConfig()
			if test.expectedError == "" {
				require.NoError(t, err)
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
		config      *Config
		expectedErr string
	}{
		{
			"missing OTLPExporterConfig.Endpoint",
			&Config{},
			"failed to parse OTLPExporterConfig.Endpoint : missing port in address",
		},
		{
			"invalid OTLPExporterConfig.Endpoint host with 0 port",
			&Config{OTLPExporterConfig: otlpExporterConfig{Endpoint: ".:0"}},
			"failed determining desired port from OTLPExporterConfig.Endpoint .:0: listen tcp: lookup .",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(*testing.T) {
			params := receivertest.NewNopSettings()
			jmxReceiver := newJMXMetricReceiver(params, test.config, consumertest.NewNop())
			otlpReceiver, err := jmxReceiver.buildOTLPReceiver()
			require.ErrorContains(t, err, test.expectedErr)
			require.Nil(t, otlpReceiver)
		})
	}
}
