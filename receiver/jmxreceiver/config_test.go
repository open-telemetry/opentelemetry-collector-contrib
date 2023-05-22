// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package jmxreceiver

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/jmxreceiver/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	initSupportedJars()
	tests := []struct {
		id          component.ID
		expected    component.Config
		expectedErr string
	}{
		{
			id:          component.NewIDWithName(metadata.Type, ""),
			expectedErr: "missing required field(s): `endpoint`, `target_system`",
			expected:    createDefaultConfig(),
		},
		{
			id: component.NewIDWithName(metadata.Type, "all"),
			expected: &Config{
				JARPath:            "testdata/fake_jmx.jar",
				Endpoint:           "myendpoint:12345",
				TargetSystem:       "jvm",
				CollectionInterval: 15 * time.Second,
				Username:           "myusername",
				Password:           "mypassword",
				LogLevel:           "trace",
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
					"testdata/fake_additional.jar",
				},
				ResourceAttributes: map[string]string{
					"one": "two",
				},
			},
		},
		{
			id:          component.NewIDWithName(metadata.Type, "missingendpoint"),
			expectedErr: "missing required field(s): `endpoint`",
			expected: &Config{
				JARPath:            "testdata/fake_jmx.jar",
				TargetSystem:       "jvm",
				CollectionInterval: 10 * time.Second,
				OTLPExporterConfig: otlpExporterConfig{
					Endpoint: "0.0.0.0:0",
					TimeoutSettings: exporterhelper.TimeoutSettings{
						Timeout: 5 * time.Second,
					},
				},
			},
		},
		{
			id:          component.NewIDWithName(metadata.Type, "missingtarget"),
			expectedErr: "missing required field(s): `target_system`",
			expected: &Config{
				JARPath:            "testdata/fake_jmx.jar",
				Endpoint:           "service:jmx:rmi:///jndi/rmi://host:12345/jmxrmi",
				CollectionInterval: 10 * time.Second,
				OTLPExporterConfig: otlpExporterConfig{
					Endpoint: "0.0.0.0:0",
					TimeoutSettings: exporterhelper.TimeoutSettings{
						Timeout: 5 * time.Second,
					},
				},
			},
		},
		{
			id:          component.NewIDWithName(metadata.Type, "invalidinterval"),
			expectedErr: "`interval` must be positive: -100ms",
			expected: &Config{
				JARPath:            "testdata/fake_jmx.jar",
				Endpoint:           "myendpoint:23456",
				TargetSystem:       "jvm",
				CollectionInterval: -100 * time.Millisecond,
				OTLPExporterConfig: otlpExporterConfig{
					Endpoint: "0.0.0.0:0",
					TimeoutSettings: exporterhelper.TimeoutSettings{
						Timeout: 5 * time.Second,
					},
				},
			},
		},
		{
			id:          component.NewIDWithName(metadata.Type, "invalidotlptimeout"),
			expectedErr: "`otlp.timeout` must be positive: -100ms",
			expected: &Config{
				JARPath:            "testdata/fake_jmx.jar",
				Endpoint:           "myendpoint:34567",
				TargetSystem:       "jvm",
				CollectionInterval: 10 * time.Second,
				OTLPExporterConfig: otlpExporterConfig{
					Endpoint: "0.0.0.0:0",
					TimeoutSettings: exporterhelper.TimeoutSettings{
						Timeout: -100 * time.Millisecond,
					},
				},
			},
		},

		{
			id: component.NewIDWithName(metadata.Type, "nonexistentjar"),
			// Error is different based on OS, which is why this is contains, not equals
			expectedErr: "invalid `jar_path`: error hashing file: open testdata/file_does_not_exist.jar:",
			expected: &Config{
				JARPath:            "testdata/file_does_not_exist.jar",
				Endpoint:           "myendpoint:23456",
				TargetSystem:       "jvm",
				CollectionInterval: 10 * time.Second,
				OTLPExporterConfig: otlpExporterConfig{
					Endpoint: "0.0.0.0:0",
					TimeoutSettings: exporterhelper.TimeoutSettings{
						Timeout: 5 * time.Second,
					},
				},
			},
		},
		{
			id:          component.NewIDWithName(metadata.Type, "invalidjar"),
			expectedErr: "invalid `jar_path`: jar hash does not match known versions",
			expected: &Config{
				JARPath:            "testdata/fake_jmx_wrong.jar",
				Endpoint:           "myendpoint:23456",
				TargetSystem:       "jvm",
				CollectionInterval: 10 * time.Second,
				OTLPExporterConfig: otlpExporterConfig{
					Endpoint: "0.0.0.0:0",
					TimeoutSettings: exporterhelper.TimeoutSettings{
						Timeout: 5 * time.Second,
					},
				},
			},
		},
		{
			id:          component.NewIDWithName(metadata.Type, "invalidloglevel"),
			expectedErr: "`log_level` must be one of 'debug', 'error', 'info', 'off', 'trace', 'warn'",
			expected: &Config{
				JARPath:            "testdata/fake_jmx.jar",
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
			},
		},
		{
			id:          component.NewIDWithName(metadata.Type, "invalidtargetsystem"),
			expectedErr: "`target_system` list may only be a subset of 'activemq', 'cassandra', 'hadoop', 'hbase', 'jetty', 'jvm', 'kafka', 'kafka-consumer', 'kafka-producer', 'solr', 'tomcat', 'wildfly'",
			expected: &Config{
				JARPath:            "testdata/fake_jmx.jar",
				Endpoint:           "myendpoint:55555",
				TargetSystem:       "jvm,fakejvmtechnology",
				CollectionInterval: 10 * time.Second,
				OTLPExporterConfig: otlpExporterConfig{
					Endpoint: "0.0.0.0:0",
					TimeoutSettings: exporterhelper.TimeoutSettings{
						Timeout: 5 * time.Second,
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			mockJarVersions()
			t.Cleanup(func() {
				unmockJarVersions()
			})

			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, component.UnmarshalConfig(sub, cfg))

			if tt.expectedErr != "" {
				assert.ErrorContains(t, cfg.(*Config).Validate(), tt.expectedErr)
				assert.Equal(t, tt.expected, cfg)
				return
			}
			assert.NoError(t, component.ValidateConfig(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func TestCustomMetricsGathererConfig(t *testing.T) {
	wildflyJarVersions["7d1a54127b222502f5b79b5fb0803061152a44f92b37e23c6527baf665d4da9a"] = supportedJar{
		jar:     "fake wildfly jar",
		version: "2.3.4",
	}

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "invalidtargetsystem").String())
	require.NoError(t, err)
	require.NoError(t, component.UnmarshalConfig(sub, cfg))

	conf := cfg.(*Config)

	err = conf.Validate()
	require.Error(t, err)
	assert.Equal(t, "invalid `jar_path`: jar hash does not match known versions", err.Error())

	MetricsGathererHash = "5994471abb01112afcc18159f6cc74b4f511b99806da59b3caf5a9c173cacfc5"
	initSupportedJars()

	err = conf.Validate()
	require.Error(t, err)
	assert.Equal(t, "`target_system` list may only be a subset of 'activemq', 'cassandra', 'hadoop', 'hbase', 'jetty', 'jvm', 'kafka', 'kafka-consumer', 'kafka-producer', 'solr', 'tomcat', 'wildfly'", err.Error())

	AdditionalTargetSystems = "fakejvmtechnology,anothertechnology"
	t.Cleanup(func() {
		delete(validTargetSystems, "fakejvmtechnology")
		delete(validTargetSystems, "anothertechnology")
	})
	initAdditionalTargetSystems()

	conf.TargetSystem = "jvm,fakejvmtechnology,anothertechnology"

	require.NoError(t, conf.Validate())
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
				JARPath: "testdata/fake_jmx.jar",
			},
			existingEnvVal: "",
			expected:       "testdata/fake_jmx.jar",
		},
		{
			desc: "Additional JARS",
			cfg: &Config{
				JARPath: "testdata/fake_jmx.jar",
				AdditionalJars: []string{
					"/path/to/one.jar",
					"/path/to/two.jar",
				},
			},
			existingEnvVal: "",
			expected:       "testdata/fake_jmx.jar:/path/to/one.jar:/path/to/two.jar",
		},
		{
			desc: "Existing ENV Value",
			cfg: &Config{
				JARPath: "testdata/fake_jmx.jar",
				AdditionalJars: []string{
					"/path/to/one.jar",
					"/path/to/two.jar",
				},
			},
			existingEnvVal: "/pre/existing/class/path/",
			expected:       "testdata/fake_jmx.jar:/path/to/one.jar:/path/to/two.jar",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			t.Setenv("CLASSPATH", tc.existingEnvVal)

			actual := tc.cfg.parseClasspath()
			require.Equal(t, tc.expected, actual)
		})
	}
}

func TestWithInvalidConfig(t *testing.T) {
	f := NewFactory()
	assert.Equal(t, component.Type("jmx"), f.Type())

	cfg := f.CreateDefaultConfig().(*Config)
	require.NotNil(t, cfg)

	err := cfg.Validate()
	assert.Equal(t, "missing required field(s): `endpoint`, `target_system`", err.Error())
}

func mockJarVersions() {
	jmxMetricsGathererVersions["5994471abb01112afcc18159f6cc74b4f511b99806da59b3caf5a9c173cacfc5"] = supportedJar{
		jar:     "fake jar",
		version: "1.2.3",
	}

	wildflyJarVersions["7d1a54127b222502f5b79b5fb0803061152a44f92b37e23c6527baf665d4da9a"] = supportedJar{
		jar:     "fake wildfly jar",
		version: "2.3.4",
	}
}

func unmockJarVersions() {
	delete(jmxMetricsGathererVersions, "5994471abb01112afcc18159f6cc74b4f511b99806da59b3caf5a9c173cacfc5")
	delete(wildflyJarVersions, "7d1a54127b222502f5b79b5fb0803061152a44f92b37e23c6527baf665d4da9a")
}
