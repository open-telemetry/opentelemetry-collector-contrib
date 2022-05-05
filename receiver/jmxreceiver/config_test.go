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
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configtest"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/service/servicetest"
)

func TestLoadConfig(t *testing.T) {
	factories, err := componenttest.NopFactories()
	assert.Nil(t, err)

	factory := NewFactory()
	factories.Receivers[typeStr] = factory
	cfg, err := servicetest.LoadConfigAndValidate(filepath.Join("testdata", "config.yaml"), factories)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, len(cfg.Receivers), 8)

	r0 := cfg.Receivers[config.NewComponentID(typeStr)].(*Config)
	require.NoError(t, configtest.CheckConfigStruct(r0))
	assert.Equal(t, r0, factory.CreateDefaultConfig())
	err = r0.validate()
	require.Error(t, err)
	assert.Equal(t, "jmx missing required fields: `endpoint`, `target_system`", err.Error())

	r1 := cfg.Receivers[config.NewComponentIDWithName(typeStr, "all")].(*Config)
	require.NoError(t, configtest.CheckConfigStruct(r1))
	require.NoError(t, r1.validate())
	assert.Equal(t,
		&Config{
			ReceiverSettings:   config.NewReceiverSettings(config.NewComponentIDWithName(typeStr, "all")),
			JARPath:            "myjarpath",
			Endpoint:           "myendpoint:12345",
			TargetSystem:       "jvm",
			CollectionInterval: 15 * time.Second,
			Username:           "myusername",
			Password:           "mypassword",
			LogLevel:           "info",
			OTLPExporterConfig: otlpExporterConfig{
				Endpoint: "myotlpendpoint",
				Headers: map[string]string{
					"x-header-1": "value1",
					"x-header-2": "value2",
				},
				TimeoutSettings: exporterhelper.TimeoutSettings{
					Timeout: 5 * time.Second,
				},
			},
			KeystorePath:       "mykeystorepath",
			KeystorePassword:   "mykeystorepassword",
			KeystoreType:       "mykeystoretype",
			TruststorePath:     "mytruststorepath",
			TruststorePassword: "mytruststorepassword",
			RemoteProfile:      "myremoteprofile",
			Realm:              "myrealm",
			AdditionalJars: []string{
				"/path/to/additional.jar",
			},
			ResourceAttributes: map[string]string{
				"one": "two",
			},
		}, r1)

	assert.Equal(
		t, []string{"-Dorg.slf4j.simpleLogger.defaultLogLevel=info"},
		r1.parseProperties(),
	)

	r2 := cfg.Receivers[config.NewComponentIDWithName(typeStr, "missingendpoint")].(*Config)
	require.NoError(t, configtest.CheckConfigStruct(r2))
	assert.Equal(t,
		&Config{
			ReceiverSettings:   config.NewReceiverSettings(config.NewComponentIDWithName(typeStr, "missingendpoint")),
			JARPath:            "/opt/opentelemetry-java-contrib-jmx-metrics.jar",
			TargetSystem:       "jvm",
			LogLevel:           "info",
			CollectionInterval: 10 * time.Second,
			OTLPExporterConfig: otlpExporterConfig{
				Endpoint: "0.0.0.0:0",
				TimeoutSettings: exporterhelper.TimeoutSettings{
					Timeout: 5 * time.Second,
				},
			},
		}, r2)
	err = r2.validate()
	require.Error(t, err)
	assert.Equal(t, "jmx/missingendpoint missing required field: `endpoint`", err.Error())

	r3 := cfg.Receivers[config.NewComponentIDWithName(typeStr, "missinggroovy")].(*Config)
	require.NoError(t, configtest.CheckConfigStruct(r3))
	assert.Equal(t,
		&Config{
			ReceiverSettings:   config.NewReceiverSettings(config.NewComponentIDWithName(typeStr, "missinggroovy")),
			JARPath:            "/opt/opentelemetry-java-contrib-jmx-metrics.jar",
			Endpoint:           "service:jmx:rmi:///jndi/rmi://host:12345/jmxrmi",
			CollectionInterval: 10 * time.Second,
			LogLevel:           "info",
			OTLPExporterConfig: otlpExporterConfig{
				Endpoint: "0.0.0.0:0",
				TimeoutSettings: exporterhelper.TimeoutSettings{
					Timeout: 5 * time.Second,
				},
			},
		}, r3)
	err = r3.validate()
	require.Error(t, err)
	assert.Equal(t, "jmx/missinggroovy missing required field: `target_system`", err.Error())

	r4 := cfg.Receivers[config.NewComponentIDWithName(typeStr, "invalidinterval")].(*Config)
	require.NoError(t, configtest.CheckConfigStruct(r4))
	assert.Equal(t,
		&Config{
			ReceiverSettings:   config.NewReceiverSettings(config.NewComponentIDWithName(typeStr, "invalidinterval")),
			JARPath:            "/opt/opentelemetry-java-contrib-jmx-metrics.jar",
			Endpoint:           "myendpoint:23456",
			TargetSystem:       "jvm",
			LogLevel:           "info",
			CollectionInterval: -100 * time.Millisecond,
			OTLPExporterConfig: otlpExporterConfig{
				Endpoint: "0.0.0.0:0",
				TimeoutSettings: exporterhelper.TimeoutSettings{
					Timeout: 5 * time.Second,
				},
			},
		}, r4)
	err = r4.validate()
	require.Error(t, err)
	assert.Equal(t, "jmx/invalidinterval `interval` must be positive: -100ms", err.Error())

	r5 := cfg.Receivers[config.NewComponentIDWithName(typeStr, "invalidotlptimeout")].(*Config)
	require.NoError(t, configtest.CheckConfigStruct(r5))
	assert.Equal(t,
		&Config{
			ReceiverSettings:   config.NewReceiverSettings(config.NewComponentIDWithName(typeStr, "invalidotlptimeout")),
			JARPath:            "/opt/opentelemetry-java-contrib-jmx-metrics.jar",
			Endpoint:           "myendpoint:34567",
			TargetSystem:       "jvm",
			LogLevel:           "info",
			CollectionInterval: 10 * time.Second,
			OTLPExporterConfig: otlpExporterConfig{
				Endpoint: "0.0.0.0:0",
				TimeoutSettings: exporterhelper.TimeoutSettings{
					Timeout: -100 * time.Millisecond,
				},
			},
		}, r5)
	err = r5.validate()
	require.Error(t, err)
	assert.Equal(t, "jmx/invalidotlptimeout `otlp.timeout` must be positive: -100ms", err.Error())

	r6 := cfg.Receivers[config.NewComponentIDWithName(typeStr, "invalidloglevel")].(*Config)
	require.NoError(t, configtest.CheckConfigStruct(r6))
	assert.Equal(t,
		&Config{
			ReceiverSettings:   config.NewReceiverSettings(config.NewComponentIDWithName(typeStr, "invalidloglevel")),
			JARPath:            "/opt/opentelemetry-java-contrib-jmx-metrics.jar",
			Endpoint:           "myendpoint:55555",
			TargetSystem:       "jvm",
			LogLevel:           "truth",
			CollectionInterval: 10 * time.Second,
			OTLPExporterConfig: otlpExporterConfig{
				Endpoint: "0.0.0.0:0",
				TimeoutSettings: exporterhelper.TimeoutSettings{
					Timeout: 5 * time.Second,
				},
			},
		}, r6)
	err = r6.validate()
	require.Error(t, err)
	assert.Equal(t, "jmx/invalidloglevel `log_level` must be one of 'debug', 'error', 'info', 'off', 'trace', 'warn'", err.Error())

	r7 := cfg.Receivers[config.NewComponentIDWithName(typeStr, "invalidtargetsystem")].(*Config)
	require.NoError(t, configtest.CheckConfigStruct(r7))
	assert.Equal(t,
		&Config{
			ReceiverSettings:   config.NewReceiverSettings(config.NewComponentIDWithName(typeStr, "invalidtargetsystem")),
			JARPath:            "/opt/opentelemetry-java-contrib-jmx-metrics.jar",
			Endpoint:           "myendpoint:55555",
			TargetSystem:       "jvm,nonsense",
			LogLevel:           "info",
			CollectionInterval: 10 * time.Second,
			OTLPExporterConfig: otlpExporterConfig{
				Endpoint: "0.0.0.0:0",
				TimeoutSettings: exporterhelper.TimeoutSettings{
					Timeout: 5 * time.Second,
				},
			},
		}, r7)
	err = r7.validate()
	require.Error(t, err)
	assert.Equal(t, "jmx/invalidtargetsystem `target_system` list may only be a subset of 'activemq', 'cassandra', 'hadoop', 'hbase', 'jvm', 'kafka', 'kafka-consumer', 'kafka-producer', 'solr', 'tomcat', 'wildfly'", err.Error())
}

