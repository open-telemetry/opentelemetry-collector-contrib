package awsecsattributesprocessor

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
)

const payload = `{
	"ContainerARN": "arn:aws:ecs:eu-west-1:035955823396:container/cds-305/ec7ff82b7a3a44a5bbbe9bcf11daee33/cc1c133f-bd1f-4006-8dae-4cd8a3f54f19",
	"CreatedAt": "2023-06-22T12:41:18.335883278Z",
	"DesiredStatus": "RUNNING",
	"DockerId": "196a0e6abfce1e33ee24b65e97875f089878dd7d1d7e9f15155d6094c8b908f5",
	"DockerName": "ecs-cadvisor-task-definition-7-cadvisor-bae592b5e4c1a3bb3800",
	"Image": "gcr.io/cadvisor/cadvisor:latest",
	"ImageID": "sha256:68c29634fe49724f94ed34f18224336f776392f7a5a4014969ac5798a2ec96dc",
	"KnownStatus": "RUNNING",
	"Labels": {
	  "com.custom.app": "go-test-app",
	  "com.amazonaws.ecs.cluster": "cds-305",
	  "com.amazonaws.ecs.container-name": "cadvisor",
	  "com.amazonaws.ecs.task-arn": "arn:aws:ecs:eu-west-1:035955823396:task/cds-305/ec7ff82b7a3a44a5bbbe9bcf11daee33",
	  "com.amazonaws.ecs.task-definition-family": "cadvisor-task-definition",
	  "com.amazonaws.ecs.task-definition-version": "7"
	},
	"Limits": {
	  "CPU": 10,
	  "Memory": 300
	},
	"Name": "cadvisor",
	"Networks": [
	  {
		"IPv4Addresses": [
		  "172.17.0.2"
		],
		"NetworkMode": "bridge"
	  }
	],
	"Ports": [
	  {
		"ContainerPort": 8080,
		"HostIp": "0.0.0.0",
		"HostPort": 32911,
		"Protocol": "tcp"
	  },
	  {
		"ContainerPort": 8080,
		"HostIp": "::",
		"HostPort": 32911,
		"Protocol": "tcp"
	  }
	],
	"StartedAt": "2023-06-22T12:41:18.713571182Z",
	"Type": "NORMAL",
	"Volumes": [
	  {
		"Destination": "/var",
		"Source": "/var"
	  },
	  {
		"Destination": "/etc",
		"Source": "/etc"
	  }
	]
  }
`

var (
	testcontainerID           = "9e160da5ce00c77637967534f4390f39d9d78137e72666dc3cc52eabf211bdae"
	testendpoints             = make(map[string][]string)
	expectedFlattenedMetadata = map[string]interface{}{
		"aws.ecs.cluster":                 "cds-305",
		"aws.ecs.container.arn":           "arn:aws:ecs:eu-west-1:035955823396:container/cds-305/ec7ff82b7a3a44a5bbbe9bcf11daee33/cc1c133f-bd1f-4006-8dae-4cd8a3f54f19",
		"aws.ecs.container.name":          "cadvisor",
		"aws.ecs.task.arn":                "arn:aws:ecs:eu-west-1:035955823396:task/cds-305/ec7ff82b7a3a44a5bbbe9bcf11daee33",
		"aws.ecs.task.definition.family":  "cadvisor-task-definition",
		"aws.ecs.task.definition.version": "7",
		"aws.ecs.task.known.status":       "RUNNING",
		"created.at":                      "2023-06-22T12:41:18.335883278Z",
		"desired.status":                  "RUNNING",
		"docker.id":                       "196a0e6abfce1e33ee24b65e97875f089878dd7d1d7e9f15155d6094c8b908f5",
		"docker.name":                     "ecs-cadvisor-task-definition-7-cadvisor-bae592b5e4c1a3bb3800",
		"image":                           "gcr.io/cadvisor/cadvisor:latest",
		"image.id":                        "sha256:68c29634fe49724f94ed34f18224336f776392f7a5a4014969ac5798a2ec96dc",
		"limits.cpu":                      10,
		"limits.memory":                   300,
		"name":                            "cadvisor",
		"networks.0.ipv4.addresses.0":     "172.17.0.2",
		"networks.0.network.mode":         "bridge",
		"ports.0.container.port":          8080,
		"ports.0.host.ip":                 "0.0.0.0",
		"ports.0.host.port":               32911,
		"ports.0.protocol":                "tcp",
		"ports.1.container.port":          8080,
		"ports.1.host.ip":                 "::",
		"ports.1.host.port":               32911,
		"ports.1.protocol":                "tcp",
		"started.at":                      "2023-06-22T12:41:18.713571182Z",
		"type":                            "NORMAL",
		"volumes.0.destination":           "/var",
		"volumes.0.source":                "/var",
		"volumes.1.destination":           "/etc",
		"volumes.1.source":                "/etc",
		"labels.com.custom.app":           "go-test-app",
	}
)

func mockMetadataEndpoint(w http.ResponseWriter, r *http.Request) {
	var m map[string]any
	json.Unmarshal([]byte(payload), &m)
	sc := http.StatusOK
	w.WriteHeader(sc)
	json.NewEncoder(w).Encode(m)
}

var testlogprocessor *logsProcessor

func TestMain(m *testing.M) {
	// setup()
	server := httptest.NewServer(http.HandlerFunc(mockMetadataEndpoint))
	testlogprocessor = newLogsProcessor(
		context.Background(),
		zap.NewExample(),
		createDefaultConfig().(*Config), nil,
		func(logger *zap.Logger, ctx context.Context) (map[string][]string, error) {
			return map[string][]string{
				testcontainerID: {server.URL},
			}, nil
		}, getContainerData)

	testendpoints[testcontainerID] = []string{server.URL}

	code := m.Run()
	server.Close()
	os.Exit(code)
}

