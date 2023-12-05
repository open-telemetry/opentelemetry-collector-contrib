// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build e2e
// +build e2e

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"text/template"
	"time"

	"github.com/open-telemetry/opamp-go/protobufs"
	"github.com/open-telemetry/opamp-go/server"
	"github.com/open-telemetry/opamp-go/server/types"
	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type testLogger struct {
	t *testing.T
}

func (tl testLogger) Debugf(format string, args ...any) {
	tl.t.Logf(format, args...)
}

func (tl testLogger) Errorf(format string, args ...any) {
	tl.t.Logf(format, args...)
}

func defaultConnectingHandler(connectionCallbacks server.ConnectionCallbacksStruct) func(request *http.Request) types.ConnectionResponse {
	return func(request *http.Request) types.ConnectionResponse {
		return types.ConnectionResponse{
			Accept:              true,
			ConnectionCallbacks: connectionCallbacks,
		}
	}
}

// onConnectingFuncFactory is a function that will be given to server.CallbacksStruct as
// OnConnectingFunc. This allows changing the ConnectionCallbacks both from the newOpAMPServer
// caller and inside of newOpAMP Server, and for custom implementations of the value for `Accept`
// in types.ConnectionResponse.
type onConnectingFuncFactory func(connectionCallbacks server.ConnectionCallbacksStruct) func(request *http.Request) types.ConnectionResponse

type testingOpAMPServer struct {
	addr                string
	supervisorConnected chan bool
	sendToSupervisor    func(*protobufs.ServerToAgent)
}

func newOpAMPServer(t *testing.T, connectingCallback onConnectingFuncFactory, callbacks server.ConnectionCallbacksStruct) *testingOpAMPServer {
	var agentConn atomic.Value
	var isAgentConnected atomic.Bool
	connectedChan := make(chan bool)
	s := server.New(testLogger{t: t})
	onConnectedFunc := callbacks.OnConnectedFunc
	callbacks.OnConnectedFunc = func(conn types.Connection) {
		agentConn.Store(conn)
		isAgentConnected.Store(true)
		connectedChan <- true
		if onConnectedFunc != nil {
			onConnectedFunc(conn)
		}
	}
	onConnectionCloseFunc := callbacks.OnConnectionCloseFunc
	callbacks.OnConnectionCloseFunc = func(conn types.Connection) {
		isAgentConnected.Store(false)
		connectedChan <- false
		if onConnectionCloseFunc != nil {
			onConnectionCloseFunc(conn)
		}
	}
	handler, _, err := s.Attach(server.Settings{
		Callbacks: server.CallbacksStruct{
			OnConnectingFunc: connectingCallback(callbacks),
		},
	})
	require.NoError(t, err)
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/opamp", handler)
	httpSrv := httptest.NewServer(mux)

	shutdown := func() {
		t.Log("Shutting down")
		err := s.Stop(context.Background())
		assert.NoError(t, err)
		httpSrv.Close()
	}
	send := func(msg *protobufs.ServerToAgent) {
		if !isAgentConnected.Load() {
			require.Fail(t, "Agent connection has not been established")
		}

		agentConn.Load().(types.Connection).Send(context.Background(), msg)
	}
	t.Cleanup(func() {
		waitForSupervisorConnection(connectedChan, false)
		shutdown()
	})
	return &testingOpAMPServer{
		addr:                httpSrv.Listener.Addr().String(),
		supervisorConnected: connectedChan,
		sendToSupervisor:    send,
	}
}

func newSupervisor(t *testing.T, configType string, extraConfigData map[string]string) *supervisor.Supervisor {
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
		"goos":      runtime.GOOS,
		"goarch":    runtime.GOARCH,
		"extension": extension,
	}

	for key, val := range extraConfigData {
		configData[key] = val
	}
	err = templ.Execute(&buf, configData)
	require.NoError(t, err)
	cfgFile, _ := os.CreateTemp(t.TempDir(), "config_*.yaml")
	_, err = cfgFile.Write(buf.Bytes())
	require.NoError(t, err)

	s, err := supervisor.NewSupervisor(zap.NewNop(), cfgFile.Name())
	require.NoError(t, err)

	return s
}

func TestSupervisorStartsCollectorWithRemoteConfig(t *testing.T) {
	var agentConfig atomic.Value
	server := newOpAMPServer(
		t,
		defaultConnectingHandler,
		server.ConnectionCallbacksStruct{
			OnMessageFunc: func(_ types.Connection, message *protobufs.AgentToServer) *protobufs.ServerToAgent {
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
			// so just check that it includes some strings we know to be unique to the remote config.
			return strings.Contains(cfg, inputFile.Name()) && strings.Contains(cfg, outputFile.Name())
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

func TestSupervisorRestartsCollectorAfterBadConfig(t *testing.T) {
	var healthReport atomic.Value
	var agentConfig atomic.Value
	server := newOpAMPServer(
		t,
		defaultConnectingHandler,
		server.ConnectionCallbacksStruct{
			OnMessageFunc: func(_ types.Connection, message *protobufs.AgentToServer) *protobufs.ServerToAgent {
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
		server.ConnectionCallbacksStruct{
			OnMessageFunc: func(_ types.Connection, message *protobufs.AgentToServer) *protobufs.ServerToAgent {
				capabilities.Store(message.Capabilities)

				return &protobufs.ServerToAgent{}
			},
		})

	s := newSupervisor(t, "nocap", map[string]string{"url": server.addr})
	defer s.Shutdown()

	waitForSupervisorConnection(server.supervisorConnected, true)

	require.Eventually(t, func() bool {
		cap := capabilities.Load()

		return cap == uint64(protobufs.AgentCapabilities_AgentCapabilities_ReportsStatus)
	}, 5*time.Second, 250*time.Millisecond)
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

	outputFile, err := os.CreateTemp(tempDir, "output_*.yaml")
	require.NoError(t, err)

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
