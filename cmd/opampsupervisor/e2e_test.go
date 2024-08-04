// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build e2e

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
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
	"google.golang.org/protobuf/proto"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor"
	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor/config"
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

func defaultConnectingHandler(connectionCallbacks server.ConnectionCallbacksStruct) func(request *http.Request) types.ConnectionResponse {
	return func(_ *http.Request) types.ConnectionResponse {
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
	start               func()
	shutdown            func()
}

func newOpAMPServer(t *testing.T, connectingCallback onConnectingFuncFactory, callbacks server.ConnectionCallbacksStruct) *testingOpAMPServer {
	s := newUnstartedOpAMPServer(t, connectingCallback, callbacks)
	s.start()
	return s
}

func newUnstartedOpAMPServer(t *testing.T, connectingCallback onConnectingFuncFactory, callbacks server.ConnectionCallbacksStruct) *testingOpAMPServer {
	var agentConn atomic.Value
	var isAgentConnected atomic.Bool
	var didShutdown atomic.Bool
	connectedChan := make(chan bool)
	s := server.New(testLogger{t: t})
	onConnectedFunc := callbacks.OnConnectedFunc
	callbacks.OnConnectedFunc = func(ctx context.Context, conn types.Connection) {
		if onConnectedFunc != nil {
			onConnectedFunc(ctx, conn)
		}
		agentConn.Store(conn)
		isAgentConnected.Store(true)
		connectedChan <- true
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
	httpSrv := httptest.NewUnstartedServer(mux)

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
	s, err := supervisor.NewSupervisor(zap.NewNop(), cfgFile.Name())
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
		"storage_dir": t.TempDir(),
	}

	for key, val := range extraConfigData {
		configData[key] = val
	}
	err = templ.Execute(&buf, configData)
	require.NoError(t, err)
	cfgFile, _ := os.CreateTemp(t.TempDir(), "config_*.yaml")
	_, err = cfgFile.Write(buf.Bytes())
	require.NoError(t, err)

	return cfgFile
}

func TestSupervisorStartsCollectorWithRemoteConfig(t *testing.T) {
	var agentConfig atomic.Value
	server := newOpAMPServer(
		t,
		defaultConnectingHandler,
		server.ConnectionCallbacksStruct{
			OnMessageFunc: func(_ context.Context, _ types.Connection, message *protobufs.AgentToServer) *protobufs.ServerToAgent {
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

func TestSupervisorStartsCollectorWithNoOpAMPServer(t *testing.T) {
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

	require.NoError(t, os.WriteFile(remoteConfigFilePath, marshalledRemoteConfig, 0600))

	connected := atomic.Bool{}
	server := newUnstartedOpAMPServer(t, defaultConnectingHandler, server.ConnectionCallbacksStruct{
		OnConnectedFunc: func(ctx context.Context, conn types.Connection) {
			connected.Store(true)
		},
	})
	defer server.shutdown()

	s := newSupervisor(t, "basic", map[string]string{
		"url":         server.addr,
		"storage_dir": storageDir,
	})
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

func TestSupervisorStartsWithNoOpAMPServer(t *testing.T) {
	cfg, hash, inputFile, outputFile := createSimplePipelineCollectorConf(t)

	configuredChan := make(chan struct{})
	connected := atomic.Bool{}
	server := newUnstartedOpAMPServer(t, defaultConnectingHandler, server.ConnectionCallbacksStruct{
		OnConnectedFunc: func(ctx context.Context, conn types.Connection) {
			connected.Store(true)
		},
		OnMessageFunc: func(ctx context.Context, conn types.Connection, message *protobufs.AgentToServer) *protobufs.ServerToAgent {
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
	s := newSupervisor(t, "basic", map[string]string{
		"url": server.addr,
	})
	defer s.Shutdown()

	// Verify the collector is running by checking the metrics endpoint
	require.Eventually(t, func() bool {
		resp, err := http.DefaultClient.Get("http://localhost:8888/metrics")
		if err != nil {
			t.Logf("Failed check for prometheus metrics: %s", err)
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
		server.ConnectionCallbacksStruct{
			OnMessageFunc: func(_ context.Context, _ types.Connection, message *protobufs.AgentToServer) *protobufs.ServerToAgent {
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
			OnMessageFunc: func(_ context.Context, _ types.Connection, message *protobufs.AgentToServer) *protobufs.ServerToAgent {
				capabilities.Store(message.Capabilities)

				return &protobufs.ServerToAgent{}
			},
		})

	s := newSupervisor(t, "nocap", map[string]string{"url": server.addr})
	defer s.Shutdown()

	waitForSupervisorConnection(server.supervisorConnected, true)

	require.Eventually(t, func() bool {
		caps := capabilities.Load()

		return caps == uint64(protobufs.AgentCapabilities_AgentCapabilities_ReportsStatus)
	}, 5*time.Second, 250*time.Millisecond)
}

func TestSupervisorBootstrapsCollector(t *testing.T) {
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
		server.ConnectionCallbacksStruct{
			OnMessageFunc: func(_ context.Context, _ types.Connection, message *protobufs.AgentToServer) *protobufs.ServerToAgent {
				if message.AgentDescription != nil {
					agentDescription.Store(message.AgentDescription)
				}

				return &protobufs.ServerToAgent{}
			},
		})

	s := newSupervisor(t, "nocap", map[string]string{"url": server.addr})
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
}

func TestSupervisorReportsEffectiveConfig(t *testing.T) {
	var agentConfig atomic.Value
	server := newOpAMPServer(
		t,
		defaultConnectingHandler,
		server.ConnectionCallbacksStruct{
			OnMessageFunc: func(_ context.Context, _ types.Connection, message *protobufs.AgentToServer) *protobufs.ServerToAgent {
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

	// Create input and output files so we can "communicate" with a Collector binary.
	// The testing package will automatically clean these up after each test.
	tempDir := t.TempDir()
	testKeyFile, err := os.CreateTemp(tempDir, "confKey")
	require.NoError(t, err)
	n, err := testKeyFile.Write([]byte(testKeyFile.Name()))
	require.NoError(t, err)
	require.NotZero(t, n)

	colCfgTpl, err := os.ReadFile(path.Join("testdata", "collector", "split_config.yaml"))
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
		server.ConnectionCallbacksStruct{
			OnMessageFunc: func(_ context.Context, _ types.Connection, message *protobufs.AgentToServer) *protobufs.ServerToAgent {
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

	require.Equal(t, expectedDescription, ad.AgentDescription)

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
		server.ConnectionCallbacksStruct{
			OnMessageFunc: func(_ context.Context, _ types.Connection, message *protobufs.AgentToServer) *protobufs.ServerToAgent {
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
	}, 10*time.Second, 250*time.Millisecond, "Collector never reported healthy after restart")
}

func TestSupervisorOpAMPConnectionSettings(t *testing.T) {
	var connectedToNewServer atomic.Bool
	initialServer := newOpAMPServer(
		t,
		defaultConnectingHandler,
		server.ConnectionCallbacksStruct{})

	s := newSupervisor(t, "accepts_conn", map[string]string{"url": initialServer.addr})
	defer s.Shutdown()

	waitForSupervisorConnection(initialServer.supervisorConnected, true)

	newServer := newOpAMPServer(
		t,
		defaultConnectingHandler,
		server.ConnectionCallbacksStruct{
			OnConnectedFunc: func(_ context.Context, _ types.Connection) {
				connectedToNewServer.Store(true)
			},
			OnMessageFunc: func(_ context.Context, _ types.Connection, _ *protobufs.AgentToServer) *protobufs.ServerToAgent {
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

func TestSupervisorRestartsWithLastReceivedConfig(t *testing.T) {
	// Create a temporary directory to store the test config file.
	tempDir := t.TempDir()

	var agentConfig atomic.Value
	initialServer := newOpAMPServer(
		t,
		defaultConnectingHandler,
		server.ConnectionCallbacksStruct{
			OnMessageFunc: func(_ context.Context, _ types.Connection, message *protobufs.AgentToServer) *protobufs.ServerToAgent {
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
		server.ConnectionCallbacksStruct{
			OnMessageFunc: func(_ context.Context, _ types.Connection, message *protobufs.AgentToServer) *protobufs.ServerToAgent {
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
		server.ConnectionCallbacksStruct{
			OnMessageFunc: func(_ context.Context, _ types.Connection, message *protobufs.AgentToServer) *protobufs.ServerToAgent {

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
		server.ConnectionCallbacksStruct{
			OnMessageFunc: func(_ context.Context, _ types.Connection, message *protobufs.AgentToServer) *protobufs.ServerToAgent {

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