func TestMetadataHandlerGet(t *testing.T) {
	ctx := context.Background()
	require.NoError(t, testlogprocessor.syncMetadata(ctx, testendpoints))

	v, err := testlogprocessor.get(testcontainerID)
	flat := v.Flat()
	for k, val := range expectedFlattenedMetadata {
		assert.Equal(t, flat[k], val, "bad key: %s", k)
	}
	assert.NoError(t, err)
	assert.Equal(t, len(expectedFlattenedMetadata), len(v.Flat()))
}

type testconcumer struct {
	t     *testing.T
	match string
	len   int
}

func (c *testconcumer) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	var matches int
	ld.ResourceLogs().At(0).Resource().Attributes().Range(func(k string, v pcommon.Value) bool {
		if regexp.MustCompile(c.match).MatchString(k) {
			matches += 1
		}
		return true
	})

	assert.Equal(c.t, c.len, matches)

	numOfAttributes := ld.ResourceLogs().At(0).Resource().Attributes().Len()
	if numOfAttributes < 34 {
		assert.Equal(c.t, c.len+1, numOfAttributes)
	}
	return nil
}

func (c *testconcumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: true}
}

func TestProcessLogFunc(t *testing.T) {
	defaultRecord := func() plog.Logs {
		ld := plog.NewLogs()
		ld.ResourceLogs().AppendEmpty().Resource().Attributes().PutStr("container.id", testcontainerID)
		return ld
	}

	tests := []struct {
		name     string
		config   *Config
		wantErr  bool
		len      int
		match    string
		record   func() plog.Logs
		consumer consumer.Logs
	}{
		{
			name:    "fetch all attributes starting with ecs",
			wantErr: false,
			record:  defaultRecord,
			config: &Config{
				CacheTTL: 60,
				Attributes: []string{
					"^aws.*",
				},
				ContainerID: ContainerID{
					Sources: []string{"container.id"},
				},
			},

			consumer: &testconcumer{
				t:     t,
				match: "^aws.*",
				len:   7,
			},
		},
		{
			name:    "fetch all attributes",
			wantErr: false,
			record:  defaultRecord,
			config: &Config{
				CacheTTL: 60,
				Attributes: []string{
					".*",
				},
				ContainerID: ContainerID{
					Sources: []string{"container.id"},
				},
			},

			consumer: &testconcumer{
				t:     t,
				match: ".*",
				len:   34,
			},
		},
		{
			name:    "fetch default attributes",
			wantErr: false,
			record:  defaultRecord,
			config: func() *Config {
				c := createDefaultConfig().(*Config)
				c.ContainerID.Sources = append(c.ContainerID.Sources, "container.id")
				return c
			}(),

			consumer: &testconcumer{
				t:     t,
				match: "^aws.*|^image.*|^docker.*|^labels.*",
				len:   12,
			},
		},
		{
			name:    "no container id path",
			config:  createDefaultConfig().(*Config),
			wantErr: true,
			record:  defaultRecord,
			consumer: &testconcumer{
				t:     t,
				match: "^aws.*|^image.*|^docker.*",
				len:   11,
			},
		},
		{
			name:    "specify container id as as log.file.name",
			wantErr: false,
			config: &Config{
				CacheTTL: 60,
				Attributes: []string{
					".*",
				},
				ContainerID: ContainerID{
					Sources: []string{"log.file.name"},
				},
			},

			record: func() plog.Logs {
				ld := plog.NewLogs()
				ld.ResourceLogs().AppendEmpty().Resource().Attributes().PutStr("log.file.name", testcontainerID+"-json.log")
				return ld
			},

			consumer: &testconcumer{
				t:     t,
				match: ".*",
				len:   34,
			},
		},
		{
			name:    "bad regex",
			wantErr: true,
			config: &Config{
				Attributes: []string{
					"?=",
				},
				ContainerID: ContainerID{
					Sources: []string{"container.id"},
				},
			},

			record: defaultRecord,
			consumer: &testconcumer{
				t:     t,
				match: ".*",
				len:   9,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testlogprocessor.cfg = tt.config
			testlogprocessor.setNextConsumer(tt.consumer)

			// validate config first
			err := tt.config.Validate()
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			// then initialize
			err = tt.config.init()
			require.NoError(t, err)

			err = testlogprocessor.ConsumeLogs(context.Background(), tt.record())
			require.NoError(t, err)
		})
	}
}

// TestLogsDockerClientLifecycleBug demonstrates that defer cli.Close() in Start()
// closes the Docker client immediately after Start() returns, breaking the goroutine
// that watches for Docker events.
func TestLogsDockerClientLifecycleBug(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Docker integration test in short mode")
	}

	server := httptest.NewServer(http.HandlerFunc(mockMetadataEndpoint))
	defer server.Close()

	syncCount := 0
	syncMutex := &sync.Mutex{}

	lp := newLogsProcessor(
		context.Background(),
		zap.NewExample(),
		createDefaultConfig().(*Config),
		nil,
		func(logger *zap.Logger, ctx context.Context) (map[string][]string, error) {
			syncMutex.Lock()
			syncCount++
			syncMutex.Unlock()
			return map[string][]string{testcontainerID: {server.URL}}, nil
		},
		getContainerData,
	)

	ctx := context.Background()
	err := lp.Start(ctx, nil)
	require.NoError(t, err)

	syncMutex.Lock()
	initialSyncCount := syncCount
	syncMutex.Unlock()
	assert.Greater(t, initialSyncCount, 0, "Initial sync should have occurred")

	time.Sleep(200 * time.Millisecond)

	err = lp.Shutdown(ctx)
	require.NoError(t, err)
}
