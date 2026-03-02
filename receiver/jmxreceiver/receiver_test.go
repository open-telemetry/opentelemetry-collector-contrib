// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package jmxreceiver

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/jmxreceiver/internal/metadata"
)

func TestReceiver(t *testing.T) {
	params := receivertest.NewNopSettings(metadata.Type)
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

	require.NoError(t, receiver.Start(t.Context(), componenttest.NewNopHost()))
	require.NoError(t, receiver.Shutdown(t.Context()))
}

func TestBuildJMXConfig(t *testing.T) {
	scs := scraperhelper.NewDefaultControllerConfig()
	scs.CollectionInterval = 123 * time.Second

	tests := []struct {
		name                  string
		mockedGathererJarHash string
		mockedScraperJarHash  string
		config                *Config
		expectedConfig        string
		expectedError         string
	}{
		{
			"handles all JMX Metric Gatherer relevant input appropriately",
			"5994471abb01112afcc18159f6cc74b4f511b99806da59b3caf5a9c173cacfc5",
			"",
			&Config{
				JARPath:          "testdata/fake_jmx.jar",
				Endpoint:         "myhost:12345",
				TargetSystem:     "mytargetsystem",
				ControllerConfig: scs,
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
otel.jmx.service.url = service:jmx:rmi:///jndi/rmi://myhost:12345/jmxrmi
otel.jmx.target.system = mytargetsystem
otel.jmx.username = myuser\nname
otel.metrics.exporter = otlp
otel.resource.attributes = abc=123,one=two`,
			"",
		},
		{
			"handles all JMX Scraper relevant input appropriately",
			"",
			"dce3d9a8457bb5097144e88e1c1246f428e047a677462cff1a638c172c7eeab1",
			&Config{
				JARPath:          "testdata/fake_jmx_scraper.jar",
				Endpoint:         "myhost:12345",
				TargetSystem:     "mytargetsystem",
				ControllerConfig: scs,
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
otel.jmx.password = mypass \nword
otel.jmx.realm = myrealm
otel.jmx.remote.profile = myprofile
otel.jmx.service.url = service:jmx:rmi:///jndi/rmi://myhost:12345/jmxrmi
otel.jmx.target.source = auto
otel.jmx.target.system = mytargetsystem
otel.jmx.username = myuser\nname
otel.metric.export.interval = 2m3s
otel.metrics.exporter = otlp
otel.resource.attributes = abc=123,one=two`,
			"",
		},
		{
			"errors on portless endpoint",
			"",
			"",
			&Config{
				Endpoint:         "myhostwithoutport",
				TargetSystem:     "mytargetsystem",
				ControllerConfig: scs,
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
			"",
			"",
			&Config{
				Endpoint:         "myhost:withoutvalidport",
				TargetSystem:     "mytargetsystem",
				ControllerConfig: scs,
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
			"",
			"",
			&Config{
				Endpoint:         ":::",
				TargetSystem:     "mytargetsystem",
				ControllerConfig: scs,
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
			// mock supported JAR
			MetricsGathererHash = test.mockedGathererJarHash
			ScraperHash = test.mockedScraperJarHash
			initSupportedJars()
			params := receivertest.NewNopSettings(metadata.Type)
			receiver := newJMXMetricReceiver(params, test.config, consumertest.NewNop())
			jmxConfig, err := receiver.config.buildJMXConfig()
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
			params := receivertest.NewNopSettings(metadata.Type)
			jmxReceiver := newJMXMetricReceiver(params, test.config, consumertest.NewNop())
			otlpReceiver, err := jmxReceiver.buildOTLPReceiver()
			require.ErrorContains(t, err, test.expectedErr)
			require.Nil(t, otlpReceiver)
		})
	}
}
