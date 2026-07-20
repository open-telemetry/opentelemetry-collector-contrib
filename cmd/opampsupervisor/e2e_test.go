// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build e2e

package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"text/template"
	"time"

	"github.com/google/uuid"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/providers/rawbytes"
	"github.com/knadh/koanf/v2"
	clientTypes "github.com/open-telemetry/opamp-go/client/types"
	"github.com/open-telemetry/opamp-go/protobufs"
	"github.com/open-telemetry/opamp-go/server"
	"github.com/open-telemetry/opamp-go/server/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/protobuf/proto"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor"
	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor/config"
	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor/telemetry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
)

// getTestModes returns the test modes for the supervisor tests.
// On Windows, HUPReload mode is excluded as SIGHUP signals are not available.
func getTestModes() []struct {
	name               string
	UseHUPConfigReload bool
} {
	modes := []struct {
		name               string
		UseHUPConfigReload bool
	}{
		{
			name:               "ProcessRestart",
			UseHUPConfigReload: false,
		},
	}

	// Only add HUPReload mode on non-Windows platforms
	if runtime.GOOS != "windows" {
		modes = append(modes, struct {
			name               string
			UseHUPConfigReload bool
		}{
			name:               "HUPReload",
			UseHUPConfigReload: true,
		})
	}

	return modes
}

var _ clientTypes.Logger = testLogger{}

type testLogger struct {
	t *testing.T
}

func (tl testLogger) Debugf(_ context.Context, format string, args ...any) {
	tl.t.Logf(format, args...)
}

func (tl testLogger) Errorf(_ context.Context, format string, args ...any) {
	tl.t.Logf(format, args...)
}

func defaultConnectingHandler(connectionCallbacks types.ConnectionCallbacks) func(request *http.Request) types.ConnectionResponse {
	return func(*http.Request) types.ConnectionResponse {
		return types.ConnectionResponse{
			Accept:              true,
			ConnectionCallbacks: connectionCallbacks,
		}
	}
}

func getAgentLogs(t *testing.T, storageDir string) string {
	agentLogFile := filepath.Join(storageDir, "agent.log")
	agentLog, err := os.ReadFile(agentLogFile)
	require.NoError(t, err)
	return string(agentLog)
}

// onConnectingFuncFactory is a function that will be given to types.ConnectionCallbacks as
// OnConnectingFunc. This allows changing the ConnectionCallbacks both from the newOpAMPServer
// caller and inside of newOpAMP Server, and for custom implementations of the value for `Accept`
// in types.ConnectionResponse.
type onConnectingFuncFactory func(connectionCallbacks types.ConnectionCallbacks) func(request *http.Request) types.ConnectionResponse

type testingOpAMPServer struct {
	addr                string
	supervisorConnected chan bool
	sendToSupervisor    func(*protobufs.ServerToAgent)
	disconnectAgent     func() error
	start               func()
	shutdown            func()
}

func newOpAMPServer(t *testing.T, connectingCallback onConnectingFuncFactory, callbacks types.ConnectionCallbacks) *testingOpAMPServer {
	s := newUnstartedOpAMPServer(t, connectingCallback, callbacks)
	s.start()
	return s
}

func newUnstartedOpAMPServer(t *testing.T, connectingCallback onConnectingFuncFactory, callbacks types.ConnectionCallbacks) *testingOpAMPServer {
	var agentConn atomic.Value
	var isAgentConnected atomic.Bool
	var didShutdown atomic.Bool
	connectedChan := make(chan bool)
	s := server.New(testLogger{t: t})
	onConnectedFunc := callbacks.OnConnected
	callbacks.OnConnected = func(ctx context.Context, conn types.Connection) {
		if didShutdown.Load() {
			return
		}
		if onConnectedFunc != nil {
			onConnectedFunc(ctx, conn)
		}
		agentConn.Store(conn)
		isAgentConnected.Store(true)
		connectedChan <- true
	}
	onConnectionCloseFunc := callbacks.OnConnectionClose
	callbacks.OnConnectionClose = func(conn types.Connection) {
		if didShutdown.Load() {
			return
		}
		isAgentConnected.Store(false)
		connectedChan <- false
		if onConnectionCloseFunc != nil {
			onConnectionCloseFunc(conn)
		}
	}
	handler, connContext, err := s.Attach(server.Settings{
		Callbacks: types.Callbacks{
			OnConnecting: connectingCallback(callbacks),
		},
	})
	require.NoError(t, err)
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/opamp", handler)
	httpSrv := httptest.NewUnstartedServer(mux)
	httpSrv.Config.ConnContext = connContext

	shutdown := func() {
		if !didShutdown.Load() {
			waitForSupervisorConnection(connectedChan, false)
			t.Log("Shutting down")
			err := s.Stop(t.Context())
			assert.NoError(t, err)
			httpSrv.Close()
			// Ensure that the connectedChan is drained and closed.
			select {
			case <-connectedChan:
			default:
			}
			close(connectedChan)
		}
		didShutdown.Store(true)
	}

	send := func(msg *protobufs.ServerToAgent) {
		if !isAgentConnected.Load() {
			require.Fail(t, "Agent connection has not been established")
		}
		err = agentConn.Load().(types.Connection).Send(t.Context(), msg)
		require.NoError(t, err)
	}

	disconnectAgent := func() error {
		if !isAgentConnected.Load() {
			return errors.New("agent connection has not been established")
		}
		return agentConn.Load().(types.Connection).Disconnect()
	}

	t.Cleanup(func() {
		shutdown()
	})

	return &testingOpAMPServer{
		addr:                httpSrv.Listener.Addr().String(),
		supervisorConnected: connectedChan,
		sendToSupervisor:    send,
		disconnectAgent:     disconnectAgent,
		start:               httpSrv.Start,
		shutdown:            shutdown,
	}
}

// createHealthCheckCollectorConfWithPort creates a collector config with a healthcheck on the specified port.
// Returns the config bytes and hash.
func createHealthCheckCollectorConfWithPort(t *testing.T, port string) (*bytes.Buffer, []byte) {
	cfg := fmt.Sprintf(`
receivers:
  nop:

exporters:
  nop:

extensions:
  health_check:
    endpoint: "localhost:%s"

service:
  extensions: [health_check]
  pipelines:
    logs:
      receivers: [nop]
      exporters: [nop]
`, port)

	h := sha256.Sum256([]byte(cfg))
	return bytes.NewBufferString(cfg), h[:]
}

// createHealthCheckCollectorConfFile creates a collector config file with a healthcheck on the specified port.
func createHealthCheckCollectorConfFile(t *testing.T, port string) string {
	cfg, _ := createHealthCheckCollectorConfWithPort(t, port)
	cfgFile, err := os.CreateTemp(t.TempDir(), "healthcheck_config_*.yaml")
	require.NoError(t, err)
	t.Cleanup(func() { cfgFile.Close() })

	_, err = cfgFile.Write(cfg.Bytes())
	require.NoError(t, err)

	return cfgFile.Name()
}

func newSupervisor(t *testing.T, configType string, extraConfigData map[string]string) (*supervisor.Supervisor, *config.Supervisor) {
	cfgFile := getSupervisorConfig(t, configType, extraConfigData)
	return newSupervisorFromConfigFile(t, cfgFile.Name())
}

func newSupervisorFromConfigFile(t *testing.T, path string) (*supervisor.Supervisor, *config.Supervisor) {
	cfg, err := config.Load(path)
	require.NoError(t, err)

	logger, err := zap.NewDevelopment()
	require.NoError(t, err)

	s, err := supervisor.NewSupervisor(t.Context(), logger, cfg)
	require.NoError(t, err)

	return s, &cfg
}

func getSupervisorConfig(t *testing.T, configType string, extraConfigData map[string]string) *os.File {
	tpl, err := os.ReadFile(path.Join("testdata", "supervisor", "supervisor_"+configType+".yaml"))
	require.NoError(t, err)

	templ, err := template.New("").Parse(string(tpl))
	require.NoError(t, err)

	var buf bytes.Buffer
	var extension string
	if runtime.GOOS == "windows" {
		extension = ".exe"
	}

	configData := map[string]string{
		"goos":        runtime.GOOS,
		"goarch":      runtime.GOARCH,
		"extension":   extension,
		"storage_dir": escapePathStringForWin(t.TempDir()),
	}

	for key, val := range extraConfigData {
		configData[key] = val
	}
	err = templ.Execute(&buf, configData)
	require.NoError(t, err)
	cfgFile, err := os.CreateTemp(t.TempDir(), "config_*.yaml")
	require.NoError(t, err)
	t.Cleanup(func() { cfgFile.Close() })

	_, err = cfgFile.Write(buf.Bytes())
	require.NoError(t, err)

	return cfgFile
}

func writeTempConfigFile(t *testing.T, body string) string {
	t.Helper()

	f, err := os.CreateTemp(t.TempDir(), "config_*.yaml")
	require.NoError(t, err)
	t.Cleanup(func() { f.Close() })

	_, err = f.WriteString(body)
	require.NoError(t, err)

	return f.Name()
}

func writeSupervisorConfigFile(t *testing.T, serverAddr, storageDir string, configFiles []string, useHUP bool) string {
	t.Helper()

	var extension string
	if runtime.GOOS == "windows" {
		extension = ".exe"
	}
	executablePath, err := filepath.Abs("../../bin/otelcontribcol_" + runtime.GOOS + "_" + runtime.GOARCH + extension)
	require.NoError(t, err)

	var buf strings.Builder
	fmt.Fprintf(&buf, "server:\n  endpoint: %q\n\n", "ws://"+serverAddr+"/v1/opamp")
	buf.WriteString("capabilities:\n")
	buf.WriteString("  reports_effective_config: true\n")
	buf.WriteString("  reports_own_metrics: true\n")
	buf.WriteString("  reports_own_logs: true\n")
	buf.WriteString("  reports_own_traces: true\n")
	buf.WriteString("  reports_health: true\n")
	buf.WriteString("  accepts_remote_config: true\n")
	buf.WriteString("  reports_remote_config: true\n")
	buf.WriteString("  accepts_restart_command: true\n\n")
	fmt.Fprintf(&buf, "storage:\n  directory: %q\n\n", storageDir)
	fmt.Fprintf(&buf, "agent:\n  executable: %q\n", executablePath)
	if useHUP {
		buf.WriteString("  use_hup_config_reload: true\n")
	}
	buf.WriteString("  config_files:\n")
	for _, file := range configFiles {
		fmt.Fprintf(&buf, "    - %q\n", file)
	}

	return writeTempConfigFile(t, buf.String())
}

func writeCountingCollectorWrapper(t *testing.T, invocationsFile string) string {
	t.Helper()

	var extension string
	if runtime.GOOS == "windows" {
		extension = ".exe"
	}
	collectorPath, err := filepath.Abs("../../bin/otelcontribcol_" + runtime.GOOS + "_" + runtime.GOARCH + extension)
	require.NoError(t, err)

	wrapperPath := filepath.Join(t.TempDir(), "otelcontribcol-wrapper")
	if runtime.GOOS == "windows" {
		wrapperPath += ".bat"
		script := fmt.Sprintf("@echo off\r\necho %%* >> %q\r\n%q %%*\r\nexit /b %%ERRORLEVEL%%\r\n", invocationsFile, collectorPath)
		require.NoError(t, os.WriteFile(wrapperPath, []byte(script), 0o600))
		return wrapperPath
	}

	script := fmt.Sprintf("#!/bin/sh\nprintf '%%s\\n' \"$*\" >> %q\nexec %q \"$@\"\n", invocationsFile, collectorPath)
	require.NoError(t, os.WriteFile(wrapperPath, []byte(script), 0o700))
	return wrapperPath
}

func collectorInvocationCount(t *testing.T, invocationsFile string) int {
	t.Helper()

	content, err := os.ReadFile(invocationsFile)
	if errors.Is(err, os.ErrNotExist) {
		return 0
	}
	require.NoError(t, err)
	trimmed := strings.TrimSpace(string(content))
	if trimmed == "" {
		return 0
	}
	return len(strings.Split(trimmed, "\n"))
}

func waitForEffectiveConfigMessage(t *testing.T, effectiveConfig *atomic.Value) string {
	t.Helper()

	var cfg string
	require.Eventually(t, func() bool {
		value, ok := effectiveConfig.Load().(string)
		if !ok || value == "" {
			return false
		}
		cfg = value
		return strings.Contains(cfg, "service:")
	}, 10*time.Second, 200*time.Millisecond)

	return cfg
}

// escapePathStringForWin escapes Windows paths for YAML double-quoted strings.
// Some test templates wrap storage.directory in double quotes, where backslashes
// would be treated as escape prefixes (e.g., \U, \t), so we must escape them.
// Non-Windows paths are returned unchanged.
func escapePathStringForWin(path string) string {
	if runtime.GOOS != "windows" {
		return path
	}
	return strings.ReplaceAll(path, "\\", "\\\\")
}

// This test ensures the Supervisor config validation path can validate
// agent::startup_fallback_configs by executing the real Collector binary with:
//
//	<collector> validate --config <cfg1> [--config <cfg2> ...]
//
// The e2e test Makefile target builds the binary at:
//
//	../../bin/otelcontribcol_<GOOS>_<GOARCH>[.exe]
func TestValidateFallbackConfigsWithColBin_E2E(t *testing.T) {
	// Ensure the collector binary exists where the e2e tests expect it.
	ext := ""
	if runtime.GOOS == "windows" {
		ext = ".exe"
	}
	colBinName := fmt.Sprintf("otelcontribcol_%s_%s%s", runtime.GOOS, runtime.GOARCH, ext)
	collectorPath := filepath.Clean(filepath.Join("..", "..", "bin", colBinName))
	require.FileExists(t, collectorPath, "expected Collector binary at %q", collectorPath)

	t.Run("Valid fallback config", func(t *testing.T) {
		goodColConfigPath := filepath.Join("testdata", "collector", "healthcheck_config.yaml")
		cfgFile := getSupervisorConfig(t, "fallback", map[string]string{
			"url":                     "localhost:12345",
			"storage_dir":             t.TempDir(),
			"startup_fallback_config": escapePathStringForWin(goodColConfigPath),
		})

		cfg, err := config.Load(cfgFile.Name())
		require.NoError(t, err)
		_, err = supervisor.NewSupervisor(t.Context(), zap.NewNop(), cfg)
		require.NoError(t, err)
	})

	t.Run("Valid profiles fallback config using agent feature gates", func(t *testing.T) {
		profilesConfigPath := filepath.Join("testdata", "collector", "profiles_pipeline.yaml")
		cfgFile := getSupervisorConfig(t, "fallback", map[string]string{
			"url":                     "localhost:12345",
			"storage_dir":             t.TempDir(),
			"agent_argument":          "--feature-gates=+service.profilesSupport",
			"startup_fallback_config": escapePathStringForWin(profilesConfigPath),
		})

		cfg, err := config.Load(cfgFile.Name())
		require.NoError(t, err)
		require.Equal(t, []string{"--feature-gates=+service.profilesSupport"}, cfg.Agent.Arguments)
		_, err = supervisor.NewSupervisor(t.Context(), zap.NewNop(), cfg)
		require.NoError(t, err)
	})

	t.Run("Invalid fallback config", func(t *testing.T) {
		badColConfigPath := filepath.Join("testdata", "collector", "bad_config.yaml")
		badCfgFile := getSupervisorConfig(t, "fallback", map[string]string{
			"url":                     "localhost:12345",
			"storage_dir":             t.TempDir(),
			"startup_fallback_config": escapePathStringForWin(badColConfigPath),
		})

		cfg, err := config.Load(badCfgFile.Name())
		require.NoError(t, err)
		_, err = supervisor.NewSupervisor(t.Context(), zap.NewNop(), cfg)
		require.ErrorContains(t, err, "could not validate startup fallback configs with agent::executable")
	})
}

func TestSupervisorStartsCollectorWithRemoteConfig(t *testing.T) {
	modes := getTestModes()

	for _, mode := range modes {
		t.Run(mode.name, func(t *testing.T) {
			storageDir := t.TempDir()
			var agentConfig atomic.Value
			server := newOpAMPServer(
				t,
				defaultConnectingHandler,
				types.ConnectionCallbacks{
					OnMessage: func(_ context.Context, _ types.Connection, message *protobufs.AgentToServer) *protobufs.ServerToAgent {
						if message.EffectiveConfig != nil {
							config := message.EffectiveConfig.ConfigMap.ConfigMap[""]
							if config != nil {
								agentConfig.Store(string(config.Body))
							}
						}

						return &protobufs.ServerToAgent{}
					},
				})

			extraConfigData := map[string]string{"url": server.addr, "storage_dir": storageDir}
			if mode.UseHUPConfigReload {
				extraConfigData["use_hup_config_reload"] = "true"
			}

			s, supervisorCfg := newSupervisor(t, "basic", extraConfigData)
			if mode.UseHUPConfigReload {
				require.True(t, supervisorCfg.Agent.UseHUPConfigReload)
			}

			require.Nil(t, s.Start(t.Context()))
			defer s.Shutdown()

			waitForSupervisorConnection(server.supervisorConnected, true)

			cfg, hash, inputFile, outputFile := createSimplePipelineCollectorConf(t)

			server.sendToSupervisor(&protobufs.ServerToAgent{
				RemoteConfig: &protobufs.AgentRemoteConfig{
					Config: &protobufs.AgentConfigMap{
						ConfigMap: map[string]*protobufs.AgentConfigFile{
							"": {Body: cfg.Bytes()},
						},
					},
					ConfigHash: hash,
				},
			})

			require.Eventually(t, func() bool {
				cfg, ok := agentConfig.Load().(string)
				if ok {
					// The effective config may be structurally different compared to what was sent,
					// and will also have some data redacted,
					// so just check that it includes the file_log receiver
					return strings.Contains(cfg, "file_log")
				}

				return false
			}, 5*time.Second, 500*time.Millisecond, "Collector was not started with remote config")

			n, err := inputFile.WriteString("{\"body\":\"hello, world\"}\n")
			require.NotZero(t, n, "Could not write to input file")
			require.NoError(t, err)

			require.Eventually(t, func() bool {
				logRecord := make([]byte, 1024)
				n, _ := outputFile.Read(logRecord)

				return n != 0
			}, 10*time.Second, 500*time.Millisecond, "Log never appeared in output")

			require.Never(t, func() bool {
				_, err := os.Stat(filepath.Join(storageDir, "last_working_remote_config.dat"))
				return err == nil
			}, time.Second, 100*time.Millisecond, "Last working remote config should not be persisted when automatic rollback is disabled")
		})
	}
}

