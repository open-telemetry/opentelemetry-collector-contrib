// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dockerstatsreceiver

import (
	"bytes"
	"context"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap/zaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/docker"
)

var mockFolder = filepath.Join("testdata", "mock")

func TestNewLogsReceiver(t *testing.T) {
	cfg := &Config{
		Config: docker.Config{
			Endpoint:         "unix:///run/some.sock",
			DockerAPIVersion: defaultDockerAPIVersion,
		},
	}
	lr := newLogsReceiver(receivertest.NewNopSettings(), cfg, &consumertest.LogsSink{})
	assert.NotNil(t, lr)
}

func TestErrorsInStartLogs(t *testing.T) {
	unreachable := "unix:///not/a/thing.sock"
	cfg := &Config{
		Config: docker.Config{
			Endpoint:         unreachable,
			DockerAPIVersion: defaultDockerAPIVersion,
		},
	}
	recv := newLogsReceiver(receivertest.NewNopSettings(), cfg, &consumertest.LogsSink{})
	assert.NotNil(t, recv)

	cfg.Endpoint = "..not/a/valid/endpoint"
	err := recv.Start(context.Background(), componenttest.NewNopHost())
	assert.ErrorContains(t, err, "unable to parse docker host")

	cfg.Endpoint = unreachable
	err = recv.Start(context.Background(), componenttest.NewNopHost())
	assert.ErrorContains(t, err, "context deadline exceeded")
}

func createEventsMockServer(t *testing.T, eventsFilePaths []string) (*httptest.Server, error) {
	t.Helper()
	eventsPayloads := make([][]byte, len(eventsFilePaths))
	for i, eventPath := range eventsFilePaths {
		events, err := os.ReadFile(filepath.Clean(eventPath))
		if err != nil {
			return nil, err
		}
		eventsPayloads[i] = events
	}
	containerID := "73364842ef014441cac89fed05df19463b1230db25a31252cdf82e754f1ec581"
	containerInfo := map[string]string{
		"/v1.25/containers/json":                      filepath.Join(mockFolder, "single_container_with_optional_resource_attributes", "containers.json"),
		"/v1.25/containers/" + containerID + "/json":  filepath.Join(mockFolder, "single_container_with_optional_resource_attributes", "container.json"),
		"/v1.25/containers/" + containerID + "/stats": filepath.Join(mockFolder, "single_container_with_optional_resource_attributes", "stats.json"),
	}
	urlToFileContents := make(map[string][]byte, len(containerInfo))
	for urlPath, filePath := range containerInfo {
		err := func() error {
			fileContents, err := os.ReadFile(filepath.Clean(filePath))
			if err != nil {
				return err
			}
			urlToFileContents[urlPath] = fileContents
			return nil
		}()
		if err != nil {
			return nil, err
		}
	}

	eventCallCount := 0
	failureCount := 0
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	return httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.Header().Set("Content-Type", "application/json")
		switch req.URL.Path {
		case "/v1.25/events":
			if eventCallCount >= len(eventsPayloads) {
				return
			}
			// the receiver should be resilient and reconnect on http failures.
			// randomly, return 500 up to 3 times and assert the receiver retry behavior.
			if failureCount < 3 && r.Float32() < 0.4 {
				rw.WriteHeader(http.StatusInternalServerError)
				failureCount++
				return
			}
			for _, event := range bytes.Split(eventsPayloads[eventCallCount], []byte{'\n'}) {
				if len(bytes.TrimSpace(event)) == 0 {
					continue
				}
				_, err := rw.Write(append(event, '\n'))
				assert.NoError(t, err)
				rw.(http.Flusher).Flush()
			}
			eventCallCount++
			// test out disconnection/reconnection capability
			conn, _, err := rw.(http.Hijacker).Hijack()
			assert.NoError(t, err)
			err = conn.Close()
			assert.NoError(t, err)
		default:
			data, ok := urlToFileContents[req.URL.Path]
			if !ok {
				rw.WriteHeader(http.StatusNotFound)
				return
			}
			_, err := rw.Write(data)
			assert.NoError(t, err)
		}
	})), nil
}

type dockerLogEventTestCase struct {
	name          string
	expectedBody  map[string]any
	expectedAttrs map[string]any
	timestamp     time.Time
}

func TestDockerEventPolling(t *testing.T) {
	// events across connections should be aggregated
	testCases := []dockerLogEventTestCase{
		{
			name: "container health status event",
			expectedBody: map[string]any{
				"image": "alpine:latest",
				"name":  "test-container",
			},
			expectedAttrs: map[string]any{
				"event.name":  "docker.container.health_status.running",
				"event.scope": "local",
				"event.id":    "f97ed5bca0a5a0b85bfd52c4144b96174e825c92a138bc0458f0e196f2c7c1b4",
			},
			timestamp: time.Unix(1699483576, 1699483576081311000),
		},
		{
			name: "network create event",
			expectedBody: map[string]any{
				"name":   "test-network",
				"type":   "bridge",
				"driver": "bridge",
			},
			expectedAttrs: map[string]any{
				"event.name":  "docker.network.create",
				"event.scope": "swarm",
				"event.id":    "8c0b5d75f8fb4c06b31f25e9d3c2702827d7a43c82a26c1538ddd5f7b3307d05",
			},
			timestamp: time.Unix(1699483577, 1699483577123456000),
		},
		{
			name: "volume destroy event with no attributes",
			expectedAttrs: map[string]any{
				"event.name": "docker.volume.destroy",
				"event.id":   "def456789",
			},
			timestamp: time.Unix(1699483578, 1699483578123456000),
		},
		{
			name: "daemon start event with empty id",
			expectedBody: map[string]any{
				"name": "docker-daemon",
			},
			expectedAttrs: map[string]any{
				"event.name":  "docker.daemon.start",
				"event.scope": "local",
			},
			timestamp: time.Unix(1699483579, 1699483579123456000),
		},
	}

	mockDockerEngine, err := createEventsMockServer(t, []string{
		filepath.Join(mockFolder, "single_container", "events.json"),
		filepath.Join(mockFolder, "single_container", "events2.json"),
	})
	require.NoError(t, err)
	defer mockDockerEngine.Close()
	mockLogsConsumer := &consumertest.LogsSink{}
	rcv := newLogsReceiver(
		receiver.Settings{
			TelemetrySettings: component.TelemetrySettings{
				Logger: zaptest.NewLogger(t),
			},
		}, &Config{
			Config: docker.Config{
				Endpoint:         mockDockerEngine.URL,
				DockerAPIVersion: defaultDockerAPIVersion,
				Timeout:          time.Second,
			},
		}, mockLogsConsumer)
	err = rcv.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() { require.NoError(t, rcv.Shutdown(context.Background())) }()

	require.Eventually(t, func() bool {
		return len(mockLogsConsumer.AllLogs()) == len(testCases)
	}, 5*time.Second, 100*time.Millisecond)
	logs := mockLogsConsumer.AllLogs()
	require.Equal(t, len(logs), len(testCases))

	for i, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			curLogRecord := logs[i].ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
			if tc.expectedBody == nil {
				assert.Nil(t, curLogRecord.Body().AsRaw())
			} else {
				assert.Equal(t, tc.expectedBody, curLogRecord.Body().AsRaw())
			}
			assert.Equal(t, tc.expectedAttrs, curLogRecord.Attributes().AsRaw())
			assert.Equal(t, pcommon.NewTimestampFromTime(tc.timestamp), curLogRecord.Timestamp())
		})
	}
}