func TestClassPathParse(t *testing.T) {
	testCases := []struct {
		desc           string
		cfg            *Config
		existingEnvVal string
		expected       string
	}{
		{
			desc: "Metric Gatherer JAR Only",
			cfg: &Config{
				JARPath: "/opt/opentelemetry-java-contrib-jmx-metrics.jar",
			},
			existingEnvVal: "",
			expected:       "/opt/opentelemetry-java-contrib-jmx-metrics.jar",
		},
		{
			desc: "Additional JARS",
			cfg: &Config{
				JARPath: "/opt/opentelemetry-java-contrib-jmx-metrics.jar",
				AdditionalJars: []string{
					"/path/to/one.jar",
					"/path/to/two.jar",
				},
			},
			existingEnvVal: "",
			expected:       "/opt/opentelemetry-java-contrib-jmx-metrics.jar:/path/to/one.jar:/path/to/two.jar",
		},
		{
			desc: "Existing ENV Value",
			cfg: &Config{
				JARPath: "/opt/opentelemetry-java-contrib-jmx-metrics.jar",
				AdditionalJars: []string{
					"/path/to/one.jar",
					"/path/to/two.jar",
				},
			},
			existingEnvVal: "/pre/existing/class/path/",
			expected:       "/opt/opentelemetry-java-contrib-jmx-metrics.jar:/path/to/one.jar:/path/to/two.jar",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			os.Unsetenv("CLASSPATH")
			err := os.Setenv("CLASSPATH", tc.existingEnvVal)
			require.NoError(t, err)

			actual := tc.cfg.parseClasspath()
			require.Equal(t, tc.expected, actual)
		})
	}
}