func TestSupervisorStartsCollectorWithLocalConfigOnly(t *testing.T) {
	modes := getTestModes()

	for _, mode := range modes {
		t.Run(mode.name, func(t *testing.T) {
			connected := atomic.Bool{}
			server := newOpAMPServer(t, defaultConnectingHandler, types.ConnectionCallbacks{
				OnConnected: func(context.Context, types.Connection) {
					connected.Store(true)
				},
			})

			cfg, _, inputFile, outputFile := createSimplePipelineCollectorConf(t)

			collectorConfigDir := t.TempDir()
			cfgFile, err := os.CreateTemp(collectorConfigDir, "config_*.yaml")
			t.Cleanup(func() { cfgFile.Close() })
			require.NoError(t, err)

			_, err = cfgFile.Write(cfg.Bytes())
			require.NoError(t, err)

			storageDir := t.TempDir()

			extraConfigData := map[string]string{
				"url":          server.addr,
				"storage_dir":  storageDir,
				"local_config": cfgFile.Name(),
			}
			if mode.UseHUPConfigReload {
				extraConfigData["use_hup_config_reload"] = "true"
			}

			s, supervisorCfg := newSupervisor(t, "basic", extraConfigData)
			if mode.UseHUPConfigReload {
				require.True(t, supervisorCfg.Agent.UseHUPConfigReload)
			}
			t.Cleanup(s.Shutdown)
			require.NoError(t, s.Start(t.Context()))

			waitForSupervisorConnection(server.supervisorConnected, true)
			require.True(t, connected.Load(), "Supervisor failed to connect")

			require.EventuallyWithTf(t, func(c *assert.CollectT) {
				require.Contains(c, getAgentLogs(t, storageDir), "Connected to the OpAMP server")
			}, 10*time.Second, 500*time.Millisecond, "Collector did not connected to the OpAMP server")

			n, err := inputFile.WriteString("{\"body\":\"hello, world\"}\n")
			require.NotZero(t, n, "Could not write to input file")
			require.NoError(t, err)

			require.Eventually(t, func() bool {
				logRecord := make([]byte, 1024)
				n, _ := outputFile.Read(logRecord)

				return n != 0
			}, 10*time.Second, 500*time.Millisecond, "Log never appeared in output")
		})
	}
}

func TestSupervisorStartsCollectorWithDeclarativeTelemetryResourceConfig(t *testing.T) {
	modes := getTestModes()

	for _, mode := range modes {
		t.Run(mode.name, func(t *testing.T) {
			storageDir := t.TempDir()

			var effectiveConfig atomic.Value
			server := newOpAMPServer(t, defaultConnectingHandler, types.ConnectionCallbacks{
				OnMessage: func(_ context.Context, _ types.Connection, message *protobufs.AgentToServer) *protobufs.ServerToAgent {
					if message.EffectiveConfig != nil {
						configFile := message.EffectiveConfig.ConfigMap.ConfigMap[""]
						if configFile != nil {
							effectiveConfig.Store(string(configFile.Body))
						}
					}
					return &protobufs.ServerToAgent{}
				},
			})

			cfgFile := writeTempConfigFile(t, `
receivers:
  nop:

exporters:
  nop:

service:
  telemetry:
    resource:
      attributes:
        - name: otelcol.service.mode
          value: agent
  pipelines:
    logs:
      receivers: [nop]
      exporters: [nop]
`)

			extraConfigData := map[string]string{
				"url":          server.addr,
				"storage_dir":  storageDir,
				"local_config": cfgFile,
			}
			if mode.UseHUPConfigReload {
				extraConfigData["use_hup_config_reload"] = "true"
			}

			s, supervisorCfg := newSupervisor(t, "basic", extraConfigData)
			if mode.UseHUPConfigReload {
				require.True(t, supervisorCfg.Agent.UseHUPConfigReload)
			}
			t.Cleanup(s.Shutdown)
			require.NoError(t, s.Start(t.Context()))

			waitForSupervisorConnection(server.supervisorConnected, true)

			k := koanf.New("::")
			require.NoError(t, k.Load(rawbytes.Provider([]byte(waitForEffectiveConfigMessage(t, &effectiveConfig))), yaml.Parser()))

			resource, ok := k.Get("service::telemetry::resource").(map[string]any)
			require.True(t, ok)
			assert.NotContains(t, resource, "service.name")
			assert.NotContains(t, resource, "service.version")

			attrs, ok := resource["attributes"].([]any)
			require.True(t, ok)

			attrNames := make(map[string]struct{}, len(attrs))
			for _, attr := range attrs {
				attrMap, ok := attr.(map[string]any)
				require.True(t, ok)
				name, ok := attrMap["name"].(string)
				require.True(t, ok)
				attrNames[name] = struct{}{}
			}

			assert.Contains(t, attrNames, "otelcol.service.mode")
			assert.Contains(t, attrNames, "service.name")
		})
	}
}

func TestSupervisorStartsCollectorWithNoPipelineConfig(t *testing.T) {
	modes := getTestModes()

	for _, mode := range modes {
		t.Run(mode.name, func(t *testing.T) {
			connected := atomic.Bool{}
			server := newOpAMPServer(t, defaultConnectingHandler, types.ConnectionCallbacks{
				OnConnected: func(context.Context, types.Connection) {
					connected.Store(true)
				},
			})

			cfg, _ := createEmptyPipelineCollectorConf(t)

			collectorConfigDir := t.TempDir()
			cfgFile, err := os.CreateTemp(collectorConfigDir, "config_*.yaml")
			t.Cleanup(func() { cfgFile.Close() })
			require.NoError(t, err)

			_, err = cfgFile.Write(cfg.Bytes())
			require.NoError(t, err)

			storageDir := t.TempDir()

			extraConfigData := map[string]string{
				"url":          server.addr,
				"storage_dir":  storageDir,
				"local_config": cfgFile.Name(),
			}
			if mode.UseHUPConfigReload {
				extraConfigData["use_hup_config_reload"] = "true"
			}

			s, supervisorCfg := newSupervisor(t, "basic", extraConfigData)
			if mode.UseHUPConfigReload {
				require.True(t, supervisorCfg.Agent.UseHUPConfigReload)
			}
			t.Cleanup(s.Shutdown)
			require.NoError(t, s.Start(t.Context()))

			waitForSupervisorConnection(server.supervisorConnected, true)
			require.True(t, connected.Load(), "Supervisor failed to connect")

			require.EventuallyWithTf(t, func(c *assert.CollectT) {
				require.Contains(c, getAgentLogs(t, storageDir), "Connected to the OpAMP server")
			}, 10*time.Second, 500*time.Millisecond, "Collector did not connected to the OpAMP server")
		})
	}
}

func TestSupervisorStartsCollectorWithNoOpAMPServerWithNoLastRemoteConfig(t *testing.T) {
	modes := getTestModes()

	for _, mode := range modes {
		t.Run(mode.name, func(t *testing.T) {
			storageDir := t.TempDir()
			t.Log("Storage dir:", storageDir)
			t.Cleanup(func() {
				content, _ := os.ReadFile(filepath.Join(storageDir, "effective_config.yaml"))
				t.Logf("EffectiveConfig:\n%s", string(content))

				content, _ = os.ReadFile(filepath.Join(storageDir, "agent.log"))
				t.Logf("Agent logs:\n%s", string(content))
			})

			connected := atomic.Bool{}
			server := newUnstartedOpAMPServer(t, defaultConnectingHandler, types.ConnectionCallbacks{
				OnConnected: func(ctx context.Context, conn types.Connection) {
					connected.Store(true)
				},
			})

			extraConfigData := map[string]string{
				"url":          server.addr,
				"storage_dir":  storageDir,
				"local_config": filepath.Join("testdata", "collector", "healthcheck_config.yaml"),
			}
			if mode.UseHUPConfigReload {
				extraConfigData["use_hup_config_reload"] = "true"
			}

			s, supervisorCfg := newSupervisor(t, "healthcheck_port", extraConfigData)
			if mode.UseHUPConfigReload {
				require.True(t, supervisorCfg.Agent.UseHUPConfigReload)
			}
			t.Cleanup(s.Shutdown)
			require.Nil(t, s.Start(t.Context()))

			// Verify the collector runs eventually by pinging the healthcheck extension
			require.Eventually(t, func() bool {
				resp, err := http.DefaultClient.Get("http://localhost:13133")
				if err != nil {
					t.Logf("Failed healthcheck: %s", err)
					return false
				}
				require.NoError(t, resp.Body.Close())
				if resp.StatusCode >= 300 || resp.StatusCode < 200 {
					t.Logf("Got non-2xx status code: %d", resp.StatusCode)
					return false
				}
				return true
			}, 3*time.Second, 100*time.Millisecond)

			// Start the server and wait for the supervisor to connect
			server.start()

			// Verify supervisor connects to server
			waitForSupervisorConnection(server.supervisorConnected, true)

			require.True(t, connected.Load(), "Supervisor failed to connect")
		})
	}
}

func TestSupervisorStartsCollectorWithNoOpAMPServerUsingLastRemoteConfig(t *testing.T) {
	modes := getTestModes()

	for _, mode := range modes {
		t.Run(mode.name, func(t *testing.T) {
			storageDir := t.TempDir()
			remoteConfigFilePath := filepath.Join(storageDir, "last_recv_remote_config.dat")

			cfg, hash, healthcheckPort := createHealthCheckCollectorConf(t, true)
			remoteConfigProto := &protobufs.AgentRemoteConfig{
				Config: &protobufs.AgentConfigMap{
					ConfigMap: map[string]*protobufs.AgentConfigFile{
						"": {Body: cfg.Bytes()},
					},
				},
				ConfigHash: hash,
			}
			marshalledRemoteConfig, err := proto.Marshal(remoteConfigProto)
			require.NoError(t, err)

			require.NoError(t, os.WriteFile(remoteConfigFilePath, marshalledRemoteConfig, 0o600))

			fallbackConfigHealthCheckPort, err := findRandomPort()
			require.NoError(t, err)
			fallbackConfigPath, _, _ := createFallbackCollectorConf(
				t,
				strconv.Itoa(fallbackConfigHealthCheckPort),
			)

			connected := atomic.Bool{}
			server := newUnstartedOpAMPServer(t, defaultConnectingHandler, types.ConnectionCallbacks{
				OnConnected: func(ctx context.Context, conn types.Connection) {
					connected.Store(true)
				},
			})
			defer server.shutdown()

			extraConfigData := map[string]string{
				"url":                     server.addr,
				"storage_dir":             storageDir,
				"startup_fallback_config": escapePathStringForWin(fallbackConfigPath),
			}
			if mode.UseHUPConfigReload {
				extraConfigData["use_hup_config_reload"] = "true"
			}

			s, supervisorCfg := newSupervisor(t, "fallback", extraConfigData)
			if mode.UseHUPConfigReload {
				require.True(t, supervisorCfg.Agent.UseHUPConfigReload)
			}

			require.Nil(t, s.Start(t.Context()))
			defer s.Shutdown()

			// Fallback should not be applied when a persisted remote config exists.
			require.Never(t, func() bool {
				return healthCheckOK(fallbackConfigHealthCheckPort)
			}, 2*time.Second, 200*time.Millisecond, "Fallback config should not be applied when persisted config exists")

			// Verify the collector runs eventually by pinging the healthcheck extension
			require.Eventually(t, func() bool {
				resp, err := http.DefaultClient.Get(fmt.Sprintf("http://localhost:%d", healthcheckPort))
				if err != nil {
					t.Logf("Failed healthcheck: %s", err)
					return false
				}
				require.NoError(t, resp.Body.Close())
				if resp.StatusCode >= 300 || resp.StatusCode < 200 {
					t.Logf("Got non-2xx status code: %d", resp.StatusCode)
					return false
				}
				return true
			}, 3*time.Second, 100*time.Millisecond)

			// Start the server and wait for the supervisor to connect
			server.start()

			// Verify supervisor connects to server
			waitForSupervisorConnection(server.supervisorConnected, true)

			require.True(t, connected.Load(), "Supervisor failed to connect")
		})
	}
}

func TestSupervisorRestartsWithLastWorkingRemoteConfigAfterFailedConfig(t *testing.T) {
	modes := getTestModes()

	for _, mode := range modes {
		t.Run(mode.name, func(t *testing.T) {
			storageDir := t.TempDir()
			var firstRemoteConfigStatus atomic.Value
			firstServer := newOpAMPServer(
				t,
				defaultConnectingHandler,
				types.ConnectionCallbacks{
					OnMessage: func(_ context.Context, _ types.Connection, message *protobufs.AgentToServer) *protobufs.ServerToAgent {
						if message.RemoteConfigStatus != nil {
							firstRemoteConfigStatus.Store(message.RemoteConfigStatus)
						}
						return &protobufs.ServerToAgent{}
					},
				},
			)

			extraConfigData := map[string]string{
				"url":                       firstServer.addr,
				"storage_dir":               storageDir,
				"automatic_config_rollback": "true",
			}
			if mode.UseHUPConfigReload {
				extraConfigData["use_hup_config_reload"] = "true"
			}

			firstSupervisor, supervisorCfg := newSupervisor(t, "basic", extraConfigData)
			require.True(t, supervisorCfg.Agent.AutomaticConfigRollback)
			if mode.UseHUPConfigReload {
				require.True(t, supervisorCfg.Agent.UseHUPConfigReload)
			}

			require.NoError(t, firstSupervisor.Start(t.Context()))
			waitForSupervisorConnection(firstServer.supervisorConnected, true)

			workingCfg, workingHash, healthcheckPort := createHealthCheckCollectorConf(t, true)
			firstServer.sendToSupervisor(&protobufs.ServerToAgent{
				RemoteConfig: &protobufs.AgentRemoteConfig{
					Config: &protobufs.AgentConfigMap{
						ConfigMap: map[string]*protobufs.AgentConfigFile{
							"": {Body: workingCfg.Bytes()},
						},
					},
					ConfigHash: workingHash,
				},
			})

			require.EventuallyWithT(t, func(c *assert.CollectT) {
				resp, err := http.DefaultClient.Get(fmt.Sprintf("http://localhost:%d", healthcheckPort))
				if err != nil {
					c.Errorf("failed healthcheck: %v", err)
					return
				}
				require.NoError(c, resp.Body.Close())
				require.GreaterOrEqual(c, resp.StatusCode, 200)
				require.True(c, resp.StatusCode >= 200 && resp.StatusCode < 300)
			}, 10*time.Second, 100*time.Millisecond, "Collector did not become healthy with the working config")

			require.Eventually(t, func() bool {
				status, ok := firstRemoteConfigStatus.Load().(*protobufs.RemoteConfigStatus)
				return ok && status.Status == protobufs.RemoteConfigStatuses_RemoteConfigStatuses_APPLIED && bytes.Equal(status.LastRemoteConfigHash, workingHash)
			}, 15*time.Second, 100*time.Millisecond, "Working remote config was not reported as applied")

			badCfg, badHash := createBadCollectorConf(t)
			firstServer.sendToSupervisor(&protobufs.ServerToAgent{
				RemoteConfig: &protobufs.AgentRemoteConfig{
					Config: &protobufs.AgentConfigMap{
						ConfigMap: map[string]*protobufs.AgentConfigFile{
							"": {Body: badCfg.Bytes()},
						},
					},
					ConfigHash: badHash,
				},
			})

			require.Eventually(t, func() bool {
				status, ok := firstRemoteConfigStatus.Load().(*protobufs.RemoteConfigStatus)
				return ok && status.Status == protobufs.RemoteConfigStatuses_RemoteConfigStatuses_FAILED && bytes.Equal(status.LastRemoteConfigHash, badHash)
			}, 15*time.Second, 100*time.Millisecond, "Failed remote config status was not reported")

			firstSupervisor.Shutdown()
			firstServer.shutdown()
			require.EventuallyWithT(t, func(c *assert.CollectT) {
				resp, err := http.DefaultClient.Get(fmt.Sprintf("http://localhost:%d", healthcheckPort))
				if err != nil {
					return
				}
				require.NoError(c, resp.Body.Close())
				c.Errorf("healthcheck endpoint still responding with status %d", resp.StatusCode)
			}, 5*time.Second, 100*time.Millisecond, "Previous collector instance did not shut down")
			time.Sleep(250 * time.Millisecond)

			var restartedFallbackStatusReported atomic.Bool
			restartServer := newOpAMPServer(
				t,
				defaultConnectingHandler,
				types.ConnectionCallbacks{
					OnMessage: func(_ context.Context, _ types.Connection, message *protobufs.AgentToServer) *protobufs.ServerToAgent {
						if status := message.RemoteConfigStatus; status != nil &&
							status.Status == protobufs.RemoteConfigStatuses_RemoteConfigStatuses_APPLIED &&
							bytes.Equal(status.LastRemoteConfigHash, workingHash) {
							restartedFallbackStatusReported.Store(true)
						}
						return &protobufs.ServerToAgent{}
					},
				},
			)

			restartedSupervisor, restartedSupervisorCfg := newSupervisor(t, "basic", map[string]string{
				"url":                       restartServer.addr,
				"storage_dir":               storageDir,
				"automatic_config_rollback": "true",
				"use_hup_config_reload": func() string {
					if mode.UseHUPConfigReload {
						return "true"
					}
					return ""
				}(),
			})
			require.True(t, restartedSupervisorCfg.Agent.AutomaticConfigRollback)
			if mode.UseHUPConfigReload {
				require.True(t, restartedSupervisorCfg.Agent.UseHUPConfigReload)
			}

			require.NoError(t, restartedSupervisor.Start(t.Context()))
			defer restartedSupervisor.Shutdown()

			waitForSupervisorConnection(restartServer.supervisorConnected, true)

			require.EventuallyWithT(t, func(c *assert.CollectT) {
				resp, err := http.DefaultClient.Get(fmt.Sprintf("http://localhost:%d", healthcheckPort))
				if err != nil {
					c.Errorf("failed healthcheck: %v", err)
					return
				}
				require.NoError(c, resp.Body.Close())
				require.True(c, resp.StatusCode >= 200 && resp.StatusCode < 300)
			}, 10*time.Second, 100*time.Millisecond, "Collector did not restart with the last working config")

			require.Never(t, restartedFallbackStatusReported.Load, time.Second, 100*time.Millisecond, "Last working remote config status should not be reported during restart fallback")
		})
	}
}

