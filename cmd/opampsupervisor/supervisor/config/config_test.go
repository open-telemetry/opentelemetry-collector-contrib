// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/open-telemetry/opamp-go/protobufs"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/configtls"
	"go.uber.org/zap/zapcore"
)

func TestValidate(t *testing.T) {
	tlsConfig := configtls.NewDefaultClientConfig()
	tlsConfig.InsecureSkipVerify = true

	testCases := []struct {
		name          string
		config        Supervisor
		expectedError string
	}{
		{
			name: "Valid filled out config",
			config: Supervisor{
				Server: OpAMPServer{
					Endpoint: "wss://localhost:9090/opamp",
					Headers: http.Header{
						"Header1": []string{"HeaderValue"},
					},
					TLSSetting: tlsConfig,
				},
				Agent: Agent{
					Executable:              "${file_path}",
					OrphanDetectionInterval: 5 * time.Second,
					ConfigApplyTimeout:      2 * time.Second,
					BootstrapTimeout:        5 * time.Second,
				},
				Capabilities: Capabilities{
					AcceptsRemoteConfig: true,
				},
				Storage: Storage{
					Directory: "/etc/opamp-supervisor/storage",
				},
			},
		},
		{
			name: "Endpoint unspecified",
			config: Supervisor{
				Server: OpAMPServer{
					Headers: http.Header{
						"Header1": []string{"HeaderValue"},
					},
					TLSSetting: tlsConfig,
				},
				Agent: Agent{
					Executable:              "${file_path}",
					ConfigApplyTimeout:      2 * time.Second,
					OrphanDetectionInterval: 5 * time.Second,
				},
				Capabilities: Capabilities{
					AcceptsRemoteConfig: true,
				},
				Storage: Storage{
					Directory: "/etc/opamp-supervisor/storage",
				},
			},
			expectedError: "server::endpoint must be specified",
		},
		{
			name: "Invalid URL",
			config: Supervisor{
				Server: OpAMPServer{
					Endpoint: "\000",
					Headers: http.Header{
						"Header1": []string{"HeaderValue"},
					},
					TLSSetting: tlsConfig,
				},
				Agent: Agent{
					Executable:              "${file_path}",
					ConfigApplyTimeout:      2 * time.Second,
					OrphanDetectionInterval: 5 * time.Second,
				},
				Capabilities: Capabilities{
					AcceptsRemoteConfig: true,
				},
				Storage: Storage{
					Directory: "/etc/opamp-supervisor/storage",
				},
			},
			expectedError: "invalid URL for server::endpoint:",
		},
		{
			name: "Invalid endpoint scheme",
			config: Supervisor{
				Server: OpAMPServer{
					Endpoint: "tcp://localhost:9090/opamp",
					Headers: http.Header{
						"Header1": []string{"HeaderValue"},
					},
					TLSSetting: tlsConfig,
				},
				Agent: Agent{
					Executable:              "${file_path}",
					ConfigApplyTimeout:      2 * time.Second,
					OrphanDetectionInterval: 5 * time.Second,
				},
				Capabilities: Capabilities{
					AcceptsRemoteConfig: true,
				},
				Storage: Storage{
					Directory: "/etc/opamp-supervisor/storage",
				},
			},
			expectedError: `invalid scheme "tcp" for server::endpoint, must be one of "http", "https", "ws", or "wss"`,
		},
		{
			name: "Invalid tls settings",
			config: Supervisor{
				Server: OpAMPServer{
					Endpoint: "wss://localhost:9090/opamp",
					Headers: http.Header{
						"Header1": []string{"HeaderValue"},
					},
					TLSSetting: configtls.ClientConfig{
						Insecure: true,
						Config: configtls.Config{
							MaxVersion: "1.2",
							MinVersion: "1.3",
						},
					},
				},
				Agent: Agent{
					Executable:              "${file_path}",
					ConfigApplyTimeout:      2 * time.Second,
					OrphanDetectionInterval: 5 * time.Second,
				},
				Capabilities: Capabilities{
					AcceptsRemoteConfig: true,
				},
				Storage: Storage{
					Directory: "/etc/opamp-supervisor/storage",
				},
			},
			expectedError: "invalid server::tls settings:",
		},
		{
			name: "Empty agent executable path",
			config: Supervisor{
				Server: OpAMPServer{
					Endpoint: "wss://localhost:9090/opamp",
					Headers: http.Header{
						"Header1": []string{"HeaderValue"},
					},
					TLSSetting: tlsConfig,
				},
				Agent: Agent{
					Executable:              "",
					OrphanDetectionInterval: 5 * time.Second,
					ConfigApplyTimeout:      2 * time.Second,
					BootstrapTimeout:        5 * time.Second,
				},
				Capabilities: Capabilities{
					AcceptsRemoteConfig: true,
				},
				Storage: Storage{
					Directory: "/etc/opamp-supervisor/storage",
				},
			},
			expectedError: "agent::executable must be specified",
		},
		{
			name: "agent executable does not exist",
			config: Supervisor{
				Server: OpAMPServer{
					Endpoint: "wss://localhost:9090/opamp",
					Headers: http.Header{
						"Header1": []string{"HeaderValue"},
					},
					TLSSetting: tlsConfig,
				},
				Agent: Agent{
					Executable:              "./path/does/not/exist",
					OrphanDetectionInterval: 5 * time.Second,
					ConfigApplyTimeout:      2 * time.Second,
					BootstrapTimeout:        5 * time.Second,
				},
				Capabilities: Capabilities{
					AcceptsRemoteConfig: true,
				},
				Storage: Storage{
					Directory: "/etc/opamp-supervisor/storage",
				},
			},
			expectedError: "could not stat agent::executable path:",
		},
		{
			name: "Invalid orphan detection interval",
			config: Supervisor{
				Server: OpAMPServer{
					Endpoint: "wss://localhost:9090/opamp",
					Headers: http.Header{
						"Header1": []string{"HeaderValue"},
					},
					TLSSetting: tlsConfig,
				},
				Agent: Agent{
					Executable:              "${file_path}",
					ConfigApplyTimeout:      2 * time.Second,
					OrphanDetectionInterval: -1,
				},
				Capabilities: Capabilities{
					AcceptsRemoteConfig: true,
				},
				Storage: Storage{
					Directory: "/etc/opamp-supervisor/storage",
				},
			},
			expectedError: "agent::orphan_detection_interval must be positive",
		},
		{
			name: "Invalid health check port number",
			config: Supervisor{
				Server: OpAMPServer{
					Endpoint: "wss://localhost:9090/opamp",
					Headers: http.Header{
						"Header1": []string{"HeaderValue"},
					},
					TLSSetting: tlsConfig,
				},
				Agent: Agent{
					Executable:              "${file_path}",
					OrphanDetectionInterval: 5 * time.Second,
					HealthCheckPort:         65536,
					ConfigApplyTimeout:      2 * time.Second,
					BootstrapTimeout:        5 * time.Second,
				},
				Capabilities: Capabilities{
					AcceptsRemoteConfig: true,
				},
				Storage: Storage{
					Directory: "/etc/opamp-supervisor/storage",
				},
			},
			expectedError: "agent::health_check_port must be a valid port number",
		},
		{
			name: "Zero value health check port number",
			config: Supervisor{
				Server: OpAMPServer{
					Endpoint: "wss://localhost:9090/opamp",
					Headers: http.Header{
						"Header1": []string{"HeaderValue"},
					},
					TLSSetting: tlsConfig,
				},
				Agent: Agent{
					Executable:              "${file_path}",
					OrphanDetectionInterval: 5 * time.Second,
					HealthCheckPort:         0,
					ConfigApplyTimeout:      2 * time.Second,
					BootstrapTimeout:        5 * time.Second,
				},
				Capabilities: Capabilities{
					AcceptsRemoteConfig: true,
				},
				Storage: Storage{
					Directory: "/etc/opamp-supervisor/storage",
				},
			},
		},
		{
			name: "Normal health check port number",
			config: Supervisor{
				Server: OpAMPServer{
					Endpoint: "wss://localhost:9090/opamp",
					Headers: http.Header{
						"Header1": []string{"HeaderValue"},
					},
					TLSSetting: tlsConfig,
				},
				Agent: Agent{
					Executable:              "${file_path}",
					OrphanDetectionInterval: 5 * time.Second,
					HealthCheckPort:         29848,
					ConfigApplyTimeout:      2 * time.Second,
					BootstrapTimeout:        5 * time.Second,
				},
				Capabilities: Capabilities{
					AcceptsRemoteConfig: true,
				},
				Storage: Storage{
					Directory: "/etc/opamp-supervisor/storage",
				},
			},
		},
		{
			name: "config with invalid agent bootstrap timeout",
			config: Supervisor{
				Server: OpAMPServer{
					Endpoint: "wss://localhost:9090/opamp",
					Headers: http.Header{
						"Header1": []string{"HeaderValue"},
					},
					TLSSetting: tlsConfig,
				},
				Agent: Agent{
					Executable:              "${file_path}",
					OrphanDetectionInterval: 5 * time.Second,
					ConfigApplyTimeout:      2 * time.Second,
					BootstrapTimeout:        -5 * time.Second,
				},
				Capabilities: Capabilities{
					AcceptsRemoteConfig: true,
				},
				Storage: Storage{
					Directory: "/etc/opamp-supervisor/storage",
				},
			},
			expectedError: "agent::bootstrap_timeout must be positive",
		},
		{
			name: "Invalid opamp server port number",
			config: Supervisor{
				Server: OpAMPServer{
					Endpoint: "wss://localhost:9090/opamp",
					Headers: http.Header{
						"Header1": []string{"HeaderValue"},
					},
				},
				Agent: Agent{
					Executable:              "${file_path}",
					OrphanDetectionInterval: 5 * time.Second,
					ConfigApplyTimeout:      2 * time.Second,
					OpAMPServerPort:         65536,
					BootstrapTimeout:        5 * time.Second,
				},
				Capabilities: Capabilities{
					AcceptsRemoteConfig: true,
				},
				Storage: Storage{
					Directory: "/etc/opamp-supervisor/storage",
				},
			},
			expectedError: "agent::opamp_server_port must be a valid port number",
		},
		{
			name: "Zero value opamp server port number",
			config: Supervisor{
				Server: OpAMPServer{
					Endpoint: "wss://localhost:9090/opamp",
					Headers: http.Header{
						"Header1": []string{"HeaderValue"},
					},
				},
				Agent: Agent{
					Executable:              "${file_path}",
					OrphanDetectionInterval: 5 * time.Second,
					ConfigApplyTimeout:      2 * time.Second,
					OpAMPServerPort:         0,
					BootstrapTimeout:        5 * time.Second,
				},
				Capabilities: Capabilities{
					AcceptsRemoteConfig: true,
				},
				Storage: Storage{
					Directory: "/etc/opamp-supervisor/storage",
				},
			},
		},
		{
			name: "Invalid config apply timeout",
			config: Supervisor{
				Server: OpAMPServer{
					Endpoint: "wss://localhost:9090/opamp",
					Headers: http.Header{
						"Header1": []string{"HeaderValue"},
					},
					TLSSetting: tlsConfig,
				},
				Agent: Agent{
					Executable:              "${file_path}",
					OrphanDetectionInterval: 5 * time.Second,
					OpAMPServerPort:         8080,
					BootstrapTimeout:        5 * time.Second,
				},
				Capabilities: Capabilities{
					AcceptsRemoteConfig: true,
				},
				Storage: Storage{
					Directory: "/etc/opamp-supervisor/storage",
				},
			},
			expectedError: "agent::config_apply_timeout must be valid duration",
		},
	}

	// create some fake files for validating agent config
	tmpDir := t.TempDir()

	filePath := filepath.Join(tmpDir, "file")
	require.NoError(t, os.WriteFile(filePath, []byte{}, 0o600))

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Fill in path to agent executable
			tc.config.Agent.Executable = os.Expand(tc.config.Agent.Executable,
				func(s string) string {
					if s == "file_path" {
						return filePath
					}
					return ""
				})

			err := tc.config.Validate()

			if tc.expectedError == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tc.expectedError)
			}
		})
	}
}

