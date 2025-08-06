// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package jmxreceiver

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"
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
					TimeoutSettings: exporterhelper.TimeoutConfig{
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
					TimeoutSettings: exporterhelper.TimeoutConfig{
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
					TimeoutSettings: exporterhelper.TimeoutConfig{
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
					TimeoutSettings: exporterhelper.TimeoutConfig{
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
					TimeoutSettings: exporterhelper.TimeoutConfig{
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
					TimeoutSettings: exporterhelper.TimeoutConfig{
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
					TimeoutSettings: exporterhelper.TimeoutConfig{
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
					TimeoutSettings: exporterhelper.TimeoutConfig{
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
					TimeoutSettings: exporterhelper.TimeoutConfig{
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
			require.NoError(t, sub.Unmarshal(cfg))

			if tt.expectedErr != "" {
				assert.ErrorContains(t, cfg.(*Config).Validate(), tt.expectedErr)
				assert.Equal(t, tt.expected, cfg)
				return
			}
			assert.NoError(t, xconfmap.Validate(cfg))
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
	require.NoError(t, sub.Unmarshal(cfg))

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

func TestPasswordFileValidation(t *testing.T) {
	testCases := []struct {
		desc          string
		cfg           *Config
		passwordMap   map[string]string
		expectedError error
	}{
		{
			desc:          "no validation for default",
			cfg:           &Config{},
			passwordMap:   map[string]string{},
			expectedError: nil,
		},
		{
			desc: "all passwords set in file",
			cfg: &Config{
				Username:       "test-user",
				TruststorePath: "trust-store-path",
				KeystorePath:   "key-store-path",
			},
			passwordMap: map[string]string{
				"test-user":        "XXXXXXX",
				roleNameKeyStore:   "XXXXXXX",
				roleNameTrustStore: "XXXXXXX",
			},
			expectedError: nil,
		},
		{
			desc: "all passwords set in config",
			cfg: &Config{
				Username:           "test-user",
				Password:           "XXXXXXXX",
				TruststorePath:     "trust-store-path",
				TruststorePassword: "XXXXXXX",
				KeystorePath:       "key-store-path",
				KeystorePassword:   "XXXXXXXX",
			},
			passwordMap:   map[string]string{},
			expectedError: nil,
		},
		{
			desc: "user password is missing",
			cfg: &Config{
				Username:       "test-user-wrong",
				TruststorePath: "trust-store-path",
				KeystorePath:   "key-store-path",
			},
			passwordMap: map[string]string{
				"test-user":        "XXXXXXX",
				roleNameKeyStore:   "XXXXXXX",
				roleNameTrustStore: "XXXXXXX",
			},
			expectedError: fmt.Errorf("password for `username` (%s) not found in `password_file`", "test-user-wrong"),
		},
		{
			desc: "key store password is missing",
			cfg: &Config{
				Username:       "test-user",
				TruststorePath: "trust-store-path",
				KeystorePath:   "key-store-path",
			},
			passwordMap: map[string]string{
				"test-user":        "XXXXXXX",
				roleNameTrustStore: "XXXXXXX",
			},
			expectedError: fmt.Errorf("password for %s not found in `password_file`", roleNameKeyStore),
		},
		{
			desc: "trust store password is missing",
			cfg: &Config{
				Username:       "test-user",
				TruststorePath: "trust-store-path",
				KeystorePath:   "key-store-path",
			},
			passwordMap: map[string]string{
				"test-user":      "XXXXXXX",
				roleNameKeyStore: "XXXXXXX",
			},
			expectedError: fmt.Errorf("password for %s not found in `password_file`", roleNameTrustStore),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			actual := tc.cfg.validatePasswords(tc.passwordMap)
			require.Equal(t, tc.expectedError, actual)
		})
	}
}

func TestPasswordFilePermissions(t *testing.T) {
	// Create a temporary directory for test files
	tempDir := t.TempDir()

	testCases := []struct {
		desc           string
		setupFile      func(t *testing.T) string
		expectedError  string
		skipOnPlatform string // Skip test on specific platforms
	}{
		{
			desc: "nonexistent file",
			setupFile: func(_ *testing.T) string {
				return filepath.Join(tempDir, "nonexistent.properties")
			},
			expectedError: "`password_file` is inaccessible:",
		},
		{
			desc: "file with valid permissions (0400)",
			setupFile: func(t *testing.T) string {
				filePath := filepath.Join(tempDir, "readonly.properties")
				content := "user1=password1\nuser2=password2\n"
				err := os.WriteFile(filePath, []byte(content), 0o400)
				require.NoError(t, err)
				return filePath
			},
			expectedError: "", // Should pass on all platforms
		},
		{
			desc: "file with valid permissions (0600)",
			setupFile: func(t *testing.T) string {
				filePath := filepath.Join(tempDir, "valid.properties")
				content := "user1=password1\nuser2=password2\n"
				err := os.WriteFile(filePath, []byte(content), 0o600)
				require.NoError(t, err)
				return filePath
			},
			expectedError: "", // Should pass on all platforms
		},
		{
			desc: "file with invalid permissions (0644)",
			setupFile: func(t *testing.T) string {
				filePath := filepath.Join(tempDir, "readable.properties")
				content := "user1=password1\nuser2=password2\n"
				// #nosec G306 -- This test intentionally creates a file with 0644 permissions to verify validation logic
				err := os.WriteFile(filePath, []byte(content), 0o644)
				require.NoError(t, err)
				return filePath
			},
			expectedError: "`password_file` read access must be restricted to owner-only:",
		},
		{
			desc: "file with invalid permissions (0000)",
			setupFile: func(t *testing.T) string {
				filePath := filepath.Join(tempDir, "unreadable.properties")
				content := "user1=password1\nuser2=password2\n"
				err := os.WriteFile(filePath, []byte(content), 0o000)
				require.NoError(t, err)
				return filePath
			},
			expectedError:  "`password_file` read access must be restricted to owner-only:",
			skipOnPlatform: "windows", // Windows file permissions work differently
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			if tc.skipOnPlatform != "" && runtime.GOOS == tc.skipOnPlatform {
				t.Skipf("Skipping test on %s", tc.skipOnPlatform)
			}

			filePath := tc.setupFile(t)
			cfg := &Config{
				PasswordFile: filePath,
			}

			err := cfg.validatePasswordFilePermissions()

			if tc.expectedError == "" {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedError)
			}
		})
	}
}

func TestPasswordFileIntegration(t *testing.T) {
	// Test the full validation flow including file permissions and password parsing
	tempDir := t.TempDir()

	// Create a valid password file
	passwordFile := filepath.Join(tempDir, "passwords.properties")
	content := "testuser=testpass\nkeystore=keystorepass\ntruststore=truststorepass\n"
	err := os.WriteFile(passwordFile, []byte(content), 0o600)
	require.NoError(t, err)

	cfg := &Config{
		JARPath:        "testdata/fake_jmx.jar",
		Endpoint:       "localhost:9999",
		TargetSystem:   "jvm",
		Username:       "testuser",
		PasswordFile:   passwordFile,
		KeystorePath:   "/path/to/keystore",
		TruststorePath: "/path/to/truststore",
	}

	// Mock the jar versions for validation
	mockJarVersions()
	t.Cleanup(func() {
		unmockJarVersions()
	})

	// This should pass - file exists, is readable, and contains required passwords
	err = cfg.Validate()
	assert.NoError(t, err)
}

func TestWithInvalidConfig(t *testing.T) {
	f := NewFactory()
	assert.Equal(t, metadata.Type, f.Type())

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