func TestSupervisorRestoresLastWorkingRemoteConfigAtRuntimeAfterFailedConfig(t *testing.T) {
	storageDir := t.TempDir()
	var remoteConfigStatus atomic.Value
	server := newOpAMPServer(
		t,
		defaultConnectingHandler,
		types.ConnectionCallbacks{
			OnMessage: func(_ context.Context, _ types.Connection, message *protobufs.AgentToServer) *protobufs.ServerToAgent {
				if message.RemoteConfigStatus != nil {
					remoteConfigStatus.Store(message.RemoteConfigStatus)
				}
				return &protobufs.ServerToAgent{}
			},
		},
	)

	s, supervisorCfg := newSupervisor(t, "basic", map[string]string{
		"url":                       server.addr,
		"storage_dir":               storageDir,
		"automatic_config_rollback": "true",
	})
	require.True(t, supervisorCfg.Agent.AutomaticConfigRollback)

	require.NoError(t, s.Start(t.Context()))
	defer s.Shutdown()

	waitForSupervisorConnection(server.supervisorConnected, true)

	workingCfg, workingHash, healthcheckPort := createHealthCheckCollectorConf(t, true)
	server.sendToSupervisor(&protobufs.ServerToAgent{
		RemoteConfig: &protobufs.AgentRemoteConfig{
			Config: &protobufs.AgentConfigMap{
				ConfigMap: map[string]*protobufs.AgentConfigFile{
					"": {Body: workingCfg.Bytes()},
				},
			},
			ConfigHash: workingHash,
		},
	})

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		resp, err := http.DefaultClient.Get(fmt.Sprintf("http://localhost:%d", healthcheckPort))
		if err != nil {
			c.Errorf("failed healthcheck: %v", err)
			return
		}
		require.NoError(c, resp.Body.Close())
		require.True(c, resp.StatusCode >= 200 && resp.StatusCode < 300)
	}, 10*time.Second, 100*time.Millisecond, "Collector did not become healthy with the working config")

	require.Eventually(t, func() bool {
		status, ok := remoteConfigStatus.Load().(*protobufs.RemoteConfigStatus)
		return ok && status.Status == protobufs.RemoteConfigStatuses_RemoteConfigStatuses_APPLIED && bytes.Equal(status.LastRemoteConfigHash, workingHash)
	}, 15*time.Second, 100*time.Millisecond, "Working remote config was not reported as applied")

	badCfg, badHash := createBadCollectorConf(t)
	server.sendToSupervisor(&protobufs.ServerToAgent{
		RemoteConfig: &protobufs.AgentRemoteConfig{
			Config: &protobufs.AgentConfigMap{
				ConfigMap: map[string]*protobufs.AgentConfigFile{
					"": {Body: badCfg.Bytes()},
				},
			},
			ConfigHash: badHash,
		},
	})

	require.Eventually(t, func() bool {
		status, ok := remoteConfigStatus.Load().(*protobufs.RemoteConfigStatus)
		return ok && status.Status == protobufs.RemoteConfigStatuses_RemoteConfigStatuses_FAILED && bytes.Equal(status.LastRemoteConfigHash, badHash)
	}, 15*time.Second, 100*time.Millisecond, "Failed remote config status was not reported")

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		resp, err := http.DefaultClient.Get(fmt.Sprintf("http://localhost:%d", healthcheckPort))
		if err != nil {
			c.Errorf("failed healthcheck: %v", err)
			return
		}
		require.NoError(c, resp.Body.Close())
		require.True(c, resp.StatusCode >= 200 && resp.StatusCode < 300)
	}, 10*time.Second, 100*time.Millisecond, "Collector did not restore the last working config at runtime")

	require.Eventually(t, func() bool {
		status, ok := remoteConfigStatus.Load().(*protobufs.RemoteConfigStatus)
		return ok && status.Status == protobufs.RemoteConfigStatuses_RemoteConfigStatuses_APPLIED && bytes.Equal(status.LastRemoteConfigHash, workingHash)
	}, 15*time.Second, 100*time.Millisecond, "Restored last working remote config was not reported as applied")
}

func TestSupervisorDoesNotTightlyLoopWhenRestoredLastWorkingRemoteConfigFails(t *testing.T) {
	for _, mode := range getTestModes() {
		t.Run(mode.name, func(t *testing.T) {
			storageDir := t.TempDir()
			workingPort, err := findRandomPort()
			require.NoError(t, err)
			workingCfg, workingHash := createHealthCheckCollectorConfWithPort(t, strconv.Itoa(workingPort))
			badCfg, badHash := createBadCollectorConf(t)
			collectorInvocationsFile := filepath.Join(t.TempDir(), "collector-invocations.txt")
			collectorWrapper := writeCountingCollectorWrapper(t, collectorInvocationsFile)

			server := newOpAMPServer(
				t,
				defaultConnectingHandler,
				types.ConnectionCallbacks{
					OnMessage: func(context.Context, types.Connection, *protobufs.AgentToServer) *protobufs.ServerToAgent {
						return &protobufs.ServerToAgent{}
					},
				},
			)

			cfgFile := writeTempConfigFile(t, fmt.Sprintf(`
server:
  endpoint: %q

capabilities:
  reports_effective_config: true
  reports_health: true
  accepts_remote_config: true
  reports_remote_config: true

storage:
  directory: %q

agent:
  executable: %q
  bootstrap_timeout: 10s
  automatic_config_rollback: true
  use_hup_config_reload: %v
`, "ws://"+server.addr+"/v1/opamp", storageDir, collectorWrapper, mode.UseHUPConfigReload))

			s, supervisorCfg := newSupervisorFromConfigFile(t, cfgFile)
			require.True(t, supervisorCfg.Agent.AutomaticConfigRollback)
			if mode.UseHUPConfigReload {
				require.True(t, supervisorCfg.Agent.UseHUPConfigReload)
			}

			require.NoError(t, s.Start(t.Context()))
			t.Cleanup(s.Shutdown)

			waitForSupervisorConnection(server.supervisorConnected, true)

			server.sendToSupervisor(&protobufs.ServerToAgent{
				RemoteConfig: &protobufs.AgentRemoteConfig{
					Config: &protobufs.AgentConfigMap{
						ConfigMap: map[string]*protobufs.AgentConfigFile{
							"": {Body: workingCfg.Bytes()},
						},
					},
					ConfigHash: workingHash,
				},
			})

			require.Eventually(t, func() bool {
				return healthCheckOK(workingPort)
			}, 10*time.Second, 100*time.Millisecond, "Collector did not become healthy with the working config")
			require.Eventually(t, func() bool {
				_, err := os.Stat(filepath.Join(storageDir, "last_working_remote_config.dat"))
				return err == nil
			}, 15*time.Second, 100*time.Millisecond, "Working remote config was not saved as last working")

			invocationsBeforeBadConfig := collectorInvocationCount(t, collectorInvocationsFile)
			claimedWorkingPortCh := claimPortWhenFree(t, workingPort)
			server.sendToSupervisor(&protobufs.ServerToAgent{
				RemoteConfig: &protobufs.AgentRemoteConfig{
					Config: &protobufs.AgentConfigMap{
						ConfigMap: map[string]*protobufs.AgentConfigFile{
							"": {Body: badCfg.Bytes()},
						},
					},
					ConfigHash: badHash,
				},
			})

			select {
			case claimedWorkingPort := <-claimedWorkingPortCh:
				require.NotNil(t, claimedWorkingPort)
				t.Cleanup(func() {
					require.NoError(t, claimedWorkingPort.Close())
				})
			case <-time.After(10 * time.Second):
				t.Fatal("did not claim the last working config port after the collector stopped")
			}

			// In process-restart mode the bad config and the restored last working
			// config each start a new collector process. In HUP mode the bad config
			// is reloaded in-place, so whether it adds an invocation depends on when
			// the collector exits; only the restored config is guaranteed to start a
			// new process.
			minInvocations := 2
			if mode.UseHUPConfigReload {
				minInvocations = 1
			}
			require.Eventually(t, func() bool {
				return collectorInvocationCount(t, collectorInvocationsFile) >= invocationsBeforeBadConfig+minInvocations
			}, 15*time.Second, 100*time.Millisecond, "Restored last working config was not started after the bad config")

			require.Never(t, func() bool {
				return collectorInvocationCount(t, collectorInvocationsFile) > invocationsBeforeBadConfig+2
			}, 2*time.Second, 100*time.Millisecond, "Restored last working remote config failed repeatedly without waiting for restart backoff")
		})
	}
}

func TestSupervisorStartsCollectorWithRemoteConfigAndExecParams(t *testing.T) {
	storageDir := t.TempDir()

	// create remote config to check agent's health
	remoteConfigFilePath := filepath.Join(storageDir, "last_recv_remote_config.dat")
	cfg, hash, healthcheckPort := createHealthCheckCollectorConf(t, false)
	remoteConfigProto := &protobufs.AgentRemoteConfig{
		Config: &protobufs.AgentConfigMap{
			ConfigMap: map[string]*protobufs.AgentConfigFile{
				"": {Body: cfg.Bytes()},
			},
		},
		ConfigHash: hash,
	}
	marshalledRemoteConfig, err := proto.Marshal(remoteConfigProto)
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(remoteConfigFilePath, marshalledRemoteConfig, 0o600))

	// create server
	var agentConfig atomic.Value
	server := newOpAMPServer(
		t,
		defaultConnectingHandler,
		types.ConnectionCallbacks{
			OnMessage: func(_ context.Context, _ types.Connection, message *protobufs.AgentToServer) *protobufs.ServerToAgent {
				if message.EffectiveConfig != nil {
					config := message.EffectiveConfig.ConfigMap.ConfigMap[""]
					if config != nil {
						agentConfig.Store(string(config.Body))
					}
				}
				return &protobufs.ServerToAgent{}
			},
		})

	// create input and output log files for checking the config passed via config_files param
	inputFile, err := os.CreateTemp(storageDir, "input.log")
	require.NoError(t, err)
	t.Cleanup(func() { inputFile.Close() })

	outputFile, err := os.CreateTemp(storageDir, "output.log")
	require.NoError(t, err)
	t.Cleanup(func() { outputFile.Close() })

	secondHealthcheckPort, err := findRandomPort()
	require.NoError(t, err)

	// fill env variables passed via parameters which are used in the collector config passed via config_files param
	s, _ := newSupervisor(t, "exec_config", map[string]string{
		"url":             server.addr,
		"storage_dir":     storageDir,
		"inputLogFile":    inputFile.Name(),
		"outputLogFile":   outputFile.Name(),
		"healthcheckPort": strconv.Itoa(secondHealthcheckPort),
	})

	require.Nil(t, s.Start(t.Context()))
	defer s.Shutdown()

	waitForSupervisorConnection(server.supervisorConnected, true)

	for _, port := range []int{healthcheckPort, secondHealthcheckPort} {
		require.Eventually(t, func() bool {
			resp, err := http.DefaultClient.Get(fmt.Sprintf("http://localhost:%d", port))
			if err != nil {
				t.Logf("Failed healthcheck: %s", err)
				return false
			}
			require.NoError(t, resp.Body.Close())
			if resp.StatusCode >= 300 || resp.StatusCode < 200 {
				t.Logf("Got non-2xx status code: %d", resp.StatusCode)
				return false
			}
			return true
		}, 3*time.Second, 100*time.Millisecond)
	}

	// check that collector uses file_log receiver and file exporter from config passed via config_files param
	n, err := inputFile.WriteString("{\"body\":\"hello, world\"}\n")
	require.NotZero(t, n, "Could not write to input file")
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		logRecord := make([]byte, 1024)
		n, _ := outputFile.Read(logRecord)

		return n != 0
	}, 20*time.Second, 500*time.Millisecond, "Log never appeared in output")
}

func TestSupervisorStartsWithNoOpAMPServer(t *testing.T) {
	cfg, hash, inputFile, outputFile := createSimplePipelineCollectorConf(t)

	configuredChan := make(chan struct{})
	connected := atomic.Bool{}
	server := newUnstartedOpAMPServer(t, defaultConnectingHandler,
		types.ConnectionCallbacks{
			OnConnected: func(ctx context.Context, conn types.Connection) {
				connected.Store(true)
			},
			OnMessage: func(ctx context.Context, conn types.Connection, message *protobufs.AgentToServer) *protobufs.ServerToAgent {
				lastCfgHash := message.GetRemoteConfigStatus().GetLastRemoteConfigHash()
				if bytes.Equal(lastCfgHash, hash) {
					close(configuredChan)
				}

				return &protobufs.ServerToAgent{}
			},
		})
	defer server.shutdown()

	// The supervisor is started without a running OpAMP server.
	// The supervisor should start successfully, even if the OpAMP server is stopped.
	s, _ := newSupervisor(t, "healthcheck_port", map[string]string{
		"url":              server.addr,
		"healthcheck_port": "12345",
	})

	require.Nil(t, s.Start(t.Context()))
	defer s.Shutdown()

	// Verify the collector is not running by checking the healthcheck endpoint fails consistently
	// Start the server and wait for the supervisor to connect
	time.Sleep(250 * time.Millisecond)
	_, err := http.DefaultClient.Get("http://localhost:12345")

	if runtime.GOOS != "windows" {
		require.ErrorContains(t, err, "connection refused")
	} else {
		require.ErrorContains(t, err, "No connection could be made")
	}

	server.start()

	// Verify supervisor connects to server
	waitForSupervisorConnection(server.supervisorConnected, true)

	require.True(t, connected.Load(), "Supervisor failed to connect")

	// Verify that the collector can run a new config sent to it
	server.sendToSupervisor(&protobufs.ServerToAgent{
		RemoteConfig: &protobufs.AgentRemoteConfig{
			Config: &protobufs.AgentConfigMap{
				ConfigMap: map[string]*protobufs.AgentConfigFile{
					"": {Body: cfg.Bytes()},
				},
			},
			ConfigHash: hash,
		},
	})

	select {
	case <-configuredChan:
	case <-time.After(2 * time.Second):
		require.FailNow(t, "timed out waiting for collector to reconfigure")
	}

	sampleLog := `{"body":"hello, world"}`
	n, err := inputFile.WriteString(sampleLog + "\n")
	require.NotZero(t, n, "Could not write to input file")
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		logRecord := make([]byte, 1024)

		n, err = outputFile.Read(logRecord)
		if !errors.Is(err, io.EOF) {
			require.NoError(t, err)
		}

		return n != 0
	}, 10*time.Second, 500*time.Millisecond, "Log never appeared in output")
}

func TestSupervisorRestartsCollectorAfterBadConfig(t *testing.T) {
	modes := getTestModes()

	for _, mode := range modes {
		t.Run(mode.name, func(t *testing.T) {
			var healthReport atomic.Value
			var agentConfig atomic.Value
			server := newOpAMPServer(
				t,
				defaultConnectingHandler,
				types.ConnectionCallbacks{
					OnMessage: func(_ context.Context, _ types.Connection, message *protobufs.AgentToServer) *protobufs.ServerToAgent {
						if message.Health != nil {
							healthReport.Store(message.Health)
						}
						if message.EffectiveConfig != nil {
							config := message.EffectiveConfig.ConfigMap.ConfigMap[""]
							if config != nil {
								agentConfig.Store(string(config.Body))
							}
						}

						return &protobufs.ServerToAgent{}
					},
				})

			extraConfigData := map[string]string{"url": server.addr}
			if mode.UseHUPConfigReload {
				extraConfigData["use_hup_config_reload"] = "true"
			}

			s, supervisorCfg := newSupervisor(t, "basic", extraConfigData)
			if mode.UseHUPConfigReload {
				require.True(t, supervisorCfg.Agent.UseHUPConfigReload)
			}

			require.Nil(t, s.Start(t.Context()))
			defer s.Shutdown()

			waitForSupervisorConnection(server.supervisorConnected, true)

			cfg, hash := createBadCollectorConf(t)

			server.sendToSupervisor(&protobufs.ServerToAgent{
				RemoteConfig: &protobufs.AgentRemoteConfig{
					Config: &protobufs.AgentConfigMap{
						ConfigMap: map[string]*protobufs.AgentConfigFile{
							"": {Body: cfg.Bytes()},
						},
					},
					ConfigHash: hash,
				},
			})

			require.Eventually(t, func() bool {
				cfg, ok := agentConfig.Load().(string)
				if ok {
					// The effective config may be structurally different compared to what was sent,
					// so just check that it includes some strings we know to be unique to the remote config.
					return strings.Contains(cfg, "doesntexist")
				}

				return false
			}, 5*time.Second, 500*time.Millisecond, "Collector was not started with remote config")

			unhealthyTimeout := supervisorCfg.Agent.BootstrapTimeout + 2*time.Second
			require.Eventually(t, func() bool {
				health := healthReport.Load().(*protobufs.ComponentHealth)

				if health != nil {
					return !health.Healthy && health.LastError != ""
				}

				return false
			}, unhealthyTimeout, 250*time.Millisecond, "Supervisor never reported that the Collector was unhealthy")

			cfg, hash, _, _ = createSimplePipelineCollectorConf(t)

			server.sendToSupervisor(&protobufs.ServerToAgent{
				RemoteConfig: &protobufs.AgentRemoteConfig{
					Config: &protobufs.AgentConfigMap{
						ConfigMap: map[string]*protobufs.AgentConfigFile{
							"": {Body: cfg.Bytes()},
						},
					},
					ConfigHash: hash,
				},
			})

			require.Eventually(t, func() bool {
				health := healthReport.Load().(*protobufs.ComponentHealth)

				if health != nil {
					return health.Healthy && health.LastError == ""
				}

				return false
			}, 5*time.Second, 250*time.Millisecond, "Supervisor never reported that the Collector became healthy")
		})
	}
}

