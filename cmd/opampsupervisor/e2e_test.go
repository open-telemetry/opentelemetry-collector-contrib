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
	"strings"
	"sync/atomic"
	"testing"
	"text/template"
	"time"

	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"

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
	semconv "go.opentelemetry.io/collector/semconv/v1.21.0"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/protobuf/proto"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor"
	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor/config"
	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor/telemetry"
)

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
	return func(_ *http.Request) types.ConnectionResponse {
		return types.ConnectionResponse{
			Accept:              true,
			ConnectionCallbacks: connectionCallbacks,
		}
	}
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
		if onConnectedFunc != nil {
			onConnectedFunc(ctx, conn)
		}
		agentConn.Store(conn)
		isAgentConnected.Store(true)
		connectedChan <- true
	}
	onConnectionCloseFunc := callbacks.OnConnectionClose
	callbacks.OnConnectionClose = func(conn types.Connection) {
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
			err := s.Stop(context.Background())
			assert.NoError(t, err)
			httpSrv.Close()
		}
		didShutdown.Store(true)
	}
	send := func(msg *protobufs.ServerToAgent) {
		if !isAgentConnected.Load() {
			require.Fail(t, "Agent connection has not been established")
		}

		err = agentConn.Load().(types.Connection).Send(context.Background(), msg)
		require.NoError(t, err)
	}
	t.Cleanup(func() {
		shutdown()
	})
	return &testingOpAMPServer{
		addr:                httpSrv.Listener.Addr().String(),
		supervisorConnected: connectedChan,
		sendToSupervisor:    send,
		start:               httpSrv.Start,
		shutdown:            shutdown,
	}
}