func TestCapabilities_SupportedCapabilities(t *testing.T) {
	testCases := []struct {
		name                      string
		capabilities              Capabilities
		expectedAgentCapabilities protobufs.AgentCapabilities
	}{
		{
			name:         "Default capabilities",
			capabilities: DefaultSupervisor().Capabilities,
			expectedAgentCapabilities: protobufs.AgentCapabilities_AgentCapabilities_ReportsStatus |
				protobufs.AgentCapabilities_AgentCapabilities_ReportsOwnMetrics |
				protobufs.AgentCapabilities_AgentCapabilities_ReportsEffectiveConfig |
				protobufs.AgentCapabilities_AgentCapabilities_ReportsHealth,
		},
		{
			name:                      "Empty capabilities",
			capabilities:              Capabilities{},
			expectedAgentCapabilities: protobufs.AgentCapabilities_AgentCapabilities_ReportsStatus,
		},
		{
			name: "Many capabilities",
			capabilities: Capabilities{
				AcceptsRemoteConfig:            true,
				AcceptsRestartCommand:          true,
				AcceptsOpAMPConnectionSettings: true,
				ReportsEffectiveConfig:         true,
				ReportsOwnMetrics:              true,
				ReportsOwnLogs:                 true,
				ReportsOwnTraces:               true,
				ReportsHealth:                  true,
				ReportsRemoteConfig:            true,
				ReportsAvailableComponents:     true,
			},
			expectedAgentCapabilities: protobufs.AgentCapabilities_AgentCapabilities_ReportsStatus |
				protobufs.AgentCapabilities_AgentCapabilities_ReportsEffectiveConfig |
				protobufs.AgentCapabilities_AgentCapabilities_ReportsHealth |
				protobufs.AgentCapabilities_AgentCapabilities_ReportsOwnMetrics |
				protobufs.AgentCapabilities_AgentCapabilities_ReportsOwnLogs |
				protobufs.AgentCapabilities_AgentCapabilities_ReportsOwnTraces |
				protobufs.AgentCapabilities_AgentCapabilities_AcceptsRemoteConfig |
				protobufs.AgentCapabilities_AgentCapabilities_ReportsRemoteConfig |
				protobufs.AgentCapabilities_AgentCapabilities_AcceptsRestartCommand |
				protobufs.AgentCapabilities_AgentCapabilities_AcceptsOpAMPConnectionSettings |
				protobufs.AgentCapabilities_AgentCapabilities_ReportsAvailableComponents,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.expectedAgentCapabilities, tc.capabilities.SupportedCapabilities())
		})
	}
}