func TestSupervisorConfiguresCapabilities(t *testing.T) {
	var capabilities atomic.Uint64
	server := newOpAMPServer(
		t,
		defaultConnectingHandler,
		types.ConnectionCallbacks{
			OnMessage: func(_ context.Context, _ types.Connection, message *protobufs.AgentToServer) *protobufs.ServerToAgent {
				capabilities.Store(message.Capabilities)

				return &protobufs.ServerToAgent{}
			},
		})

	s, _ := newSupervisor(t, "nocap", map[string]string{"url": server.addr})

	require.Nil(t, s.Start(t.Context()))
	defer s.Shutdown()

	waitForSupervisorConnection(server.supervisorConnected, true)

	require.Eventually(t, func() bool {
		caps := capabilities.Load()

		return caps == uint64(protobufs.AgentCapabilities_AgentCapabilities_ReportsStatus|protobufs.AgentCapabilities_AgentCapabilities_ReportsHeartbeat)
	}, 5*time.Second, 250*time.Millisecond)
}

func TestSupervisorPackageCapabilitiesReturnError(t *testing.T) {
	// Verifies that when accepts_packages or reports_package_statuses are enabled,
	// the supervisor fails to start and never connects to the server.
	if runtime.GOOS == "windows" {
		t.Skip("Zap does not close the log file and Windows disallows removing files that are still opened.")
	}

	connected := atomic.Bool{}
	server := newUnstartedOpAMPServer(t, defaultConnectingHandler, types.ConnectionCallbacks{
		OnConnected: func(ctx context.Context, conn types.Connection) {
			connected.Store(true)
		},
	})
	defer server.shutdown()
	server.start()

	storageDir := t.TempDir()
	supervisorLogFilePath := filepath.Join(storageDir, "supervisor_log.log")
	cfgFile := getSupervisorConfig(t, "packages_cap", map[string]string{
		"url":         server.addr,
		"storage_dir": storageDir,
		"log_level":   "0",
		"log_file":    supervisorLogFilePath,
	})
	cfg, err := config.Load(cfgFile.Name())
	require.NoError(t, err)
	logger, err := telemetry.NewLogger(cfg.Telemetry.Logs)
	require.NoError(t, err)

	s, err := supervisor.NewSupervisor(t.Context(), logger, cfg)
	require.NoError(t, err)
	err = s.Start(t.Context())
	require.ErrorContains(t, err, "accepts_packages and reports_package_statuses capabilities are not yet fully implemented")
	require.False(t, connected.Load(), "Supervisor should not have connected to the server")
}

func TestSupervisorBootstrapsCollector(t *testing.T) {
	tests := []struct {
		name     string
		cfg      string
		env      []string
		precheck func(t *testing.T)
	}{
		{
			name: "With service.AllowNoPipelines",
			cfg:  "nocap",
			precheck: func(t *testing.T) {
			},
		},
		{
			name: "Without service.AllowNoPipelines",
			cfg:  "no_fg",
			env: []string{
				"COLLECTOR_BIN=../../bin/otelcontribcol_" + runtime.GOOS + "_" + runtime.GOARCH,
			},
			precheck: func(t *testing.T) {
				if runtime.GOOS == "windows" {
					t.Skip("This test requires a shell script, which may not be supported by Windows")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.precheck(t)
			agentDescription := atomic.Value{}

			// Load the Supervisor config so we can get the location of
			// the Collector that will be run.
			var cfg config.Supervisor
			cfgFile := getSupervisorConfig(t, tt.cfg, map[string]string{})
			k := koanf.New("::")
			err := k.Load(file.Provider(cfgFile.Name()), yaml.Parser())
			require.NoError(t, err)
			err = k.UnmarshalWithConf("", &cfg, koanf.UnmarshalConf{
				Tag: "mapstructure",
			})
			require.NoError(t, err)

			// Get the binary name and version from the Collector binary
			// using the `components` command that prints a YAML-encoded
			// map of information about the Collector build. Some of this
			// information will be used as defaults for the telemetry
			// attributes.
			agentPath := cfg.Agent.Executable
			cmd := exec.Command(agentPath, "components")
			for _, env := range tt.env {
				cmd.Env = append(cmd.Env, env)
			}
			componentsInfo, err := cmd.Output()
			require.NoError(t, err)
			k = koanf.New("::")
			err = k.Load(rawbytes.Provider(componentsInfo), yaml.Parser())
			require.NoError(t, err)
			buildinfo := k.StringMap("buildinfo")
			command := buildinfo["command"]
			version := buildinfo["version"]

			server := newOpAMPServer(
				t,
				defaultConnectingHandler,
				types.ConnectionCallbacks{
					OnMessage: func(_ context.Context, _ types.Connection, message *protobufs.AgentToServer) *protobufs.ServerToAgent {
						if message.AgentDescription != nil {
							agentDescription.Store(message.AgentDescription)
						}

						return &protobufs.ServerToAgent{}
					},
				})

			s, _ := newSupervisor(t, "nocap", map[string]string{"url": server.addr})

			require.Nil(t, s.Start(t.Context()))
			defer s.Shutdown()

			waitForSupervisorConnection(server.supervisorConnected, true)

			require.Eventually(t, func() bool {
				ad, ok := agentDescription.Load().(*protobufs.AgentDescription)
				if !ok {
					return false
				}

				var agentName, agentVersion string
				identAttr := ad.IdentifyingAttributes
				for _, attr := range identAttr {
					switch attr.Key {
					case "service.name":
						agentName = attr.Value.GetStringValue()
					case "service.version":
						agentVersion = attr.Value.GetStringValue()
					}
				}

				// By default the Collector should report its name and version
				// from the component.BuildInfo struct built into the Collector
				// binary.
				return agentName == command && agentVersion == version
			}, 5*time.Second, 250*time.Millisecond)
		})
	}
}

func TestSupervisorBootstrapsCollectorAvailableComponents(t *testing.T) {
	agentDescription := atomic.Value{}
	availableComponents := atomic.Value{}

	// Load the Supervisor config so we can get the location of
	// the Collector that will be run.
	var cfg config.Supervisor
	cfgFile := getSupervisorConfig(t, "reports_available_components", map[string]string{})
	k := koanf.New("::")
	err := k.Load(file.Provider(cfgFile.Name()), yaml.Parser())
	require.NoError(t, err)
	err = k.UnmarshalWithConf("", &cfg, koanf.UnmarshalConf{
		Tag: "mapstructure",
	})
	require.NoError(t, err)

	// Get the binary name and version from the Collector binary
	// using the `components` command that prints a YAML-encoded
	// map of information about the Collector build. Some of this
	// information will be used as defaults for the telemetry
	// attributes.
	agentPath := cfg.Agent.Executable
	componentsInfo, err := exec.Command(agentPath, "components").Output()
	require.NoError(t, err)
	k = koanf.New("::")
	err = k.Load(rawbytes.Provider(componentsInfo), yaml.Parser())
	require.NoError(t, err)
	buildinfo := k.StringMap("buildinfo")
	command := buildinfo["command"]
	version := buildinfo["version"]

	server := newOpAMPServer(
		t,
		defaultConnectingHandler,
		types.ConnectionCallbacks{
			OnMessage: func(_ context.Context, _ types.Connection, message *protobufs.AgentToServer) *protobufs.ServerToAgent {
				if message.AgentDescription != nil {
					agentDescription.Store(message.AgentDescription)
				}

				response := &protobufs.ServerToAgent{}
				if message.AvailableComponents != nil {
					availableComponents.Store(message.AvailableComponents)

					if message.GetAvailableComponents().GetComponents() == nil {
						response.Flags = uint64(protobufs.ServerToAgentFlags_ServerToAgentFlags_ReportAvailableComponents)
					}
				}

				return response
			},
		})

	s, _ := newSupervisor(t, "reports_available_components", map[string]string{"url": server.addr})

	require.Nil(t, s.Start(t.Context()))
	defer s.Shutdown()

	waitForSupervisorConnection(server.supervisorConnected, true)

	require.Eventually(t, func() bool {
		ac, ok := availableComponents.Load().(*protobufs.AvailableComponents)
		if !ok {
			return false
		}

		if ac.GetComponents() == nil {
			return false
		}

		require.Len(t, ac.GetComponents(), 5) // connectors, exporters, extensions, processors, receivers
		require.NotNil(t, ac.GetComponents()["extensions"])
		require.NotNil(t, ac.GetComponents()["extensions"].GetSubComponentMap())
		require.NotNil(t, ac.GetComponents()["extensions"].GetSubComponentMap()["opamp"])

		ad, ok := agentDescription.Load().(*protobufs.AgentDescription)
		if !ok {
			return false
		}

		var agentName, agentVersion string
		identAttr := ad.IdentifyingAttributes
		for _, attr := range identAttr {
			switch attr.Key {
			case "service.name":
				agentName = attr.Value.GetStringValue()
			case "service.version":
				agentVersion = attr.Value.GetStringValue()
			}
		}

		// By default the Collector should report its name and version
		// from the component.BuildInfo struct built into the Collector
		// binary.
		return agentName == command && agentVersion == version
	}, 10*time.Second, 250*time.Millisecond)
}

func TestSupervisorReportsEffectiveConfig(t *testing.T) {
	var agentConfig atomic.Value
	server := newOpAMPServer(
		t,
		defaultConnectingHandler,
		types.ConnectionCallbacks{
			OnMessage: func(_ context.Context, _ types.Connection, message *protobufs.AgentToServer) *protobufs.ServerToAgent {
				if message.EffectiveConfig != nil {
					config := message.EffectiveConfig.ConfigMap.ConfigMap[""]
					if config != nil {
						agentConfig.Store(string(config.Body))
					}
				}

				return &protobufs.ServerToAgent{}
			},
		})

	s, _ := newSupervisor(t, "basic", map[string]string{"url": server.addr})

	require.Nil(t, s.Start(t.Context()))
	defer s.Shutdown()

	waitForSupervisorConnection(server.supervisorConnected, true)

	// Create input and output files so we can "communicate" with a Collector binary.
	// The testing package will automatically clean these up after each test.
	tempDir := t.TempDir()
	testKeyFile, err := os.CreateTemp(tempDir, "confKey")
	require.NoError(t, err)
	t.Cleanup(func() { testKeyFile.Close() })

	n, err := testKeyFile.Write([]byte(testKeyFile.Name()))
	require.NoError(t, err)
	require.NotZero(t, n)

	colCfgTpl, err := os.ReadFile(filepath.Join("testdata", "collector", "split_config.yaml"))
	require.NoError(t, err)

	templ, err := template.New("").Parse(string(colCfgTpl))
	require.NoError(t, err)

	var cfg bytes.Buffer
	err = templ.Execute(
		&cfg,
		map[string]string{
			"TestKeyFile": testKeyFile.Name(),
		},
	)
	require.NoError(t, err)

	h := sha256.New()
	if _, err := io.Copy(h, bytes.NewBuffer(cfg.Bytes())); err != nil {
		t.Fatal(err)
	}

	server.sendToSupervisor(&protobufs.ServerToAgent{
		RemoteConfig: &protobufs.AgentRemoteConfig{
			Config: &protobufs.AgentConfigMap{
				ConfigMap: map[string]*protobufs.AgentConfigFile{
					"": {Body: cfg.Bytes()},
				},
			},
			ConfigHash: h.Sum(nil),
		},
	})

	require.Eventually(t, func() bool {
		cfg, ok := agentConfig.Load().(string)
		if ok {
			// The effective config may be structurally different compared to what was sent.
			// Recent Collector versions may normalize telemetry resource keys into the
			// declarative `attributes` list, so accept both shapes.
			k := koanf.New("::")
			if err := k.Load(rawbytes.Provider([]byte(cfg)), yaml.Parser()); err != nil {
				return false
			}

			if k.Exists("service::telemetry::resource::test_key") {
				return true
			}

			attrs, ok := k.Get("service::telemetry::resource::attributes").([]any)
			if !ok {
				return false
			}

			for _, attr := range attrs {
				attrMap, ok := attr.(map[string]any)
				if !ok {
					continue
				}
				if name, ok := attrMap["name"].(string); ok && name == "test_key" {
					return true
				}
			}
		}

		return false
	}, 5*time.Second, 500*time.Millisecond, "Collector never reported effective config")
}

func TestSupervisorAgentDescriptionConfigApplies(t *testing.T) {
	// Load the Supervisor config so we can get the location of
	// the Collector that will be run.
	var cfg config.Supervisor
	cfgFile := getSupervisorConfig(t, "agent_description", map[string]string{})
	k := koanf.New("::")
	err := k.Load(file.Provider(cfgFile.Name()), yaml.Parser())
	require.NoError(t, err)
	err = k.UnmarshalWithConf("", &cfg, koanf.UnmarshalConf{
		Tag: "mapstructure",
	})
	require.NoError(t, err)

	host, err := os.Hostname()
	require.NoError(t, err)

	// Get the binary name and version from the Collector binary
	// using the `components` command that prints a YAML-encoded
	// map of information about the Collector build. Some of this
	// information will be used as defaults for the telemetry
	// attributes.
	agentPath := cfg.Agent.Executable
	componentsInfo, err := exec.Command(agentPath, "components").Output()
	require.NoError(t, err)
	k = koanf.New("::")
	err = k.Load(rawbytes.Provider(componentsInfo), yaml.Parser())
	require.NoError(t, err)
	buildinfo := k.StringMap("buildinfo")
	command := buildinfo["command"]
	version := buildinfo["version"]

	agentDescMessageChan := make(chan *protobufs.AgentToServer, 1)

	server := newOpAMPServer(
		t,
		defaultConnectingHandler,
		types.ConnectionCallbacks{
			OnMessage: func(_ context.Context, _ types.Connection, message *protobufs.AgentToServer) *protobufs.ServerToAgent {
				if message.AgentDescription != nil {
					select {
					case agentDescMessageChan <- message:
					default:
					}
				}

				return &protobufs.ServerToAgent{}
			},
		})

	s, _ := newSupervisor(t, "agent_description", map[string]string{"url": server.addr})

	require.Nil(t, s.Start(t.Context()))
	defer s.Shutdown()

	waitForSupervisorConnection(server.supervisorConnected, true)
	var ad *protobufs.AgentToServer
	select {
	case ad = <-agentDescMessageChan:
	case <-time.After(5 * time.Second):
		t.Fatal("Failed to get agent description after 5 seconds")
	}

	expectedIdentifyingAttributes := map[string]string{
		"client.id":           "my-client-id",
		"service.instance.id": uuid.UUID(ad.InstanceUid).String(),
		"service.name":        command,
		"service.version":     version,
	}
	expectedNonIdentifyingAttributes := map[string]string{
		"env":       "prod",
		"host.arch": runtime.GOARCH,
		"host.name": host,
		"os.type":   runtime.GOOS,
	}
	actualIdentifyingAttributes := keyValuesToStringMap(ad.AgentDescription.IdentifyingAttributes)
	require.Subset(t, actualIdentifyingAttributes, expectedIdentifyingAttributes)
	actualNonIdentifyingAttributes := keyValuesToStringMap(ad.AgentDescription.NonIdentifyingAttributes)
	require.Subset(t, actualNonIdentifyingAttributes, expectedNonIdentifyingAttributes)

	time.Sleep(250 * time.Millisecond)
}

func TestSupervisorForwardsUpdatedAgentDescriptionFromCollector(t *testing.T) {
	const updatedServiceName = "updated-agent-description-e2e"
	const updatedServiceVersion = "updated-version-e2e"
	const updatedResourceAttributeValue = "updated-resource-value"

	var agentDescription atomic.Value
	server := newOpAMPServer(
		t,
		defaultConnectingHandler,
		types.ConnectionCallbacks{
			OnMessage: func(_ context.Context, _ types.Connection, message *protobufs.AgentToServer) *protobufs.ServerToAgent {
				if message.AgentDescription != nil {
					agentDescription.Store(proto.Clone(message.AgentDescription).(*protobufs.AgentDescription))
				}

				return &protobufs.ServerToAgent{}
			},
		})

	s, _ := newSupervisor(t, "agent_description", map[string]string{
		"url":                         server.addr,
		"include_resource_attributes": "true",
	})

	require.NoError(t, s.Start(t.Context()))
	defer s.Shutdown()

	waitForSupervisorConnection(server.supervisorConnected, true)

	updatedConfig := []byte(fmt.Sprintf(`
receivers:
  nop:

exporters:
  nop:

service:
  pipelines:
    logs:
      receivers: [nop]
      exporters: [nop]
  telemetry:
    resource:
      service.name: %s
      service.version: %s
      test.resource.attr: %s
`, updatedServiceName, updatedServiceVersion, updatedResourceAttributeValue))
	updatedConfigHash := sha256.Sum256(updatedConfig)

	server.sendToSupervisor(&protobufs.ServerToAgent{
		RemoteConfig: &protobufs.AgentRemoteConfig{
			Config: &protobufs.AgentConfigMap{
				ConfigMap: map[string]*protobufs.AgentConfigFile{
					"": {Body: updatedConfig},
				},
			},
			ConfigHash: updatedConfigHash[:],
		},
	})

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		ad, ok := agentDescription.Load().(*protobufs.AgentDescription)
		require.True(c, ok)

		identifyingAttributes := keyValuesToStringMap(ad.IdentifyingAttributes)
		nonIdentifyingAttributes := keyValuesToStringMap(ad.NonIdentifyingAttributes)

		assert.Equal(c, updatedServiceName, identifyingAttributes["service.name"])
		assert.Equal(c, updatedServiceVersion, identifyingAttributes["service.version"])
		assert.Equal(c, "my-client-id", identifyingAttributes["client.id"])
		assert.Equal(c, "prod", nonIdentifyingAttributes["env"])
		assert.Equal(c, updatedResourceAttributeValue, nonIdentifyingAttributes["test.resource.attr"])
	}, 10*time.Second, 250*time.Millisecond)
}