func newSupervisor(t *testing.T, configType string, extraConfigData map[string]string) *supervisor.Supervisor {
	cfgFile := getSupervisorConfig(t, configType, extraConfigData)

	cfg, err := config.Load(cfgFile.Name())
	require.NoError(t, err)

	logger, err := zap.NewDevelopment()
	require.NoError(t, err)

	s, err := supervisor.NewSupervisor(logger, cfg)
	require.NoError(t, err)

	return s
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
		"storage_dir": strings.ReplaceAll(t.TempDir(), "\\", "\\\\"),
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

func TestSupervisorStartsCollectorWithRemoteConfig(t *testing.T) {
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

	s := newSupervisor(t, "basic", map[string]string{"url": server.addr})

	require.Nil(t, s.Start())
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
			// so just check that it includes the filelog receiver
			return strings.Contains(cfg, "filelog")
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

func TestSupervisorStartsCollectorWithNoOpAMPServerWithNoLastRemoteConfig(t *testing.T) {
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

	healthcheckPort, err := findRandomPort()
	require.NoError(t, err)

	s := newSupervisor(t, "healthcheck_port", map[string]string{
		"url":              server.addr,
		"storage_dir":      storageDir,
		"healthcheck_port": fmt.Sprintf("%d", healthcheckPort),
		"local_config":     filepath.Join("testdata", "collector", "nop_config.yaml"),
	})
	t.Cleanup(s.Shutdown)
	require.Nil(t, s.Start())

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
}

func TestSupervisorStartsCollectorWithNoOpAMPServerUsingLastRemoteConfig(t *testing.T) {
	storageDir := t.TempDir()
	remoteConfigFilePath := filepath.Join(storageDir, "last_recv_remote_config.dat")

	cfg, hash, healthcheckPort := createHealthCheckCollectorConf(t)
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

	connected := atomic.Bool{}
	server := newUnstartedOpAMPServer(t, defaultConnectingHandler, types.ConnectionCallbacks{
		OnConnected: func(ctx context.Context, conn types.Connection) {
			connected.Store(true)
		},
	})
	defer server.shutdown()

	s := newSupervisor(t, "basic", map[string]string{
		"url":         server.addr,
		"storage_dir": storageDir,
	})

	require.Nil(t, s.Start())
	defer s.Shutdown()

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
}

func TestSupervisorStartsCollectorWithRemoteConfigAndExecParams(t *testing.T) {
	storageDir := t.TempDir()

	// create remote config to check agent's health
	remoteConfigFilePath := filepath.Join(storageDir, "last_recv_remote_config.dat")
	cfg, hash, healthcheckPort := createHealthCheckCollectorConf(t)
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

	// fill env variables passed via parameters which are used in the collector config passed via config_files param
	s := newSupervisor(t, "exec_config", map[string]string{
		"url":           server.addr,
		"storage_dir":   storageDir,
		"inputLogFile":  inputFile.Name(),
		"outputLogFile": outputFile.Name(),
	})

	require.Nil(t, s.Start())
	defer s.Shutdown()

	waitForSupervisorConnection(server.supervisorConnected, true)

	// check health
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

	// check that collector uses filelog receiver and file exporter from config passed via config_files param
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
	s := newSupervisor(t, "healthcheck_port", map[string]string{
		"url":              server.addr,
		"healthcheck_port": "12345",
	})

	require.Nil(t, s.Start())
	defer s.Shutdown()

	// Verify the collector is not running after 250 ms by checking the healthcheck endpoint
	time.Sleep(250 * time.Millisecond)
	_, err := http.DefaultClient.Get("http://localhost:12345")

	if runtime.GOOS != "windows" {
		require.ErrorContains(t, err, "connection refused")
	} else {
		require.ErrorContains(t, err, "No connection could be made")
	}

	// Start the server and wait for the supervisor to connect
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

	s := newSupervisor(t, "basic", map[string]string{"url": server.addr})

	require.Nil(t, s.Start())
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

	require.Eventually(t, func() bool {
		health := healthReport.Load().(*protobufs.ComponentHealth)

		if health != nil {
			return !health.Healthy && health.LastError != ""
		}

		return false
	}, 5*time.Second, 250*time.Millisecond, "Supervisor never reported that the Collector was unhealthy")

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

	s := newSupervisor(t, "nocap", map[string]string{"url": server.addr})

	require.Nil(t, s.Start())
	defer s.Shutdown()

	waitForSupervisorConnection(server.supervisorConnected, true)

	require.Eventually(t, func() bool {
		caps := capabilities.Load()

		return caps == uint64(protobufs.AgentCapabilities_AgentCapabilities_ReportsStatus)
	}, 5*time.Second, 250*time.Millisecond)
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

			s := newSupervisor(t, "nocap", map[string]string{"url": server.addr})

			require.Nil(t, s.Start())
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
					case semconv.AttributeServiceName:
						agentName = attr.Value.GetStringValue()
					case semconv.AttributeServiceVersion:
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

	s := newSupervisor(t, "reports_available_components", map[string]string{"url": server.addr})

	require.Nil(t, s.Start())
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
			case semconv.AttributeServiceName:
				agentName = attr.Value.GetStringValue()
			case semconv.AttributeServiceVersion:
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

	s := newSupervisor(t, "basic", map[string]string{"url": server.addr})

	require.Nil(t, s.Start())
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
			// The effective config may be structurally different compared to what was sent,
			// and currently has most values redacted,
			// so just check that it includes some strings we know to be unique to the remote config.
			return strings.Contains(cfg, "test_key:")
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

	s := newSupervisor(t, "agent_description", map[string]string{"url": server.addr})

	require.Nil(t, s.Start())
	defer s.Shutdown()

	waitForSupervisorConnection(server.supervisorConnected, true)
	var ad *protobufs.AgentToServer
	select {
	case ad = <-agentDescMessageChan:
	case <-time.After(5 * time.Second):
		t.Fatal("Failed to get agent description after 5 seconds")
	}

	expectedDescription := &protobufs.AgentDescription{
		IdentifyingAttributes: []*protobufs.KeyValue{
			stringKeyValue("client.id", "my-client-id"),
			stringKeyValue(semconv.AttributeServiceInstanceID, uuid.UUID(ad.InstanceUid).String()),
			stringKeyValue(semconv.AttributeServiceName, command),
			stringKeyValue(semconv.AttributeServiceVersion, version),
		},
		NonIdentifyingAttributes: []*protobufs.KeyValue{
			stringKeyValue("env", "prod"),
			stringKeyValue(semconv.AttributeHostArch, runtime.GOARCH),
			stringKeyValue(semconv.AttributeHostName, host),
			stringKeyValue(semconv.AttributeOSType, runtime.GOOS),
		},
	}

	require.Subset(t, ad.AgentDescription.IdentifyingAttributes, expectedDescription.IdentifyingAttributes)
	require.Subset(t, ad.AgentDescription.NonIdentifyingAttributes, expectedDescription.NonIdentifyingAttributes)

	time.Sleep(250 * time.Millisecond)
}

func stringKeyValue(key, val string) *protobufs.KeyValue {
	return &protobufs.KeyValue{
		Key: key,
		Value: &protobufs.AnyValue{
			Value: &protobufs.AnyValue_StringValue{
				StringValue: val,
			},
		},
	}
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

func createBadCollectorConf(t *testing.T) (*bytes.Buffer, []byte) {
	colCfg, err := os.ReadFile(path.Join("testdata", "collector", "bad_config.yaml"))
	require.NoError(t, err)

	h := sha256.New()
	if _, err := io.Copy(h, bytes.NewBuffer(colCfg)); err != nil {
		log.Fatal(err)
	}

	return bytes.NewBuffer(colCfg), h.Sum(nil)
}

func createHealthCheckCollectorConf(t *testing.T) (cfg *bytes.Buffer, hash []byte, remotePort int) {
	colCfgTpl, err := os.ReadFile(path.Join("testdata", "collector", "healthcheck_config.yaml"))
	require.NoError(t, err)

	templ, err := template.New("").Parse(string(colCfgTpl))
	require.NoError(t, err)

	port, err := findRandomPort()

	var confmapBuf bytes.Buffer
	err = templ.Execute(
		&confmapBuf,
		map[string]string{
			"HealthCheckEndpoint": fmt.Sprintf("localhost:%d", port),
		},
	)
	require.NoError(t, err)

	h := sha256.Sum256(confmapBuf.Bytes())

	return &confmapBuf, h[:], port
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

	s := newSupervisor(t, "basic", map[string]string{"url": server.addr})

	require.Nil(t, s.Start())
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
}

func TestSupervisorOpAMPConnectionSettings(t *testing.T) {
	var connectedToNewServer atomic.Bool
	initialServer := newOpAMPServer(
		t,
		defaultConnectingHandler,
		types.ConnectionCallbacks{})

	s := newSupervisor(t, "accepts_conn", map[string]string{"url": initialServer.addr})

	require.Nil(t, s.Start())
	defer s.Shutdown()

	waitForSupervisorConnection(initialServer.supervisorConnected, true)

	newServer := newOpAMPServer(
		t,
		defaultConnectingHandler,
		types.ConnectionCallbacks{
			OnConnected: func(_ context.Context, _ types.Connection) {
				connectedToNewServer.Store(true)
			},
			OnMessage: func(_ context.Context, _ types.Connection, _ *protobufs.AgentToServer) *protobufs.ServerToAgent {
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

	s := newSupervisor(t, "http", map[string]string{"url": initialServer.addr})

	require.Nil(t, s.Start())
	defer s.Shutdown()

	waitForSupervisorConnection(initialServer.supervisorConnected, true)
	require.True(t, connected.Load(), "Supervisor failed to connect")
}

func TestSupervisorRestartsWithLastReceivedConfig(t *testing.T) {
	// Create a temporary directory to store the test config file.
	tempDir := t.TempDir()

	var agentConfig atomic.Value
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
				return &protobufs.ServerToAgent{}
			},
		})

	s := newSupervisor(t, "persistence", map[string]string{"url": initialServer.addr, "storage_dir": tempDir})

	require.Nil(t, s.Start())

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

	s1 := newSupervisor(t, "persistence", map[string]string{"url": newServer.addr, "storage_dir": tempDir})

	require.Nil(t, s1.Start())
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

		return strings.Contains(loadedConfig, "filelog")
	}, 10*time.Second, 500*time.Millisecond, "Collector was not started with the last received remote config")
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

	s := newSupervisor(t, "basic", map[string]string{
		"url":         server.addr,
		"storage_dir": storageDir,
	})

	require.Nil(t, s.Start())

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

	s = newSupervisor(t, "basic", map[string]string{
		"url":         server.addr,
		"storage_dir": storageDir,
	})

	require.Nil(t, s.Start())
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

	s := newSupervisor(t, "basic", map[string]string{
		"url":         server.addr,
		"storage_dir": storageDir,
	})

	require.Nil(t, s.Start())

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

	s = newSupervisor(t, "basic", map[string]string{
		"url":         server.addr,
		"storage_dir": storageDir,
	})

	require.Nil(t, s.Start())
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

	s := newSupervisor(t, "basic", map[string]string{
		"url":         server.addr,
		"storage_dir": storageDir,
	})

	require.Nil(t, s.Start())

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

				return &protobufs.ServerToAgent{}
			},
		})

	s := newSupervisor(t, "healthcheck_port", map[string]string{
		"url":              server.addr,
		"healthcheck_port": "12345",
	})

	require.Nil(t, s.Start())
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
		resp, err := http.DefaultClient.Get("http://localhost:12345")
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

	collectorCfg, hash, _, _ := createSimplePipelineCollectorConf(t)
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

	s, err := supervisor.NewSupervisor(logger, cfg)
	require.NoError(t, err)
	require.Nil(t, s.Start())

	// Start the server and wait for the supervisor to connect
	server.start()
	waitForSupervisorConnection(server.supervisorConnected, true)
	require.True(t, connected.Load(), "Supervisor failed to connect")

	s.Shutdown()

	// Read from log file checking for Info level logs
	logFile, err := os.Open(supervisorLogFilePath)
	require.NoError(t, err)

	scanner := bufio.NewScanner(logFile)
	seenCollectorLog := false
	for scanner.Scan() {
		line := scanner.Bytes()
		var log logEntry
		err := json.Unmarshal(line, &log)
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
	require.NoError(t, logFile.Close())
}

