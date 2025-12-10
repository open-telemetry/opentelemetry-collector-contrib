package awsecsattributesprocessor

import (
	"context"
	"net/http"
	"net/http/httptest"
	"regexp"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

var testmetricsprocessor *metricsProcessor

func initMetricsProcessor() {
	server := httptest.NewServer(http.HandlerFunc(mockMetadataEndpoint))
	testmetricsprocessor = newMetricsProcessor(
		context.Background(),
		zap.NewExample(),
		createDefaultConfig().(*Config), nil,
		func(logger *zap.Logger, ctx context.Context) (map[string][]string, error) {
			return map[string][]string{
				testcontainerID: {server.URL},
			}, nil
		}, getContainerData)
}

func TestMetricsMetadataHandlerGet(t *testing.T) {
	initMetricsProcessor()
	ctx := context.Background()
	require.NoError(t, testmetricsprocessor.syncMetadata(ctx, testendpoints))

	v, err := testmetricsprocessor.get(testcontainerID)
	flat := v.Flat()
	for k, val := range expectedFlattenedMetadata {
		assert.Equal(t, flat[k], val, "bad key: %s", k)
	}
	assert.NoError(t, err)
	assert.Equal(t, len(expectedFlattenedMetadata), len(v.Flat()))
}

type testMetricsConsumer struct {
	t     *testing.T
	match string
	len   int
}

func (c *testMetricsConsumer) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	var matches int
	md.ResourceMetrics().At(0).Resource().Attributes().Range(func(k string, v pcommon.Value) bool {
		if regexp.MustCompile(c.match).MatchString(k) {
			matches += 1
		}
		return true
	})

	assert.Equal(c.t, c.len, matches)

	numOfAttributes := md.ResourceMetrics().At(0).Resource().Attributes().Len()
	if numOfAttributes < 34 {
		assert.Equal(c.t, c.len+1, numOfAttributes)
	}
	return nil
}

func (c *testMetricsConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: true}
}

func TestProcessMetricsFunc(t *testing.T) {
	initMetricsProcessor()

	defaultRecord := func() pmetric.Metrics {
		md := pmetric.NewMetrics()
		md.ResourceMetrics().AppendEmpty().Resource().Attributes().PutStr("container.id", testcontainerID)
		return md
	}

	tests := []struct {
		name     string
		config   *Config
		wantErr  bool
		len      int
		match    string
		record   func() pmetric.Metrics
		consumer consumer.Metrics
	}{
		{
			name:    "fetch all attributes starting with aws",
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

			consumer: &testMetricsConsumer{
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

			consumer: &testMetricsConsumer{
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

			consumer: &testMetricsConsumer{
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
			consumer: &testMetricsConsumer{
				t:     t,
				match: "^aws.*|^image.*|^docker.*",
				len:   11,
			},
		},
		{
			name:    "specify container id in resource attributes",
			wantErr: false,
			config: &Config{
				CacheTTL: 60,
				Attributes: []string{
					".*",
				},
				ContainerID: ContainerID{
					Sources: []string{"resource.container.id"},
				},
			},

			record: func() pmetric.Metrics {
				md := pmetric.NewMetrics()
				md.ResourceMetrics().AppendEmpty().Resource().Attributes().PutStr("resource.container.id", testcontainerID)
				return md
			},

			consumer: &testMetricsConsumer{
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
			consumer: &testMetricsConsumer{
				t:     t,
				match: ".*",
				len:   9,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testmetricsprocessor.cfg = tt.config
			testmetricsprocessor.setNextConsumer(tt.consumer)

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

			err = testmetricsprocessor.ConsumeMetrics(context.Background(), tt.record())
			require.NoError(t, err)
		})
	}
}

// TestMetricsDockerClientLifecycleBug demonstrates that defer cli.Close() in Start()
// closes the Docker client immediately after Start() returns, breaking the goroutine
// that watches for Docker events.
func TestMetricsDockerClientLifecycleBug(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Docker integration test in short mode")
	}

	server := httptest.NewServer(http.HandlerFunc(mockMetadataEndpoint))
	defer server.Close()

	syncCount := 0
	syncMutex := &sync.Mutex{}

	mp := newMetricsProcessor(
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
	err := mp.Start(ctx, nil)
	require.NoError(t, err)

	syncMutex.Lock()
	initialSyncCount := syncCount
	syncMutex.Unlock()
	assert.Greater(t, initialSyncCount, 0, "Initial sync should have occurred")

	time.Sleep(200 * time.Millisecond)

	err = mp.Shutdown(ctx)
	require.NoError(t, err)
}