func keyValuesToStringMap(kvs []*protobufs.KeyValue) map[string]string {
	out := make(map[string]string, len(kvs))
	for _, kv := range kvs {
		out[kv.Key] = kv.Value.GetStringValue()
	}
	return out
}

// Creates a Collector config that reads and writes logs to files and provides
// file descriptors for I/O operations to those files. The files are placed
// in a unique temp directory that is cleaned up after the test's completion.
func createSimplePipelineCollectorConf(t *testing.T) (*bytes.Buffer, []byte, *os.File, *os.File) {
	wd, err := os.Getwd()
	require.NoError(t, err)

	// Create input and output files so we can "communicate" with a Collector binary.
	// The testing package will automatically clean these up after each test.
	tempDir := t.TempDir()
	inputFile, err := os.CreateTemp(tempDir, "input_*.yaml")
	require.NoError(t, err)
	t.Cleanup(func() { inputFile.Close() })

	outputFile, err := os.CreateTemp(tempDir, "output_*.yaml")
	require.NoError(t, err)
	t.Cleanup(func() { outputFile.Close() })

	colCfgTpl, err := os.ReadFile(path.Join(wd, "testdata", "collector", "simple_pipeline.yaml"))
	require.NoError(t, err)

	templ, err := template.New("").Parse(string(colCfgTpl))
	require.NoError(t, err)

	var confmapBuf bytes.Buffer
	err = templ.Execute(
		&confmapBuf,
		map[string]string{
			"inputLogFile":  inputFile.Name(),
			"outputLogFile": outputFile.Name(),
		},
	)
	require.NoError(t, err)

	h := sha256.New()
	if _, err := io.Copy(h, bytes.NewBuffer(confmapBuf.Bytes())); err != nil {
		log.Fatal(err)
	}

	return &confmapBuf, h.Sum(nil), inputFile, outputFile
}

// Creates a Collector config that contains no pipeline
func createEmptyPipelineCollectorConf(t *testing.T) (*bytes.Buffer, []byte) {
	wd, err := os.Getwd()
	require.NoError(t, err)

	colCfgTpl, err := os.ReadFile(path.Join(wd, "testdata", "collector", "empty_pipeline.yaml"))
	require.NoError(t, err)

	templ, err := template.New("").Parse(string(colCfgTpl))
	require.NoError(t, err)

	var confmapBuf bytes.Buffer
	err = templ.Execute(
		&confmapBuf,
		map[string]string{},
	)
	require.NoError(t, err)

	h := sha256.New()
	if _, err := io.Copy(h, bytes.NewBuffer(confmapBuf.Bytes())); err != nil {
		log.Fatal(err)
	}

	return &confmapBuf, h.Sum(nil)
}

func createBadCollectorConf(t *testing.T) (*bytes.Buffer, []byte) {
	colCfg, err := os.ReadFile(path.Join("testdata", "collector", "bad_config.yaml"))
	require.NoError(t, err)

	h := sha256.New()
	if _, err := io.Copy(h, bytes.NewBuffer(colCfg)); err != nil {
		log.Fatal(err)
	}

	return bytes.NewBuffer(colCfg), h.Sum(nil)
}

func createHealthCheckCollectorConf(t *testing.T, nopPipeline bool) (cfg *bytes.Buffer, hash []byte, remotePort int) {
	colCfgTpl, err := os.ReadFile(path.Join("testdata", "collector", "healthcheck_config.tmpl.yaml"))
	require.NoError(t, err)

	templ, err := template.New("").Parse(string(colCfgTpl))
	require.NoError(t, err)

	var confmapBuf bytes.Buffer
	err = templ.Execute(
		&confmapBuf,
		map[string]any{
			"nopPipeline": nopPipeline,
		},
	)
	require.NoError(t, err)

	h := sha256.Sum256(confmapBuf.Bytes())

	return &confmapBuf, h[:], 13133
}

func createHostMetricsCollectorConf(t *testing.T) (*bytes.Buffer, []byte) {
	wd, err := os.Getwd()
	require.NoError(t, err)

	// Create output files
	// The testing package will automatically clean these up after each test.
	tempDir := t.TempDir()
	outputFile, err := os.CreateTemp(tempDir, "output_*.json")
	require.NoError(t, err)
	t.Cleanup(func() { outputFile.Close() })

	colCfgTpl, err := os.ReadFile(path.Join(wd, "testdata", "collector", "hostmetrics_pipeline.yaml"))
	require.NoError(t, err)

	templ, err := template.New("").Parse(string(colCfgTpl))
	require.NoError(t, err)

	var confmapBuf bytes.Buffer
	err = templ.Execute(
		&confmapBuf,
		map[string]string{
			"outputLogFile": outputFile.Name(),
		},
	)
	require.NoError(t, err)

	h := sha256.New()
	if _, err := io.Copy(h, bytes.NewBuffer(confmapBuf.Bytes())); err != nil {
		log.Fatal(err)
	}

	return &confmapBuf, h.Sum(nil)
}

func healthCheckOK(port int) bool {
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d", port))
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode >= 200 && resp.StatusCode < 300
}

func claimPortWhenFree(t *testing.T, port int) <-chan net.Listener {
	t.Helper()

	claimed := make(chan net.Listener, 1)
	go func() {
		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()

		for {
			listener, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", port))
			if err == nil {
				claimed <- listener
				return
			}

			select {
			case <-t.Context().Done():
				return
			case <-ticker.C:
			}
		}
	}()
	return claimed
}

// Wait for the Supervisor to connect to or disconnect from the OpAMP server
func waitForSupervisorConnection(connection chan bool, connected bool) {
	select {
	case <-time.After(5 * time.Second):
		break
	case state := <-connection:
		if state == connected {
			break
		}
	}
}

func TestSupervisorRestartCommand(t *testing.T) {
	modes := getTestModes()

	for _, mode := range modes {
		t.Run(mode.name, func(t *testing.T) {
			var healthReport atomic.Value
			var agentConfig atomic.Value
			server := newOpAMPServer(
				t,
				defaultConnectingHandler,
				types.ConnectionCallbacks{
					OnMessage: func(_ context.Context, _ types.Connection, message *protobufs.AgentToServer) *protobufs.ServerToAgent {
						if message.Health != nil {
							healthReport.Store(message.Health)
						}

						if message.EffectiveConfig != nil {
							config := message.EffectiveConfig.ConfigMap.ConfigMap[""]
							if config != nil {
								agentConfig.Store(string(config.Body))
							}
						}
						return &protobufs.ServerToAgent{}
					},
				})

			storageDir := t.TempDir()
			extraConfigData := map[string]string{
				"url":         server.addr,
				"storage_dir": storageDir,
			}
			if mode.UseHUPConfigReload {
				extraConfigData["use_hup_config_reload"] = "true"
			}

			s, supervisorCfg := newSupervisor(t, "basic", extraConfigData)
			if mode.UseHUPConfigReload {
				require.True(t, supervisorCfg.Agent.UseHUPConfigReload)
			}

			require.Nil(t, s.Start(t.Context()))
			defer s.Shutdown()

			waitForSupervisorConnection(server.supervisorConnected, true)

			// Send the initial config
			cfg, hash, _, _ := createSimplePipelineCollectorConf(t)

			server.sendToSupervisor(&protobufs.ServerToAgent{
				RemoteConfig: &protobufs.AgentRemoteConfig{
					Config: &protobufs.AgentConfigMap{
						ConfigMap: map[string]*protobufs.AgentConfigFile{
							"": {Body: cfg.Bytes()},
						},
					},
					ConfigHash: hash,
				},
			})

			require.Eventually(t, func() bool {
				cfg, ok := agentConfig.Load().(string)
				if ok {
					return strings.Contains(cfg, "health_check")
				}
				return false
			}, 5*time.Second, 500*time.Millisecond, "Collector was not started with healthcheck")

			require.Eventually(t, func() bool {
				health := healthReport.Load().(*protobufs.ComponentHealth)

				if health != nil {
					return health.Healthy && health.LastError == ""
				}

				return false
			}, 5*time.Second, 500*time.Millisecond, "Collector never became healthy")

			// The health report should be received after the restart
			healthReport.Store(&protobufs.ComponentHealth{})

			server.sendToSupervisor(&protobufs.ServerToAgent{
				Command: &protobufs.ServerToAgentCommand{
					Type: protobufs.CommandType_CommandType_Restart,
				},
			})

			// Here we will wait for the supervisor connection to go away and come back.
			// This helps us check that the restart logic is actually restarting the agent.
			// Note: this also helps the data race detector confirm that access to the
			// [Supervisor.lastHealthFromClient] has to be synchronized to prevent races.
			waitForSupervisorConnection(server.supervisorConnected, false)
			waitForSupervisorConnection(server.supervisorConnected, true)

			server.sendToSupervisor(&protobufs.ServerToAgent{
				Flags: uint64(protobufs.ServerToAgentFlags_ServerToAgentFlags_ReportFullState),
			})

			require.Eventually(t, func() bool {
				health := healthReport.Load().(*protobufs.ComponentHealth)
				if health != nil {
					return health.Healthy && health.LastError == ""
				}
				return false
			}, 30*time.Second, 250*time.Millisecond, "Collector never reported healthy after restart")
		})
	}
}

func TestSupervisorOpAMPConnectionSettings(t *testing.T) {
	var connectedToNewServer atomic.Bool
	initialServer := newOpAMPServer(
		t,
		defaultConnectingHandler,
		types.ConnectionCallbacks{})

	s, _ := newSupervisor(t, "accepts_conn", map[string]string{"url": initialServer.addr})

	require.Nil(t, s.Start(t.Context()))
	defer s.Shutdown()

	waitForSupervisorConnection(initialServer.supervisorConnected, true)

	newServer := newOpAMPServer(
		t,
		defaultConnectingHandler,
		types.ConnectionCallbacks{
			OnConnected: func(context.Context, types.Connection) {
				connectedToNewServer.Store(true)
			},
			OnMessage: func(context.Context, types.Connection, *protobufs.AgentToServer) *protobufs.ServerToAgent {
				return &protobufs.ServerToAgent{}
			},
		})

	initialServer.sendToSupervisor(&protobufs.ServerToAgent{
		ConnectionSettings: &protobufs.ConnectionSettingsOffers{
			Opamp: &protobufs.OpAMPConnectionSettings{
				DestinationEndpoint: "ws://" + newServer.addr + "/v1/opamp",
				Headers: &protobufs.Headers{
					Headers: []*protobufs.Header{
						{
							Key:   "x-foo",
							Value: "bar",
						},
					},
				},
			},
		},
	})
	waitForSupervisorConnection(newServer.supervisorConnected, true)

	require.Eventually(t, func() bool {
		return connectedToNewServer.Load() == true
	}, 10*time.Second, 500*time.Millisecond, "Collector did not connect to new OpAMP server")
}

func TestSupervisorOpAMPWithHTTPEndpoint(t *testing.T) {
	connected := atomic.Bool{}
	initialServer := newOpAMPServer(
		t,
		defaultConnectingHandler,
		types.ConnectionCallbacks{
			OnConnected: func(ctx context.Context, conn types.Connection) {
				connected.Store(true)
			},
		})

	s, _ := newSupervisor(t, "http", map[string]string{"url": initialServer.addr})

	require.Nil(t, s.Start(t.Context()))
	defer s.Shutdown()

	waitForSupervisorConnection(initialServer.supervisorConnected, true)
	require.True(t, connected.Load(), "Supervisor failed to connect")
}

func TestSupervisorRestartsWithLastReceivedConfig(t *testing.T) {
	modes := getTestModes()

	for _, mode := range modes {
		t.Run(mode.name, func(t *testing.T) {
			// Create a temporary directory to store the test config file.
			tempDir := t.TempDir()

			var agentConfig atomic.Value
			var initialRemoteConfigStatus atomic.Value
			initialServer := newOpAMPServer(
				t,
				defaultConnectingHandler,
				types.ConnectionCallbacks{
					OnMessage: func(_ context.Context, _ types.Connection, message *protobufs.AgentToServer) *protobufs.ServerToAgent {
						if message.EffectiveConfig != nil {
							config := message.EffectiveConfig.ConfigMap.ConfigMap[""]
							if config != nil {
								agentConfig.Store(string(config.Body))
							}
						}
						if message.RemoteConfigStatus != nil {
							initialRemoteConfigStatus.Store(message.RemoteConfigStatus)
						}
						return &protobufs.ServerToAgent{}
					},
				})

			extraConfigData := map[string]string{"url": initialServer.addr, "storage_dir": tempDir}
			if mode.UseHUPConfigReload {
				extraConfigData["use_hup_config_reload"] = "true"
			}

			s, supervisorCfg := newSupervisor(t, "persistence", extraConfigData)
			if mode.UseHUPConfigReload {
				require.True(t, supervisorCfg.Agent.UseHUPConfigReload)
			}

			require.Nil(t, s.Start(t.Context()))

			waitForSupervisorConnection(initialServer.supervisorConnected, true)

			cfg, hash, _, _ := createSimplePipelineCollectorConf(t)

			initialServer.sendToSupervisor(&protobufs.ServerToAgent{
				RemoteConfig: &protobufs.AgentRemoteConfig{
					Config: &protobufs.AgentConfigMap{
						ConfigMap: map[string]*protobufs.AgentConfigFile{
							"": {Body: cfg.Bytes()},
						},
					},
					ConfigHash: hash,
				},
			})

			require.Eventually(t, func() bool {
				// Check if the config file was written to the storage directory
				_, err := os.Stat(path.Join(tempDir, "last_recv_remote_config.dat"))
				return err == nil
			}, 5*time.Second, 250*time.Millisecond, "Config file was not written to persistent storage directory")

			agentConfig.Store("")
			s.Shutdown()
			initialServer.shutdown()

			newServer := newOpAMPServer(
				t,
				defaultConnectingHandler,
				types.ConnectionCallbacks{
					OnMessage: func(_ context.Context, _ types.Connection, message *protobufs.AgentToServer) *protobufs.ServerToAgent {
						if message.EffectiveConfig != nil {
							config := message.EffectiveConfig.ConfigMap.ConfigMap[""]
							if config != nil {
								agentConfig.Store(string(config.Body))
							}
						}
						return &protobufs.ServerToAgent{}
					},
				})
			defer newServer.shutdown()

			extraConfigData["url"] = newServer.addr
			s1, supervisorCfg := newSupervisor(t, "persistence", extraConfigData)
			if mode.UseHUPConfigReload {
				require.True(t, supervisorCfg.Agent.UseHUPConfigReload)
			}

			require.Nil(t, s1.Start(t.Context()))
			defer s1.Shutdown()

			waitForSupervisorConnection(newServer.supervisorConnected, true)

			newServer.sendToSupervisor(&protobufs.ServerToAgent{
				Flags: uint64(protobufs.ServerToAgentFlags_ServerToAgentFlags_ReportFullState),
			})

			// Check that the new Supervisor instance starts with the configuration from the last received remote config
			require.Eventually(t, func() bool {
				loadedConfig, ok := agentConfig.Load().(string)
				if !ok {
					return false
				}

				return strings.Contains(loadedConfig, "file_log")
			}, 10*time.Second, 500*time.Millisecond, "Collector was not started with the last received remote config")
		})
	}
}

func TestSupervisorPersistsInstanceID(t *testing.T) {
	// Tests shutting down and starting up a new supervisor will
	// persist and re-use the same instance ID.
	storageDir := t.TempDir()

	agentIDChan := make(chan []byte, 1)
	server := newOpAMPServer(
		t,
		defaultConnectingHandler,
		types.ConnectionCallbacks{
			OnMessage: func(_ context.Context, _ types.Connection, message *protobufs.AgentToServer) *protobufs.ServerToAgent {
				select {
				case agentIDChan <- message.InstanceUid:
				default:
				}

				return &protobufs.ServerToAgent{}
			},
		})

	s, _ := newSupervisor(t, "basic", map[string]string{
		"url":         server.addr,
		"storage_dir": storageDir,
	})

	require.Nil(t, s.Start(t.Context()))

	waitForSupervisorConnection(server.supervisorConnected, true)

	t.Logf("Supervisor connected")

	var firstAgentID []byte
	select {
	case firstAgentID = <-agentIDChan:
	case <-time.After(1 * time.Second):
		t.Fatalf("failed to get first agent ID")
	}

	t.Logf("Got agent ID %s, shutting down supervisor", uuid.UUID(firstAgentID))

	s.Shutdown()

	waitForSupervisorConnection(server.supervisorConnected, false)

	t.Logf("Supervisor disconnected")

	// Drain agent ID channel so we get a fresh ID from the new supervisor
	select {
	case <-agentIDChan:
	default:
	}

	s, _ = newSupervisor(t, "basic", map[string]string{
		"url":         server.addr,
		"storage_dir": storageDir,
	})

	require.Nil(t, s.Start(t.Context()))
	defer s.Shutdown()

	waitForSupervisorConnection(server.supervisorConnected, true)

	t.Logf("Supervisor connected")

	var secondAgentID []byte
	select {
	case secondAgentID = <-agentIDChan:
	case <-time.After(1 * time.Second):
		t.Fatalf("failed to get second agent ID")
	}

	require.Equal(t, firstAgentID, secondAgentID)
}