func TestSupervisorRemoteConfigApplyStatus(t *testing.T) {
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

	s := newSupervisor(t, "report_status", map[string]string{
		"url":                  server.addr,
		"config_apply_timeout": "3s",
	})
	require.Nil(t, s.Start())
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
	require.Eventually(t, func() bool {
		status, ok := remoteConfigStatus.Load().(*protobufs.RemoteConfigStatus)
		return ok && status.Status == protobufs.RemoteConfigStatuses_RemoteConfigStatuses_APPLYING
	}, 5*time.Second, 100*time.Millisecond, "Remote config status was not set to APPLYING")

	// Wait for collector to become healthy
	require.Eventually(t, func() bool {
		health, ok := healthReport.Load().(*protobufs.ComponentHealth)
		return ok && health.Healthy
	}, 10*time.Second, 10*time.Millisecond, "Collector did not become healthy")

	// Check that the status is set to APPLIED
	require.Eventually(t, func() bool {
		status, ok := remoteConfigStatus.Load().(*protobufs.RemoteConfigStatus)
		return ok && status.Status == protobufs.RemoteConfigStatuses_RemoteConfigStatuses_APPLIED
	}, 5*time.Second, 10*time.Millisecond, "Remote config status was not set to APPLIED")

	require.Eventually(t, func() bool {
		cfg, ok := agentConfig.Load().(string)
		if ok {
			// The effective config may be structurally different compared to what was sent,
			// and will also have some data redacted,
			// so just check that it includes the filelog receiver
			return strings.Contains(cfg, "filelog")
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

	// Check that the status is set to APPLYING
	require.Eventually(t, func() bool {
		status, ok := remoteConfigStatus.Load().(*protobufs.RemoteConfigStatus)
		return ok && status.Status == protobufs.RemoteConfigStatuses_RemoteConfigStatuses_APPLYING
	}, 5*time.Second, 200*time.Millisecond, "Remote config status was not set to APPLYING for bad config")

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

	s := newSupervisor(t, "server_port", map[string]string{"url": server.addr, "supervisor_opamp_server_port": fmt.Sprintf("%d", supervisorOpAmpServerPort)})

	require.Nil(t, s.Start())
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
			// so just check that it includes the filelog receiver
			return strings.Contains(cfg, "filelog")
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
	_, err = findRandomPort()
	require.Nil(t, err)
	backend := testbed.NewOTLPHTTPDataReceiver(4318)
	mockBackend := testbed.NewMockBackend(outputPath, backend)
	mockBackend.EnableRecording()
	defer mockBackend.Stop()
	require.NoError(t, mockBackend.Start())

	s := newSupervisor(t,
		"emit_telemetry",
		map[string]string{
			"url":          server.addr,
			"telemetryUrl": fmt.Sprintf("localhost:%d", 4318),
		},
	)

	require.Nil(t, s.Start())
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
			case semconv.AttributeServiceName:
				agentName = attr.Value.GetStringValue()
			case semconv.AttributeServiceVersion:
				agentVersion = attr.Value.GetStringValue()
			}
		}

		// By default, the Collector should report its name and version
		// from the component.BuildInfo struct built into the Collector
		// binary.
		return agentName == command && agentVersion == version
	}, 5*time.Second, 250*time.Millisecond)

	require.EventuallyWithT(t, func(collect *assert.CollectT) {
		require.Len(collect, mockBackend.ReceivedTraces, 1)
	}, 10*time.Second, 250*time.Millisecond)

	require.Equal(t, 1, mockBackend.ReceivedTraces[0].ResourceSpans().Len())
	gotServiceName, ok := mockBackend.ReceivedTraces[0].ResourceSpans().At(0).Resource().Attributes().Get(semconv.AttributeServiceName)
	require.True(t, ok)
	require.Equal(t, "opamp-supervisor", gotServiceName.Str())

	require.Equal(t, 1, mockBackend.ReceivedTraces[0].ResourceSpans().At(0).ScopeSpans().Len())
	require.Equal(t, 1, mockBackend.ReceivedTraces[0].ResourceSpans().At(0).ScopeSpans().At(0).Spans().Len())
	require.Equal(t, "GetBootstrapInfo", mockBackend.ReceivedTraces[0].ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Name())
	require.Equal(t, ptrace.StatusCodeOk, mockBackend.ReceivedTraces[0].ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Status().Code())
}
