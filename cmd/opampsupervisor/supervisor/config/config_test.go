// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/open-telemetry/opamp-go/protobufs"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configtls"
	"go.uber.org/zap/zapcore"
)

func simpleError(err string) func() string {
	return func() string { return err }
}

func TestValidate(t *testing.T) {
	tlsConfig := configtls.NewDefaultClientConfig()
	tlsConfig.InsecureSkipVerify = true

	testCases := []struct {
		name   string
		config Supervisor
		err    string
	}{
		{
			name: "valid config",
			config: Supervisor{
				Server: OpAMPServer{Endpoint: "wss://localhost:9090/opamp", TLS: tlsConfig},
				Agent: Agent{
					Executable:              "${file_path}",
					OrphanDetectionInterval: 5 * time.Second,
					ConfigApplyTimeout:      2 * time.Second,
					BootstrapTimeout:        5 * time.Second,
				},
				Capabilities: Capabilities{AcceptsRemoteConfig: true},
				Storage:      Storage{Directory: "/tmp/storage"},
			},
		},
		{
			name: "missing endpoint",
			config: Supervisor{
				Server: OpAMPServer{TLS: tlsConfig},
				Agent: Agent{
					Executable:              "${file_path}",
					OrphanDetectionInterval: 5 * time.Second,
					ConfigApplyTimeout:      2 * time.Second,
				},
				Capabilities: Capabilities{AcceptsRemoteConfig: true},
				Storage:      Storage{Directory: "/tmp/storage"},
			},
			err: "server::endpoint must be specified",
		},
		{
			name: "bad healthcheck port",
			config: Supervisor{
				Server: OpAMPServer{Endpoint: "wss://localhost:9090/opamp", TLS: tlsConfig},
				Agent: Agent{
					Executable:              "${file_path}",
					OrphanDetectionInterval: 5 * time.Second,
					ConfigApplyTimeout:      2 * time.Second,
					BootstrapTimeout:        5 * time.Second,
				},
				Capabilities: Capabilities{AcceptsRemoteConfig: true},
				Storage:      Storage{Directory: "/tmp/storage"},
				HealthCheck:  HealthCheck{ServerConfig: confighttp.ServerConfig{NetAddr: confignet.AddrConfig{Transport: "tcp", Endpoint: "localhost:-1"}}},
			},
			err: "healthcheck::endpoint must contain a valid port number, got -1",
		},
	}

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "file")
	require.NoError(t, os.WriteFile(filePath, []byte{}, 0o600))

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.config.Agent.Executable = os.Expand(tc.config.Agent.Executable, func(s string) string {
				if s == "file_path" {
					return filePath
				}
				return ""
			})

			err := tc.config.Validate()
			if tc.err == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tc.err)
		})
	}
}

func TestOpAMPServer_OpaqueHeaders(t *testing.T) {
	headers := http.Header{"key1": []string{"value1"}, "key2": []string{"value2a", "value2b"}}
	expected := map[string][]configopaque.String{
		"key1": {configopaque.String("value1")},
		"key2": {configopaque.String("value2a"), configopaque.String("value2b")},
	}
	require.Equal(t, expected, OpAMPServer{Headers: headers}.OpaqueHeaders())
}

func TestCapabilities_SupportedCapabilities(t *testing.T) {
	require.Equal(t,
		protobufs.AgentCapabilities_AgentCapabilities_ReportsStatus|
			protobufs.AgentCapabilities_AgentCapabilities_ReportsOwnMetrics|
			protobufs.AgentCapabilities_AgentCapabilities_ReportsEffectiveConfig|
			protobufs.AgentCapabilities_AgentCapabilities_ReportsHealth|
			protobufs.AgentCapabilities_AgentCapabilities_ReportsHeartbeat,
		DefaultSupervisor().Capabilities.SupportedCapabilities(),
	)
}