func TestSupervisorPersistsNewInstanceID(t *testing.T) {
	// Tests that an agent ID that is given from the server to the agent in an AgentIdentification message
	// is properly persisted.
	storageDir := t.TempDir()

	newID := uuid.MustParse("018fee23-4a51-7303-a441-73faed7d9deb")

	agentIDChan := make(chan []byte, 1)
	server := newOpAMPServer(
		t,
		defaultConnectingHandler,
		types.ConnectionCallbacks{
			OnMessage: func(_ context.Context, _ types.Connection, message *protobufs.AgentToServer) *protobufs.ServerToAgent {
				select {
				case agentIDChan <- message.InstanceUid:
				default:
				}

				if !bytes.Equal(message.InstanceUid, newID[:]) {
					return &protobufs.ServerToAgent{
						InstanceUid: message.InstanceUid,
						AgentIdentification: &protobufs.AgentIdentification{
							NewInstanceUid: newID[:],
						},
					}
				}

				return &protobufs.ServerToAgent{}
			},
		})

	s, _ := newSupervisor(t, "basic", map[string]string{
		"url":         server.addr,
		"storage_dir": storageDir,
	})

	require.Nil(t, s.Start(t.Context()))

	waitForSupervisorConnection(server.supervisorConnected, true)

	t.Logf("Supervisor connected")

	for id := range agentIDChan {
		if bytes.Equal(id, newID[:]) {
			t.Logf("Agent ID was changed to new ID")
			break
		}
	}

	s.Shutdown()

	waitForSupervisorConnection(server.supervisorConnected, false)

	t.Logf("Supervisor disconnected")

	// Drain agent ID channel so we get a fresh ID from the new supervisor
	select {
	case <-agentIDChan:
	default:
	}

	s, _ = newSupervisor(t, "basic", map[string]string{
		"url":         server.addr,
		"storage_dir": storageDir,
	})

	require.Nil(t, s.Start(t.Context()))
	defer s.Shutdown()

	waitForSupervisorConnection(server.supervisorConnected, true)

	t.Logf("Supervisor connected")

	var newRecievedAgentID []byte
	select {
	case newRecievedAgentID = <-agentIDChan:
	case <-time.After(1 * time.Second):
		t.Fatalf("failed to get second agent ID")
	}

	require.Equal(t, newID, uuid.UUID(newRecievedAgentID))
}

func TestSupervisorWritesAgentFilesToStorageDir(t *testing.T) {
	// Tests that the agent logs and effective.yaml are written under the storage directory.
	storageDir := t.TempDir()

	server := newOpAMPServer(
		t,
		defaultConnectingHandler,
		types.ConnectionCallbacks{},
	)

	s, _ := newSupervisor(t, "basic", map[string]string{
		"url":         server.addr,
		"storage_dir": storageDir,
	})

	require.Nil(t, s.Start(t.Context()))

	waitForSupervisorConnection(server.supervisorConnected, true)

	t.Logf("Supervisor connected")

	s.Shutdown()

	t.Logf("Supervisor shutdown")

	// Check config and log files are written in storage dir
	require.FileExists(t, filepath.Join(storageDir, "agent.log"))
	require.FileExists(t, filepath.Join(storageDir, "effective.yaml"))
}

func TestSupervisorStopsAgentProcessWithEmptyConfigMap(t *testing.T) {
	agentCfgChan := make(chan string, 1)
	var healthReport atomic.Value
	var remoteConfigStatus atomic.Value
	server := newOpAMPServer(
		t,
		defaultConnectingHandler,
		types.ConnectionCallbacks{
			OnMessage: func(_ context.Context, _ types.Connection, message *protobufs.AgentToServer) *protobufs.ServerToAgent {
				if message.EffectiveConfig != nil {
					config := message.EffectiveConfig.ConfigMap.ConfigMap[""]
					if config != nil {
						select {
						case agentCfgChan <- string(config.Body):
						default:
						}
					}
				}
				if message.Health != nil {
					healthReport.Store(message.Health)
				}
				if message.RemoteConfigStatus != nil {
					remoteConfigStatus.Store(message.RemoteConfigStatus)
				}
				return &protobufs.ServerToAgent{}
			},
		})

	s, _ := newSupervisor(t, "healthcheck_port", map[string]string{
		"url": server.addr,
	})

	require.Nil(t, s.Start(t.Context()))
	defer s.Shutdown()

	waitForSupervisorConnection(server.supervisorConnected, true)

	cfg, hash, _, _ := createSimplePipelineCollectorConf(t)

	server.sendToSupervisor(&protobufs.ServerToAgent{
		RemoteConfig: &protobufs.AgentRemoteConfig{
			Config: &protobufs.AgentConfigMap{
				ConfigMap: map[string]*protobufs.AgentConfigFile{
					"": {Body: cfg.Bytes()},
				},
			},
			ConfigHash: hash,
		},
	})

	select {
	case <-agentCfgChan:
	case <-time.After(1 * time.Second):
		require.FailNow(t, "timed out waitng for agent to report its initial config")
	}

	// Use health check endpoint to determine if the collector is actually running
	require.Eventually(t, func() bool {
		resp, err := http.DefaultClient.Get("http://localhost:13133")
		if err != nil {
			t.Logf("Failed agent healthcheck request: %s", err)
			return false
		}
		require.NoError(t, resp.Body.Close())
		if resp.StatusCode >= 300 || resp.StatusCode < 200 {
			t.Logf("Got non-2xx status code: %d", resp.StatusCode)
			return false
		}
		return true
	}, 3*time.Second, 100*time.Millisecond)

	// Send empty config
	emptyHash := sha256.Sum256([]byte{})
	server.sendToSupervisor(&protobufs.ServerToAgent{
		RemoteConfig: &protobufs.AgentRemoteConfig{
			Config: &protobufs.AgentConfigMap{
				ConfigMap: map[string]*protobufs.AgentConfigFile{},
			},
			ConfigHash: emptyHash[:],
		},
	})

	select {
	case <-agentCfgChan:
	case <-time.After(1 * time.Second):
		require.FailNow(t, "timed out waitng for agent to report its noop config")
	}

	// Verify the collector is not running after 250 ms by checking the healthcheck endpoint
	require.EventuallyWithT(t, func(tt *assert.CollectT) {
		_, err := http.DefaultClient.Get("http://localhost:12345")
		if runtime.GOOS != "windows" {
			assert.ErrorContains(tt, err, "connection refused")
		} else {
			assert.ErrorContains(tt, err, "No connection could be made")
		}
	}, 3*time.Second, 250*time.Millisecond)

	// Verify we have a healthy status (if it was ran with the empty config it would be healthy)
	require.Eventually(t, func() bool {
		health, ok := healthReport.Load().(*protobufs.ComponentHealth)
		return ok && health.Healthy
	}, 3*time.Second, 250*time.Millisecond)

	// Verify the status is set to APPLIED (if it was ran with the empty config it would be APPLIED)
	require.Eventually(t, func() bool {
		status, ok := remoteConfigStatus.Load().(*protobufs.RemoteConfigStatus)
		return ok && status.Status == protobufs.RemoteConfigStatuses_RemoteConfigStatuses_APPLIED
	}, 3*time.Second, 250*time.Millisecond)
}

type logEntry struct {
	Level  string `json:"level"`
	Logger string `json:"logger"`
}

func TestSupervisorLogging(t *testing.T) {
	// Tests that supervisor only logs at Info level and above && that collector logs passthrough and are present in supervisor log file
	if runtime.GOOS == "windows" {
		t.Skip("Zap does not close the log file and Windows disallows removing files that are still opened.")
	}

	storageDir := t.TempDir()
	remoteCfgFilePath := filepath.Join(storageDir, "last_recv_remote_config.dat")

	collectorCfg, hash := createHostMetricsCollectorConf(t)
	remoteCfgProto := &protobufs.AgentRemoteConfig{
		Config: &protobufs.AgentConfigMap{
			ConfigMap: map[string]*protobufs.AgentConfigFile{
				"": {Body: collectorCfg.Bytes()},
			},
		},
		ConfigHash: hash,
	}
	marshalledRemoteCfg, err := proto.Marshal(remoteCfgProto)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(remoteCfgFilePath, marshalledRemoteCfg, 0o600))

	connected := atomic.Bool{}
	server := newUnstartedOpAMPServer(t, defaultConnectingHandler, types.ConnectionCallbacks{
		OnConnected: func(ctx context.Context, conn types.Connection) {
			connected.Store(true)
		},
	})
	defer server.shutdown()
	server.start()

	// manually create supervisor and logger for this test
	supervisorLogFilePath := filepath.Join(storageDir, "supervisor_log.log")
	cfgFile := getSupervisorConfig(t, "logging", map[string]string{
		"url":         server.addr,
		"storage_dir": storageDir,
		"log_level":   "0",
		"log_file":    supervisorLogFilePath,
	})
	cfg, err := config.Load(cfgFile.Name())
	require.NoError(t, err)
	logger, err := telemetry.NewLogger(cfg.Telemetry.Logs)
	require.NoError(t, err)

	s, err := supervisor.NewSupervisor(t.Context(), logger, cfg)
	require.NoError(t, err)
	require.Nil(t, s.Start(t.Context()))

	waitForSupervisorConnection(server.supervisorConnected, true)
	require.True(t, connected.Load(), "Supervisor failed to connect")

	s.Shutdown()

	// Read from log file checking for Info level logs
	logFile, err := os.Open(supervisorLogFilePath)
	require.NoError(t, err)
	defer logFile.Close()

	reader := bufio.NewReader(logFile)
	seenCollectorLog := false
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			break
		}
		line = strings.TrimRight(line, "\r\n")

		var log logEntry
		err = json.Unmarshal([]byte(line), &log)
		require.NoError(t, err)

		level, err := zapcore.ParseLevel(log.Level)
		require.NoError(t, err)
		require.GreaterOrEqual(t, level, zapcore.InfoLevel)

		if log.Logger == "collector" {
			seenCollectorLog = true
		}
	}
	// verify a collector log was read
	require.True(t, seenCollectorLog)
}

func TestSupervisorRemoteConfigApplyStatus(t *testing.T) {
	modes := getTestModes()

	for _, mode := range modes {
		t.Run(mode.name, func(t *testing.T) {
			var agentConfig atomic.Value
			var healthReport atomic.Value
			var remoteConfigStatus atomic.Value
			server := newOpAMPServer(
				t,
				defaultConnectingHandler,
				types.ConnectionCallbacks{
					OnMessage: func(_ context.Context, _ types.Connection, message *protobufs.AgentToServer) *protobufs.ServerToAgent {
						if message.EffectiveConfig != nil {
							config := message.EffectiveConfig.ConfigMap.ConfigMap[""]
							if config != nil {
								agentConfig.Store(string(config.Body))
							}
						}
						if message.Health != nil {
							healthReport.Store(message.Health)
						}
						if message.RemoteConfigStatus != nil {
							remoteConfigStatus.Store(message.RemoteConfigStatus)
						}

						return &protobufs.ServerToAgent{}
					},
				})

			outputPath := filepath.Join(t.TempDir(), "output.txt")
			backend := testbed.NewOTLPHTTPDataReceiver(4318)
			mockBackend := testbed.NewMockBackend(outputPath, backend)
			mockBackend.EnableRecording()
			defer mockBackend.Stop()
			require.NoError(t, mockBackend.Start())

			extraConfigData := map[string]string{
				"url":                  server.addr,
				"config_apply_timeout": "3s",
				"telemetryUrl":         "localhost:4318",
			}
			if mode.UseHUPConfigReload {
				extraConfigData["use_hup_config_reload"] = "true"
			}

			s, supervisorCfg := newSupervisor(t, "report_status", extraConfigData)
			if mode.UseHUPConfigReload {
				require.True(t, supervisorCfg.Agent.UseHUPConfigReload)
			}
			require.Nil(t, s.Start(t.Context()))
			defer s.Shutdown()

			waitForSupervisorConnection(server.supervisorConnected, true)

			cfg, hash, inputFile, outputFile := createSimplePipelineCollectorConf(t)

			server.sendToSupervisor(&protobufs.ServerToAgent{
				RemoteConfig: &protobufs.AgentRemoteConfig{
					Config: &protobufs.AgentConfigMap{
						ConfigMap: map[string]*protobufs.AgentConfigFile{
							"": {Body: cfg.Bytes()},
						},
					},
					ConfigHash: hash,
				},
			})

			// Check that the status is set to APPLYING
			require.EventuallyWithT(t, func(c *assert.CollectT) {
				statusVal := remoteConfigStatus.Load()
				require.NotNil(c, statusVal) // not set yet
				status := statusVal.(*protobufs.RemoteConfigStatus)
				t.Log("status", status.Status)
				assert.Equal(c, protobufs.RemoteConfigStatuses_RemoteConfigStatuses_APPLYING, status.Status)
			}, 5*time.Second, 100*time.Millisecond)

			// Wait for collector to become healthy
			require.Eventually(t, func() bool {
				health, ok := healthReport.Load().(*protobufs.ComponentHealth)
				return ok && health.Healthy
			}, 10*time.Second, 10*time.Millisecond, "Collector did not become healthy")

			// Check that the status is set to APPLIED
			require.Eventually(t, func() bool {
				status, ok := remoteConfigStatus.Load().(*protobufs.RemoteConfigStatus)
				return ok && status.Status == protobufs.RemoteConfigStatuses_RemoteConfigStatuses_APPLIED
			}, 5*time.Second, 100*time.Millisecond, "Remote config status was not set to APPLIED")

			require.Eventually(t, func() bool {
				cfg, ok := agentConfig.Load().(string)
				if ok {
					// The effective config may be structurally different compared to what was sent,
					// and will also have some data redacted,
					// so just check that it includes the file_log receiver
					return strings.Contains(cfg, "file_log")
				}

				return false
			}, 5*time.Second, 10*time.Millisecond, "Collector was not started with remote config")

			n, err := inputFile.WriteString("{\"body\":\"hello, world\"}\n")
			require.NotZero(t, n, "Could not write to input file")
			require.NoError(t, err)

			require.Eventually(t, func() bool {
				logRecord := make([]byte, 1024)
				n, _ := outputFile.Read(logRecord)

				return n != 0
			}, 10*time.Second, 100*time.Millisecond, "Log never appeared in output")

			t.Run("bad config", func(t *testing.T) {
				// Test with bad configuration
				badCfg, badHash := createBadCollectorConf(t)

				server.sendToSupervisor(&protobufs.ServerToAgent{
					RemoteConfig: &protobufs.AgentRemoteConfig{
						Config: &protobufs.AgentConfigMap{
							ConfigMap: map[string]*protobufs.AgentConfigFile{
								"": {Body: badCfg.Bytes()},
							},
						},
						ConfigHash: badHash,
					},
				})

				// Wait for the health checks to fail
				require.Eventually(t, func() bool {
					health, ok := healthReport.Load().(*protobufs.ComponentHealth)
					return ok && !health.Healthy
				}, 30*time.Second, 100*time.Millisecond, "Collector did not become unhealthy with bad config")

				// Check that the status is set to FAILED after failed health checks
				require.Eventually(t, func() bool {
					status, ok := remoteConfigStatus.Load().(*protobufs.RemoteConfigStatus)
					return ok && status.Status == protobufs.RemoteConfigStatuses_RemoteConfigStatuses_FAILED
				}, 15*time.Second, 100*time.Millisecond, "Remote config status was not set to FAILED for bad config")

				// Test with nop configuration
				emptyHash := sha256.Sum256([]byte{})
				server.sendToSupervisor(&protobufs.ServerToAgent{
					RemoteConfig: &protobufs.AgentRemoteConfig{
						Config: &protobufs.AgentConfigMap{
							ConfigMap: map[string]*protobufs.AgentConfigFile{},
						},
						ConfigHash: emptyHash[:],
					},
				})

				// Check that the status is set to APPLIED
				require.Eventually(t, func() bool {
					status, ok := remoteConfigStatus.Load().(*protobufs.RemoteConfigStatus)
					return ok && status.Status == protobufs.RemoteConfigStatuses_RemoteConfigStatuses_APPLIED
				}, 5*time.Second, 10*time.Millisecond, "Remote config status was not set to APPLIED for empty config")

				gotSpans := []string{}
				expectedSpans := []string{"GetBootstrapInfo", "onMessage"}
				require.EventuallyWithT(t, func(collect *assert.CollectT) {
					require.GreaterOrEqual(collect, len(mockBackend.GetReceivedTraces()), len(expectedSpans))
				}, 10*time.Second, 250*time.Millisecond)

				receivedTraces := mockBackend.GetReceivedTraces()
				for i := 0; i < len(receivedTraces); i++ {
					gotSpans = append(gotSpans, receivedTraces[i].ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Name())
				}

				for _, expectedSpan := range expectedSpans {
					require.Contains(t, gotSpans, expectedSpan)
				}
			})
		})
	}
}

func TestSupervisorReportsCollectorLogTailOnRemoteConfigCrash(t *testing.T) {
	var healthReport atomic.Value
	var remoteConfigStatus atomic.Value
	server := newOpAMPServer(
		t,
		defaultConnectingHandler,
		types.ConnectionCallbacks{
			OnMessage: func(_ context.Context, _ types.Connection, message *protobufs.AgentToServer) *protobufs.ServerToAgent {
				if message.Health != nil {
					healthReport.Store(message.Health)
				}
				if message.RemoteConfigStatus != nil {
					remoteConfigStatus.Store(message.RemoteConfigStatus)
				}

				return &protobufs.ServerToAgent{}
			},
		})

	storageDir := t.TempDir()
	s, _ := newSupervisor(t, "basic", map[string]string{
		"url":                             server.addr,
		"storage_dir":                     storageDir,
		"collector_crash_log_snippet_kib": "4",
	})
	require.NoError(t, s.Start(t.Context()))
	defer s.Shutdown()

	waitForSupervisorConnection(server.supervisorConnected, true)

	badCfg, badHash := createBadCollectorConf(t)
	server.sendToSupervisor(&protobufs.ServerToAgent{
		RemoteConfig: &protobufs.AgentRemoteConfig{
			Config: &protobufs.AgentConfigMap{
				ConfigMap: map[string]*protobufs.AgentConfigFile{
					"": {Body: badCfg.Bytes()},
				},
			},
			ConfigHash: badHash,
		},
	})

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		health, ok := healthReport.Load().(*protobufs.ComponentHealth)
		require.True(c, ok)
		assert.False(c, health.Healthy)
		assert.Contains(c, health.LastError, "Collector log tail:")
		assert.Contains(c, health.LastError, "failed to get config")
	}, 15*time.Second, 100*time.Millisecond, "Supervisor did not report Collector log tail in health error")

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		status, ok := remoteConfigStatus.Load().(*protobufs.RemoteConfigStatus)
		require.True(c, ok)
		assert.Equal(c, protobufs.RemoteConfigStatuses_RemoteConfigStatuses_FAILED, status.Status)
		assert.Contains(c, status.ErrorMessage, "Collector log tail:")
		assert.Contains(c, status.ErrorMessage, "failed to get config")
	}, 15*time.Second, 100*time.Millisecond, "Supervisor did not report Collector log tail in remote config status")
}