func TestLoad(t *testing.T) {
	tmpDir := t.TempDir()

	t.Cleanup(func() {
		require.NoError(t, os.Chmod(tmpDir, 0o700))
		require.NoError(t, os.RemoveAll(tmpDir))
	})

	executablePath := filepath.Join(tmpDir, "binary")
	err := os.WriteFile(executablePath, []byte{}, 0o600)
	require.NoError(t, err)

	testCases := []struct {
		desc     string
		testFunc func(t *testing.T)
	}{
		{
			desc: "Minimal Config Supervisor",
			testFunc: func(t *testing.T) {
				config := `
server:
  endpoint: ws://localhost/v1/opamp

agent:
  executable: %s
`
				config = fmt.Sprintf(config, executablePath)

				expected := Supervisor{
					Server: OpAMPServer{
						Endpoint: "ws://localhost/v1/opamp",
					},
					Capabilities: DefaultSupervisor().Capabilities,
					Storage:      DefaultSupervisor().Storage,
					Agent: Agent{
						Executable:              executablePath,
						OrphanDetectionInterval: DefaultSupervisor().Agent.OrphanDetectionInterval,
						ConfigApplyTimeout:      DefaultSupervisor().Agent.ConfigApplyTimeout,
						BootstrapTimeout:        DefaultSupervisor().Agent.BootstrapTimeout,
					},
					Telemetry: DefaultSupervisor().Telemetry,
				}

				cfgPath := setupSupervisorConfigFile(t, tmpDir, config)
				runSupervisorConfigLoadTest(t, cfgPath, expected, nil)
			},
		},
		{
			desc: "Full Config Supervisor",
			testFunc: func(t *testing.T) {
				config := `
server:
  endpoint: ws://localhost/v1/opamp
  tls:
    insecure: true

capabilities:
  reports_effective_config: false
  reports_own_metrics: false
  reports_health: false
  accepts_remote_config: true
  reports_remote_config: true
  accepts_restart_command: true
  accepts_opamp_connection_settings: true

storage:
  directory: %s

agent:
  executable: %s
  description:
    identifying_attributes:
      "service.name": "io.opentelemetry.collector"
    non_identifying_attributes:
      "os.type": darwin
  orphan_detection_interval: 10s
  config_apply_timeout: 8s
  bootstrap_timeout: 8s
  health_check_port: 8089
  opamp_server_port: 8090
  passthrough_logs: true

telemetry:
  logs:
    level: warn
    output_paths: ["stdout"]
`
				config = fmt.Sprintf(config, filepath.Join(tmpDir, "storage"), executablePath)

				expected := Supervisor{
					Server: OpAMPServer{
						Endpoint: "ws://localhost/v1/opamp",
						TLSSetting: configtls.ClientConfig{
							Insecure: true,
						},
					},
					Capabilities: Capabilities{
						ReportsEffectiveConfig:         false,
						ReportsOwnMetrics:              false,
						ReportsOwnLogs:                 false,
						ReportsOwnTraces:               false,
						ReportsHealth:                  false,
						AcceptsRemoteConfig:            true,
						ReportsRemoteConfig:            true,
						AcceptsRestartCommand:          true,
						AcceptsOpAMPConnectionSettings: true,
					},
					Storage: Storage{
						Directory: filepath.Join(tmpDir, "storage"),
					},
					Agent: Agent{
						Executable: executablePath,
						Description: AgentDescription{
							IdentifyingAttributes: map[string]string{
								"service.name": "io.opentelemetry.collector",
							},
							NonIdentifyingAttributes: map[string]string{
								"os.type": "darwin",
							},
						},
						OrphanDetectionInterval: 10 * time.Second,
						ConfigApplyTimeout:      8 * time.Second,
						BootstrapTimeout:        8 * time.Second,
						HealthCheckPort:         8089,
						OpAMPServerPort:         8090,
						PassthroughLogs:         true,
					},
					Telemetry: Telemetry{
						Logs: Logs{
							Level:       zapcore.WarnLevel,
							OutputPaths: []string{"stdout"},
						},
					},
				}

				cfgPath := setupSupervisorConfigFile(t, tmpDir, config)
				runSupervisorConfigLoadTest(t, cfgPath, expected, nil)
			},
		},
		{
			desc: "Environment Variable Config Supervisor",
			testFunc: func(t *testing.T) {
				config := `
server:
  endpoint: ${TEST_ENDPOINT}

agent:
  executable: ${TEST_EXECUTABLE_PATH}
`
				expected := Supervisor{
					Server: OpAMPServer{
						Endpoint: "ws://localhost/v1/opamp",
					},
					Capabilities: DefaultSupervisor().Capabilities,
					Storage:      DefaultSupervisor().Storage,
					Agent: Agent{
						Executable:              executablePath,
						OrphanDetectionInterval: DefaultSupervisor().Agent.OrphanDetectionInterval,
						ConfigApplyTimeout:      DefaultSupervisor().Agent.ConfigApplyTimeout,
						BootstrapTimeout:        DefaultSupervisor().Agent.BootstrapTimeout,
					},
					Telemetry: DefaultSupervisor().Telemetry,
				}

				t.Setenv("TEST_ENDPOINT", "ws://localhost/v1/opamp")
				t.Setenv("TEST_EXECUTABLE_PATH", executablePath)

				cfgPath := setupSupervisorConfigFile(t, tmpDir, config)
				runSupervisorConfigLoadTest(t, cfgPath, expected, nil)
			},
		},
		{
			desc: "Empty Config Filepath",
			testFunc: func(t *testing.T) {
				runSupervisorConfigLoadTest(t, "", Supervisor{}, errors.New("path to config file cannot be empty"))
			},
		},
		{
			desc: "Nonexistent Config File",
			testFunc: func(t *testing.T) {
				config := `
server:
  endpoint: ws://localhost/v1/opamp

agent:
  executable: %s
`
				config = fmt.Sprintf(config, executablePath)

				cfgPath := setupSupervisorConfigFile(t, tmpDir, config)
				require.NoError(t, os.Remove(cfgPath))
				runSupervisorConfigLoadTest(t, cfgPath, Supervisor{}, errors.New("cannot retrieve the configuration: unable to read the file"))
			},
		},
		{
			desc: "Failed Validation Supervisor",
			testFunc: func(t *testing.T) {
				config := `
server:

agent:
  executable: %s
`
				config = fmt.Sprintf(config, executablePath)
				cfgPath := setupSupervisorConfigFile(t, tmpDir, config)
				runSupervisorConfigLoadTest(t, cfgPath, Supervisor{}, errors.New("cannot validate supervisor config"))
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, tc.testFunc)
	}
}

func setupSupervisorConfigFile(t *testing.T, tmpDir, configString string) string {
	t.Helper()

	cfgPath := filepath.Join(tmpDir, "config.yaml")
	err := os.WriteFile(cfgPath, []byte(configString), 0o600)
	require.NoError(t, err)
	return cfgPath
}

func runSupervisorConfigLoadTest(t *testing.T, cfgPath string, expected Supervisor, expectedErr error) {
	t.Helper()

	cfg, err := Load(cfgPath)
	if expectedErr != nil {
		require.ErrorContains(t, err, expectedErr.Error())
		return
	}
	require.NoError(t, err)
	require.Equal(t, expected, cfg)
}