func TestLoad(t *testing.T) {
	tmpDir := t.TempDir()
	executablePath := filepath.Join(tmpDir, "binary")
	require.NoError(t, os.WriteFile(executablePath, []byte{}, 0o600))

	tests := []struct {
		name string
		cfg  string
		want Supervisor
	}{
		{
			name: "minimal",
			cfg: fmt.Sprintf(`
server:
  endpoint: ws://localhost/v1/opamp

agent:
  executable: %s
`, executablePath),
			want: Supervisor{
				Server:       OpAMPServer{Endpoint: "ws://localhost/v1/opamp"},
				Capabilities: DefaultSupervisor().Capabilities,
				Storage:      DefaultSupervisor().Storage,
				Agent: Agent{
					Executable:              executablePath,
					OrphanDetectionInterval: DefaultSupervisor().Agent.OrphanDetectionInterval,
					ConfigApplyTimeout:      DefaultSupervisor().Agent.ConfigApplyTimeout,
					BootstrapTimeout:        DefaultSupervisor().Agent.BootstrapTimeout,
					ValidateConfig:          DefaultSupervisor().Agent.ValidateConfig,
				},
				Telemetry:   DefaultSupervisor().Telemetry,
				HealthCheck: DefaultSupervisor().HealthCheck,
			},
		},
		{
			name: "full with console log format",
			cfg: fmt.Sprintf(`
server:
  endpoint: ws://localhost/v1/opamp
  tls:
    insecure: true

storage:
  directory: %s

agent:
  executable: %s

telemetry:
  logs:
    level: warn
    error_output_paths: ["stderr"]
    output_paths: ["stdout"]
    log_format: console
`, filepath.Join(tmpDir, "storage"), executablePath),
			want: Supervisor{
				Server:       OpAMPServer{Endpoint: "ws://localhost/v1/opamp", TLS: configtls.ClientConfig{Insecure: true}},
				Capabilities: DefaultSupervisor().Capabilities,
				Storage:      Storage{Directory: filepath.Join(tmpDir, "storage")},
				Agent: Agent{
					Executable:              executablePath,
					OrphanDetectionInterval: DefaultSupervisor().Agent.OrphanDetectionInterval,
					ConfigApplyTimeout:      DefaultSupervisor().Agent.ConfigApplyTimeout,
					BootstrapTimeout:        DefaultSupervisor().Agent.BootstrapTimeout,
					ValidateConfig:          DefaultSupervisor().Agent.ValidateConfig,
				},
				Telemetry:   Telemetry{Logs: Logs{Level: zapcore.WarnLevel, OutputPaths: []string{"stdout"}, ErrorOutputPaths: []string{"stderr"}, LogFormat: "console"}},
				HealthCheck: DefaultSupervisor().HealthCheck,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfgPath := filepath.Join(tmpDir, tt.name+".yaml")
			require.NoError(t, os.WriteFile(cfgPath, []byte(tt.cfg), 0o600))
			got, err := Load(cfgPath)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestLoad_DefaultsAndErrors(t *testing.T) {
	tmpDir := t.TempDir()
	executablePath := filepath.Join(tmpDir, "binary")
	require.NoError(t, os.WriteFile(executablePath, []byte{}, 0o600))

	cfg := fmt.Sprintf(`
server:
  endpoint: ws://localhost/v1/opamp

agent:
  executable: %s
`, executablePath)
	cfgPath := filepath.Join(tmpDir, "config.yaml")
	require.NoError(t, os.WriteFile(cfgPath, []byte(cfg), 0o600))
	got, err := Load(cfgPath)
	require.NoError(t, err)
	require.Equal(t, "", got.Telemetry.Logs.LogFormat)

	_, err = Load("")
	require.ErrorContains(t, err, "path to config file cannot be empty")
}

func setupSupervisorConfigFile(t *testing.T, tmpDir, configString string) string {
	t.Helper()
	cfgPath := filepath.Join(tmpDir, "config.yaml")
	require.NoError(t, os.WriteFile(cfgPath, []byte(configString), 0o600))
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