func TestSupervisorOpAmpServerPort(t *testing.T) {
	var agentConfig atomic.Value
	server := newOpAMPServer(
		t,
		defaultConnectingHandler,
		types.ConnectionCallbacks{
			OnMessage: func(_ context.Context, _ types.Connection, message *protobufs.AgentToServer) *protobufs.ServerToAgent {
				if message.EffectiveConfig != nil {
					config := message.EffectiveConfig.ConfigMap.ConfigMap[""]
					if config != nil {
						agentConfig.Store(string(config.Body))
					}
				}

				return &protobufs.ServerToAgent{}
			},
		})

	supervisorOpAmpServerPort, err := findRandomPort()
	require.NoError(t, err)

	s, _ := newSupervisor(t, "server_port", map[string]string{"url": server.addr, "supervisor_opamp_server_port": fmt.Sprintf("%d", supervisorOpAmpServerPort)})

	require.Nil(t, s.Start(t.Context()))
	defer s.Shutdown()

	waitForSupervisorConnection(server.supervisorConnected, true)

	cfg, hash, inputFile, outputFile := createSimplePipelineCollectorConf(t)

	server.sendToSupervisor(&protobufs.ServerToAgent{
		RemoteConfig: &protobufs.AgentRemoteConfig{
			Config: &protobufs.AgentConfigMap{
				ConfigMap: map[string]*protobufs.AgentConfigFile{
					"": {Body: cfg.Bytes()},
				},
			},
			ConfigHash: hash,
		},
	})

	require.Eventually(t, func() bool {
		cfg, ok := agentConfig.Load().(string)
		if ok {
			// The effective config may be structurally different compared to what was sent,
			// and will also have some data redacted,
			// so just check that it includes the file_log receiver
			return strings.Contains(cfg, "file_log")
		}

		return false
	}, 5*time.Second, 500*time.Millisecond, "Collector was not started with remote config")

	n, err := inputFile.WriteString("{\"body\":\"hello, world\"}\n")
	require.NotZero(t, n, "Could not write to input file")
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		logRecord := make([]byte, 1024)
		n, _ := outputFile.Read(logRecord)

		return n != 0
	}, 10*time.Second, 500*time.Millisecond, "Log never appeared in output")
}

func TestSupervisorHealthCheckServer(t *testing.T) {
	server := newOpAMPServer(
		t,
		defaultConnectingHandler,
		types.ConnectionCallbacks{},
	)

	randomPort, err := findRandomPort()
	require.NoError(t, err)

	cfgFile := getSupervisorConfig(t, "healthcheck", map[string]string{
		"url":      server.addr,
		"endpoint": fmt.Sprintf("localhost:%d", randomPort),
	})

	cfg, err := config.Load(cfgFile.Name())
	require.NoError(t, err)
	logger, err := telemetry.NewLogger(cfg.Telemetry.Logs)
	require.NoError(t, err)

	s, err := supervisor.NewSupervisor(t.Context(), logger, cfg)
	require.NoError(t, err)
	require.Nil(t, s.Start(t.Context()))
	defer s.Shutdown()
	waitForSupervisorConnection(server.supervisorConnected, true)

	// Wait for the health check server to start
	require.Eventually(t, func() bool {
		resp, err := http.Get(fmt.Sprintf("http://localhost:%d/health", randomPort))
		if err != nil {
			t.Logf("Failed health check request: %s", err)
			return false
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			t.Logf("Got non-2xx status code: %d", resp.StatusCode)
			return false
		}
		return true
	}, 5*time.Second, 100*time.Millisecond, "Health check server did not start")
}

func TestSupervisorHealthCheckServerBackendConnError(t *testing.T) {
	healthcheckPort, err := findRandomPort()
	require.NoError(t, err)

	// Find an open port on the host that has no server listening on it.
	badOpAMPServerPort, err := findRandomPort()
	require.NoError(t, err)

	cfgFile := getSupervisorConfig(t, "healthcheck", map[string]string{
		"url":      fmt.Sprintf("localhost:%d", badOpAMPServerPort),
		"endpoint": fmt.Sprintf("localhost:%d", healthcheckPort),
	})

	cfg, err := config.Load(cfgFile.Name())
	require.NoError(t, err)
	logger, err := telemetry.NewLogger(cfg.Telemetry.Logs)
	require.NoError(t, err)

	s, err := supervisor.NewSupervisor(t.Context(), logger, cfg)
	require.NoError(t, err)
	require.Nil(t, s.Start(t.Context()))
	defer s.Shutdown()

	// Wait for the health check server to start
	require.Eventually(t, func() bool {
		resp, err := http.Get(fmt.Sprintf("http://localhost:%d/health", healthcheckPort))
		if err != nil {
			t.Logf("Failed health check request: %s", err)
			return false
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			t.Logf("Got non-2xx status code: %d", resp.StatusCode)
			return false
		}
		return true
	}, 5*time.Second, 100*time.Millisecond, "Health check server did not start")
}

func findRandomPort() (int, error) {
	l, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return 0, err
	}

	port := l.Addr().(*net.TCPAddr).Port

	err = l.Close()
	if err != nil {
		return 0, err
	}

	return port, nil
}

func TestSupervisorEmitBootstrapTelemetry(t *testing.T) {
	agentDescription := atomic.Value{}

	// Load the Supervisor config so we can get the location of
	// the Collector that will be run.
	var cfg config.Supervisor
	cfgFile := getSupervisorConfig(t, "nocap", map[string]string{})
	k := koanf.New("::")
	err := k.Load(file.Provider(cfgFile.Name()), yaml.Parser())
	require.NoError(t, err)
	err = k.UnmarshalWithConf("", &cfg, koanf.UnmarshalConf{
		Tag: "mapstructure",
	})
	require.NoError(t, err)

	// Get the binary name and version from the Collector binary
	// using the `components` command that prints a YAML-encoded
	// map of information about the Collector build. Some of this
	// information will be used as defaults for the telemetry
	// attributes.
	agentPath := cfg.Agent.Executable
	componentsInfo, err := exec.Command(agentPath, "components").Output()
	require.NoError(t, err)
	k = koanf.New("::")
	err = k.Load(rawbytes.Provider(componentsInfo), yaml.Parser())
	require.NoError(t, err)
	buildinfo := k.StringMap("buildinfo")
	command := buildinfo["command"]
	version := buildinfo["version"]

	server := newOpAMPServer(
		t,
		defaultConnectingHandler,
		types.ConnectionCallbacks{
			OnMessage: func(_ context.Context, _ types.Connection, message *protobufs.AgentToServer) *protobufs.ServerToAgent {
				if message.AgentDescription != nil {
					agentDescription.Store(message.AgentDescription)
				}

				return &protobufs.ServerToAgent{}
			},
		})

	outputPath := filepath.Join(t.TempDir(), "output.txt")
	backend := testbed.NewOTLPHTTPDataReceiver(4318)
	mockBackend := testbed.NewMockBackend(outputPath, backend)
	mockBackend.EnableRecording()
	defer mockBackend.Stop()
	require.NoError(t, mockBackend.Start())

	s, _ := newSupervisor(t,
		"emit_telemetry",
		map[string]string{
			"url":          server.addr,
			"telemetryUrl": fmt.Sprintf("localhost:%d", 4318),
		},
	)

	require.Nil(t, s.Start(t.Context()))
	defer s.Shutdown()

	waitForSupervisorConnection(server.supervisorConnected, true)

	require.Eventually(t, func() bool {
		ad, ok := agentDescription.Load().(*protobufs.AgentDescription)
		if !ok {
			return false
		}

		var agentName, agentVersion string
		identAttr := ad.IdentifyingAttributes
		for _, attr := range identAttr {
			switch attr.Key {
			case "service.name":
				agentName = attr.Value.GetStringValue()
			case "service.version":
				agentVersion = attr.Value.GetStringValue()
			}
		}

		// By default, the Collector should report its name and version
		// from the component.BuildInfo struct built into the Collector
		// binary.
		return agentName == command && agentVersion == version
	}, 5*time.Second, 250*time.Millisecond)

	expectedSpans := []string{"GetBootstrapInfo"}

	require.EventuallyWithT(t, func(collect *assert.CollectT) {
		require.GreaterOrEqual(collect, len(mockBackend.GetReceivedTraces()), len(expectedSpans))
	}, 10*time.Second, 250*time.Millisecond)

	receivedTraces := mockBackend.GetReceivedTraces()
	require.Equal(t, 1, receivedTraces[0].ResourceSpans().Len())
	gotServiceName, ok := receivedTraces[0].ResourceSpans().At(0).Resource().Attributes().Get("service.name")
	require.True(t, ok)
	require.Equal(t, "opamp-supervisor", gotServiceName.Str())

	for _, expectedSpan := range expectedSpans {
		gotSpan := false
		for i := 0; i < len(receivedTraces); i++ {
			require.Equal(t, 1, receivedTraces[i].ResourceSpans().At(0).ScopeSpans().Len())
			require.Equal(t, 1, receivedTraces[i].ResourceSpans().At(0).ScopeSpans().At(0).Spans().Len())
			if receivedTraces[i].ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Name() != expectedSpan {
				continue
			}
			gotSpan = true
			require.Equal(t, ptrace.StatusCodeOk, receivedTraces[i].ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Status().Code())
		}
		require.Truef(t, gotSpan, "expected to find span '%s', but did not find it", expectedSpan)
	}
}

func TestSupervisorReportsHeartbeat(t *testing.T) {
	var heartbeatReport atomic.Bool
	server := newOpAMPServer(
		t,
		defaultConnectingHandler,
		types.ConnectionCallbacks{
			OnMessage: func(_ context.Context, _ types.Connection, message *protobufs.AgentToServer) *protobufs.ServerToAgent {
				if isHeartbeatMessage(message) {
					heartbeatReport.Store(true)
				}
				return &protobufs.ServerToAgent{}
			},
		},
	)
	s, _ := newSupervisor(t, "reports_heartbeat", map[string]string{"url": server.addr})

	require.Nil(t, s.Start(t.Context()))
	defer s.Shutdown()

	waitForSupervisorConnection(server.supervisorConnected, true)

	// Set the heartbeat interval to 1 seconds
	server.sendToSupervisor(&protobufs.ServerToAgent{
		ConnectionSettings: &protobufs.ConnectionSettingsOffers{
			Opamp: &protobufs.OpAMPConnectionSettings{
				DestinationEndpoint:      "ws://" + server.addr + "/v1/opamp",
				HeartbeatIntervalSeconds: 1,
			},
		},
	})

	// supervisor disconnects from the server
	waitForSupervisorConnection(server.supervisorConnected, false)

	// supervisor reconnects to the server
	waitForSupervisorConnection(server.supervisorConnected, true)

	require.Eventually(t, func() bool {
		return heartbeatReport.Load()
	}, 3*time.Second, 250*time.Millisecond)
}

// isHeartbeatMessage returns true if all fields of the message are nil.
func isHeartbeatMessage(message *protobufs.AgentToServer) bool {
	empty := true

	empty = empty && message.AgentDescription == nil
	empty = empty && message.Health == nil
	empty = empty && message.EffectiveConfig == nil
	empty = empty && message.RemoteConfigStatus == nil
	empty = empty && message.PackageStatuses == nil
	empty = empty && message.AgentDisconnect == nil
	empty = empty && message.ConnectionSettingsRequest == nil
	empty = empty && message.CustomCapabilities == nil
	empty = empty && message.CustomMessage == nil
	empty = empty && message.AvailableComponents == nil
	empty = empty && message.Flags == 0

	return empty
}

// createFallbackCollectorConf creates a fallback collector config file and returns
// the path to the file, along with input/output files for testing the pipeline.
func createFallbackCollectorConf(t *testing.T, healthCheckPort string) (string, *os.File, *os.File) {
	wd, err := os.Getwd()
	require.NoError(t, err)

	tempDir := t.TempDir()
	inputFile, err := os.CreateTemp(tempDir, "fallback_input_*.yaml")
	require.NoError(t, err)
	t.Cleanup(func() { inputFile.Close() })

	outputFile, err := os.CreateTemp(tempDir, "fallback_output_*.yaml")
	require.NoError(t, err)
	t.Cleanup(func() { outputFile.Close() })

	colCfgTpl, err := os.ReadFile(path.Join(wd, "testdata", "collector", "fallback_config.yaml"))
	require.NoError(t, err)

	templ, err := template.New("").Parse(string(colCfgTpl))
	require.NoError(t, err)

	var confmapBuf bytes.Buffer
	err = templ.Execute(
		&confmapBuf,
		map[string]string{
			"inputLogFile":    inputFile.Name(),
			"outputLogFile":   outputFile.Name(),
			"healthCheckPort": healthCheckPort,
		},
	)
	require.NoError(t, err)

	// Write the fallback config to a file
	fallbackConfigFile, err := os.CreateTemp(tempDir, "fallback_config_*.yaml")
	require.NoError(t, err)
	t.Cleanup(func() { fallbackConfigFile.Close() })

	_, err = fallbackConfigFile.Write(confmapBuf.Bytes())
	require.NoError(t, err)

	return fallbackConfigFile.Name(), inputFile, outputFile
}

func TestSupervisorFallbackWhenNoPersistedConfig(t *testing.T) {
	storageDir := t.TempDir()

	fallbackPort, err := findRandomPort()
	require.NoError(t, err)

	localPort, err := findRandomPort()
	require.NoError(t, err)

	fallbackConfigPath, fallbackInputFile, fallbackOutputFile := createFallbackCollectorConf(
		t,
		strconv.Itoa(fallbackPort),
	)
	localConfigPath := createHealthCheckCollectorConfFile(t, strconv.Itoa(localPort))

	// Create an unstarted server - simulating server being unavailable at startup
	connected := atomic.Bool{}
	server := newUnstartedOpAMPServer(t, defaultConnectingHandler,
		types.ConnectionCallbacks{
			OnConnected: func(ctx context.Context, conn types.Connection) {
				connected.Store(true)
			},
		})
	defer server.shutdown()

	// Start supervisor with fallback config and local config for normal operation
	s, _ := newSupervisor(t, "fallback", map[string]string{
		"url":                     server.addr,
		"storage_dir":             storageDir,
		"local_config":            localConfigPath,
		"startup_fallback_config": escapePathStringForWin(fallbackConfigPath),
	})

	require.NoError(t, s.Start(t.Context()))
	defer s.Shutdown()

	// Collector should start with fallback config while the server is unavailable.
	require.Eventually(t, func() bool {
		return healthCheckOK(fallbackPort)
	}, 10*time.Second, 500*time.Millisecond, "Collector did not start with fallback config")

	require.Never(t, func() bool {
		return healthCheckOK(localPort)
	}, 2*time.Second, 200*time.Millisecond, "Collector should not use local config before initial OpAMP connection")

	// Verify the collector is processing data with fallback config
	sampleLog := `{"body":"fallback startup test"}`
	n, err := fallbackInputFile.WriteString(sampleLog + "\n")
	require.NotZero(t, n, "Could not write to input file")
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		logRecord := make([]byte, 1024)
		n, readErr := fallbackOutputFile.Read(logRecord)
		if readErr != nil && !errors.Is(readErr, io.EOF) {
			return false
		}
		return n != 0
	}, 10*time.Second, 500*time.Millisecond, "Log never appeared in fallback output")

	require.False(t, connected.Load(), "Supervisor should not connect without a running server")
}

func TestSupervisorFallbackDisablesAfterFirstConnect(t *testing.T) {
	storageDir := t.TempDir()

	fallbackPort, err := findRandomPort()
	require.NoError(t, err)
	localPort, err := findRandomPort()
	require.NoError(t, err)
	for localPort == fallbackPort {
		localPort, err = findRandomPort()
		require.NoError(t, err)
	}

	// Create fallback config for when server is unavailable
	fallbackConfigPath, _, _ := createFallbackCollectorConf(t, strconv.Itoa(fallbackPort))
	localConfigPath := createHealthCheckCollectorConfFile(t, strconv.Itoa(localPort))

	connected := atomic.Bool{}

	// Start with an unstarted server to trigger startup fallback
	server := newUnstartedOpAMPServer(t, defaultConnectingHandler,
		types.ConnectionCallbacks{
			OnConnected: func(ctx context.Context, conn types.Connection) {
				connected.Store(true)
			},
		})
	defer server.shutdown()

	// Start supervisor with fallback config and local config
	s, _ := newSupervisor(t, "fallback", map[string]string{
		"url":                     server.addr,
		"storage_dir":             storageDir,
		"local_config":            localConfigPath,
		"startup_fallback_config": escapePathStringForWin(fallbackConfigPath),
	})

	require.NoError(t, s.Start(t.Context()))
	defer s.Shutdown()

	// Wait for fallback to be triggered and collector to start with fallback config
	require.Eventually(t, func() bool {
		return healthCheckOK(fallbackPort)
	}, 10*time.Second, 500*time.Millisecond, "Collector did not start with fallback config")

	// Now start the OpAMP server
	server.start()

	// Wait for supervisor to connect
	waitForSupervisorConnection(server.supervisorConnected, true)
	require.True(t, connected.Load(), "Supervisor failed to connect after server started")

	// After the first connection, the supervisor should switch to the normal config.
	require.Eventually(t, func() bool {
		return healthCheckOK(localPort)
	}, 10*time.Second, 500*time.Millisecond, "Collector did not switch to normal config after first connection")
	require.Eventually(t, func() bool {
		return !healthCheckOK(fallbackPort)
	}, 10*time.Second, 500*time.Millisecond, "Fallback config still active after first connection")

	// Simulate the backend going down. Fallback should not be reapplied.
	server.shutdown()
	waitForSupervisorConnection(server.supervisorConnected, false)

	require.Eventually(t, func() bool {
		return healthCheckOK(localPort)
	}, 5*time.Second, 500*time.Millisecond, "Collector stopped running after backend loss")
	require.Never(t, func() bool {
		return healthCheckOK(fallbackPort)
	}, 3*time.Second, 200*time.Millisecond, "Fallback config should not be re-applied after initial connection")
}

func TestSupervisorValidatesConfigBeforeApplying(t *testing.T) {
	modes := getTestModes()

	for _, mode := range modes {
		t.Run(mode.name, func(t *testing.T) {
			var remoteConfigStatus atomic.Value
			var agentConfig atomic.Value
			server := newOpAMPServer(
				t,
				defaultConnectingHandler,
				types.ConnectionCallbacks{
					OnMessage: func(_ context.Context, _ types.Connection, message *protobufs.AgentToServer) *protobufs.ServerToAgent {
						if message.RemoteConfigStatus != nil {
							remoteConfigStatus.Store(message.RemoteConfigStatus)
						}
						if message.EffectiveConfig != nil {
							config := message.EffectiveConfig.ConfigMap.ConfigMap[""]
							if config != nil {
								agentConfig.Store(string(config.Body))
							}
						}

						return &protobufs.ServerToAgent{}
					},
				})

			extraConfigData := map[string]string{
				"url":             server.addr,
				"validate_config": "true",
			}
			if mode.UseHUPConfigReload {
				extraConfigData["use_hup_config_reload"] = "true"
			}

			s, supervisorCfg := newSupervisor(t, "basic", extraConfigData)
			if mode.UseHUPConfigReload {
				require.True(t, supervisorCfg.Agent.UseHUPConfigReload)
			}
			require.True(t, supervisorCfg.Agent.ValidateConfig, "ValidateConfig should be enabled for this test")

			require.Nil(t, s.Start(t.Context()))
			defer s.Shutdown()

			waitForSupervisorConnection(server.supervisorConnected, true)

			// First, send a valid config
			goodCfg, goodHash, _, _ := createSimplePipelineCollectorConf(t)

			server.sendToSupervisor(&protobufs.ServerToAgent{
				RemoteConfig: &protobufs.AgentRemoteConfig{
					Config: &protobufs.AgentConfigMap{
						ConfigMap: map[string]*protobufs.AgentConfigFile{
							"": {Body: goodCfg.Bytes()},
						},
					},
					ConfigHash: goodHash,
				},
			})

			require.Eventually(t, func() bool {
				cfg, ok := agentConfig.Load().(string)
				return ok && cfg != ""
			}, 5*time.Second, 500*time.Millisecond, "Collector was not started with valid config")

			// Now send an invalid config that should fail validation
			invalidCfg, err := os.ReadFile(path.Join("testdata", "collector", "invalid_component_config.yaml"))
			require.NoError(t, err)

			h := sha256.Sum256(invalidCfg)
			invalidHash := h[:]

			server.sendToSupervisor(&protobufs.ServerToAgent{
				RemoteConfig: &protobufs.AgentRemoteConfig{
					Config: &protobufs.AgentConfigMap{
						ConfigMap: map[string]*protobufs.AgentConfigFile{
							"": {Body: invalidCfg},
						},
					},
					ConfigHash: invalidHash,
				},
			})

			// The config should be rejected with a FAILED status
			require.Eventually(t, func() bool {
				status := remoteConfigStatus.Load()
				if status != nil {
					remoteStatus := status.(*protobufs.RemoteConfigStatus)
					return remoteStatus.Status == protobufs.RemoteConfigStatuses_RemoteConfigStatuses_FAILED
				}
				return false
			}, 5*time.Second, 250*time.Millisecond, "Invalid config was not rejected")

			// Verify that the collector is still running with the old (valid) config
			currentCfg := agentConfig.Load().(string)
			require.NotContains(t, currentCfg, "nonexistent_exporter", "Config should not have been updated to invalid config")
		})
	}
}

func TestSupervisorExtensionsFeatureGateRequired(t *testing.T) {
	t.Run("feature-gate disabled", func(t *testing.T) {
		// Render configuration with nop extension
		cfgFile := getSupervisorConfig(t, "extensions", map[string]string{
			"url": "localhost:0",
		})
		cfg, err := config.Load(cfgFile.Name())
		require.NoError(t, err)

		// Attempt to create a supervisor with an extensions config. The feature
		// gate is disabled by default, so supervisor.NewSupervisor must fail with an error
		// that references the feature gate and how to enable it.
		s, err := supervisor.NewSupervisor(t.Context(), zap.NewNop(), cfg)
		require.Nil(t, s)
		require.Contains(t, err.Error(), metadata.OpampsupervisorExtensionsFeatureGate.ID())
		require.Contains(t, err.Error(), "--feature-gates=")
	})

	t.Run("feature-gate enabled", func(t *testing.T) {
		// Create basic opamp server to check that the supervisor connects
		connected := atomic.Bool{}
		server := newOpAMPServer(t, defaultConnectingHandler, types.ConnectionCallbacks{
			OnConnected: func(context.Context, types.Connection) {
				connected.Store(true)
			},
		})

		// Enable extensions feature-gate
		enableExtensionsFeatureGate(t)

		// Create supervisor with configuration that has nop extension
		s, _ := newSupervisor(t,
			"extensions",
			map[string]string{
				"url": server.addr,
			},
		)

		// Start supervisor and wait for successful connection
		defer s.Shutdown()
		require.NoError(t, s.Start(t.Context()))

		waitForSupervisorConnection(server.supervisorConnected, true)
		require.True(t, connected.Load(), "Supervisor failed to connect")
	})
}

func TestSupervisorAuthExtensions(t *testing.T) {
	// captureAuthHandler returns an OnConnecting handler that records the
	// Authorization header of each incoming connection attempt on authHeaders.
	captureAuthHandler := func(authHeaders chan string) onConnectingFuncFactory {
		return func(connectionCallbacks types.ConnectionCallbacks) func(*http.Request) types.ConnectionResponse {
			return func(req *http.Request) types.ConnectionResponse {
				// Non-blocking send so we don't deadlock if a test only consumes once.
				select {
				case authHeaders <- req.Header.Get("Authorization"):
				default:
				}
				return types.ConnectionResponse{
					Accept:              true,
					ConnectionCallbacks: connectionCallbacks,
				}
			}
		}
	}

	t.Run("bearer token auth extension", func(t *testing.T) {
		enableExtensionsFeatureGate(t)

		tokenFile := filepath.Join(t.TempDir(), "token")
		require.NoError(t, os.WriteFile(tokenFile, []byte("test-token-1"), 0o600))

		authHeaders := make(chan string, 4)
		server := newOpAMPServer(t, captureAuthHandler(authHeaders), types.ConnectionCallbacks{})

		s, _ := newSupervisor(t, "auth_bearer", map[string]string{
			"url":        server.addr,
			"token_file": tokenFile,
		})
		defer s.Shutdown()
		require.NoError(t, s.Start(t.Context()))

		waitForSupervisorConnection(server.supervisorConnected, true)

		select {
		case got := <-authHeaders:
			require.Equal(t, "Bearer test-token-1", got)
		case <-time.After(5 * time.Second):
			t.Fatal("did not observe Authorization header on initial connect")
		}
	})

	t.Run("basic auth auth extension", func(t *testing.T) {
		enableExtensionsFeatureGate(t)

		authHeaders := make(chan string, 4)
		server := newOpAMPServer(t, captureAuthHandler(authHeaders), types.ConnectionCallbacks{})

		s, _ := newSupervisor(t, "auth_basic", map[string]string{
			"url":      server.addr,
			"username": "alice",
			"password": "secret",
		})
		defer s.Shutdown()
		require.NoError(t, s.Start(t.Context()))

		waitForSupervisorConnection(server.supervisorConnected, true)

		// base64("alice:secret") = "YWxpY2U6c2VjcmV0"
		select {
		case got := <-authHeaders:
			require.Equal(t, "Basic YWxpY2U6c2VjcmV0", got)
		case <-time.After(5 * time.Second):
			t.Fatal("did not observe Authorization header on initial connect")
		}
	})

	t.Run("oauth2 client credentials auth extension", func(t *testing.T) {
		enableExtensionsFeatureGate(t)

		// Pattern follows TestClientCredentials in
		// extension/oauth2clientauthextension/extension_test.go: verify the
		// supervisor's oauth2client extension POSTs client_credentials and
		// receives back an opaque bearer token, then carries that token on the
		// OpAMP connection handshake.
		tokenServer := newMockTokenServer(t, "opamp-access-token")

		authHeaders := make(chan string, 4)
		server := newOpAMPServer(t, captureAuthHandler(authHeaders), types.ConnectionCallbacks{})

		s, _ := newSupervisor(t, "auth_oauth2", map[string]string{
			"url":           server.addr,
			"client_id":     "opamp-client-id",
			"client_secret": "opamp-client-secret",
			"token_url":     tokenServer.URL,
		})
		defer s.Shutdown()
		require.NoError(t, s.Start(t.Context()))

		waitForSupervisorConnection(server.supervisorConnected, true)

		select {
		case got := <-authHeaders:
			require.Equal(t, "Bearer opamp-access-token", got)
		case <-time.After(5 * time.Second):
			t.Fatal("did not observe Authorization header on initial connect")
		}

		require.GreaterOrEqual(t, tokenServer.requestCount(), int64(1),
			"oauth2 extension should have contacted the token server at least once")
	})

	t.Run("auth preserved on OnConnectionSettings msg", func(t *testing.T) {
		enableExtensionsFeatureGate(t)

		tokenFile := filepath.Join(t.TempDir(), "token")
		require.NoError(t, os.WriteFile(tokenFile, []byte("rotate-token"), 0o600))

		initialHeaders := make(chan string, 4)
		initialServer := newOpAMPServer(t, captureAuthHandler(initialHeaders), types.ConnectionCallbacks{})

		s, _ := newSupervisor(t, "auth_bearer_accepts_conn", map[string]string{
			"url":        initialServer.addr,
			"token_file": tokenFile,
		})
		defer s.Shutdown()
		require.NoError(t, s.Start(t.Context()))

		waitForSupervisorConnection(initialServer.supervisorConnected, true)
		select {
		case got := <-initialHeaders:
			require.Equal(t, "Bearer rotate-token", got)
		case <-time.After(5 * time.Second):
			t.Fatal("did not observe Authorization header on initial connect")
		}

		newHeaders := make(chan string, 4)
		newServer := newOpAMPServer(t, captureAuthHandler(newHeaders), types.ConnectionCallbacks{})

		initialServer.sendToSupervisor(&protobufs.ServerToAgent{
			ConnectionSettings: &protobufs.ConnectionSettingsOffers{
				Opamp: &protobufs.OpAMPConnectionSettings{
					DestinationEndpoint: "ws://" + newServer.addr + "/v1/opamp",
				},
			},
		})

		waitForSupervisorConnection(newServer.supervisorConnected, true)
		select {
		case got := <-newHeaders:
			require.Equal(t, "Bearer rotate-token", got, "auth header must persist across server-pushed connection settings")
		case <-time.After(10 * time.Second):
			t.Fatal("did not observe Authorization header on reconnect")
		}
	})

	t.Run("oauth2 token rotation across reconnect", func(t *testing.T) {
		enableExtensionsFeatureGate(t)

		// Verifies the supervisor's HeaderFunc consults the auth extension fresh
		// on every reconnect, so a rotated upstream token is picked up
		// automatically. The mock token server always returns expires_in: 1, so
		// golang.org/x/oauth2's 10s expiry buffer treats every cached token as
		// expired and the extension re-fetches on each connection attempt.
		tokenServer := newMockTokenServer(t, "token-A")

		authHeaders := make(chan string, 16)
		server := newOpAMPServer(t, captureAuthHandler(authHeaders), types.ConnectionCallbacks{})

		s, _ := newSupervisor(t, "auth_oauth2", map[string]string{
			"url":           server.addr,
			"client_id":     "opamp-client-id",
			"client_secret": "opamp-client-secret",
			"token_url":     tokenServer.URL,
		})
		defer s.Shutdown()
		require.NoError(t, s.Start(t.Context()))

		waitForSupervisorConnection(server.supervisorConnected, true)
		select {
		case got := <-authHeaders:
			require.Equal(t, "Bearer token-A", got)
		case <-time.After(5 * time.Second):
			t.Fatal("did not observe Authorization header on initial connect")
		}

		// Rotate the upstream token and force the supervisor's WS to reconnect.
		// Server-side disconnect is a transport-level event; the opamp-go client
		// reconnects on its own, which re-runs the HeaderFunc and pulls a fresh
		// token from the oauth2 extension.
		tokenServer.setToken("token-B")
		require.NoError(t, server.disconnectAgent())

		waitForSupervisorConnection(server.supervisorConnected, true)
		select {
		case got := <-authHeaders:
			require.Equal(t, "Bearer token-B", got)
		case <-time.After(5 * time.Second):
			t.Fatal("did not observe Authorization header on reconnect")
		}

		require.GreaterOrEqual(t, tokenServer.requestCount(), int64(2),
			"oauth2 extension should have re-fetched the token at least once after reconnect")
	})
}

// mockTokenServer is an oauth2 client_credentials token endpoint used by the
// auth extension tests. It validates the grant type, scope, and HTTP Basic
// client credentials, and returns the configurable currentToken with
// expires_in: 1 so the oauth2 client's expiry buffer treats every cached token
// as expired (forcing a fresh fetch on every request).
type mockTokenServer struct {
	*httptest.Server
	mu           sync.Mutex
	currentToken string
	requests     atomic.Int64
}

func newMockTokenServer(t *testing.T, initialToken string) *mockTokenServer {
	t.Helper()
	mts := &mockTokenServer{currentToken: initialToken}
	mts.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mts.requests.Add(1)

		require.NoError(t, r.ParseForm())
		require.Equal(t, "client_credentials", r.PostFormValue("grant_type"))
		require.Equal(t, "opamp", r.PostFormValue("scope"))

		// oauth2 client_credentials flow first tries HTTP Basic on the token
		// endpoint, so assert that to catch any config regression.
		username, password, ok := r.BasicAuth()
		require.True(t, ok, "token request missing basic auth")
		require.Equal(t, "opamp-client-id", username)
		require.Equal(t, "opamp-client-secret", password)

		mts.mu.Lock()
		token := mts.currentToken
		mts.mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": token,
			"token_type":   "Bearer",
			"expires_in":   1,
		})
	}))
	t.Cleanup(mts.Server.Close)
	return mts
}

func (m *mockTokenServer) setToken(s string) {
	m.mu.Lock()
	m.currentToken = s
	m.mu.Unlock()
}

func (m *mockTokenServer) requestCount() int64 {
	return m.requests.Load()
}

// enableExtensionsFeatureGate flips the extensions feature gate on for the
// duration of the test and restores it in t.Cleanup.
func enableExtensionsFeatureGate(t *testing.T) {
	t.Helper()
	require.NoError(t, featuregate.GlobalRegistry().Set(metadata.OpampsupervisorExtensionsFeatureGate.ID(), true))
	t.Cleanup(func() {
		require.NoError(t, featuregate.GlobalRegistry().Set(metadata.OpampsupervisorExtensionsFeatureGate.ID(), false))
	})
}

// supervisorBinarySizeLimitBytes is the size budget for the supervisor binary,
// in bytes. 24 MiB.
const supervisorBinarySizeLimitBytes = 24 * 1024 * 1024

// TestSupervisorBinarySize guards against unintended growth of the supervisor
// binary. It builds the supervisor with the same flags used for release
// artifacts into a temp dir, then asserts the resulting file is no larger than
// supervisorBinarySizeLimitBytes.
//
// We intentionally do NOT measure the binary produced by `make opampsupervisor`:
// that target is unstripped on purpose so contributors can attach a debugger to
// local builds. The release pipeline applies `-s -w` separately, and that is the
// artifact this budget governs — so the number reported here will not match
// `ls -la bin/opampsupervisor_*` after a default `make` build.
//
// Reference for the build process of the supervisor during releases:
// https://github.com/open-telemetry/opentelemetry-collector-releases/blob/main/cmd/opampsupervisor/.goreleaser.yaml
func TestSupervisorBinarySize(t *testing.T) {
	if runtime.GOOS != "linux" || runtime.GOARCH != "amd64" {
		t.Skipf("binary size budget is calibrated for linux/amd64; skipping on %s/%s", runtime.GOOS, runtime.GOARCH)
	}
	binPath := filepath.Join(t.TempDir(), "opampsupervisor")

	cmd := exec.CommandContext(t.Context(), "go", "build",
		"-trimpath",
		"-ldflags=-s -w -X github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/internal.version=v1.0.0",
		"-tags=",
		"-o", binPath,
		".",
	)
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "go build failed: %s", out)

	info, err := os.Stat(binPath)
	require.NoError(t, err)

	require.LessOrEqualf(t, info.Size(), int64(supervisorBinarySizeLimitBytes),
		"supervisor binary size %d bytes (%.2f MiB) exceeds limit %d bytes (%d MiB)",
		info.Size(), float64(info.Size())/(1024*1024),
		supervisorBinarySizeLimitBytes, supervisorBinarySizeLimitBytes/(1024*1024))
}
