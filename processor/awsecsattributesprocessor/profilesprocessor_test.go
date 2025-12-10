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
	"go.opentelemetry.io/collector/pdata/pprofile"
	"go.uber.org/zap"
)

var testprofilesprocessor *profilesProcessor

func initProfilesProcessor() {
	server := httptest.NewServer(http.HandlerFunc(mockMetadataEndpoint))
	testprofilesprocessor = newProfilesProcessor(
		context.Background(),
		zap.NewExample(),
		createDefaultConfig().(*Config), nil,
		func(logger *zap.Logger, ctx context.Context) (map[string][]string, error) {
			return map[string][]string{
				testcontainerID: {server.URL},
			}, nil
		}, getContainerData)
}

func TestProfilesMetadataHandlerGet(t *testing.T) {
	initProfilesProcessor()
	ctx := context.Background()
	require.NoError(t, testprofilesprocessor.syncMetadata(ctx, testendpoints))

	v, err := testprofilesprocessor.get(testcontainerID)
	flat := v.Flat()
	for k, val := range expectedFlattenedMetadata {
		assert.Equal(t, flat[k], val, "bad key: %s", k)
	}
	assert.NoError(t, err)
	assert.Equal(t, len(expectedFlattenedMetadata), len(v.Flat()))
}

type testProfilesConsumer struct {
	t     *testing.T
	match string
	len   int
}

func (c *testProfilesConsumer) ConsumeProfiles(ctx context.Context, pd pprofile.Profiles) error {
	var matches int
	pd.ResourceProfiles().At(0).Resource().Attributes().Range(func(k string, v pcommon.Value) bool {
		if regexp.MustCompile(c.match).MatchString(k) {
			matches += 1
		}
		return true
	})

	assert.Equal(c.t, c.len, matches)

	numOfAttributes := pd.ResourceProfiles().At(0).Resource().Attributes().Len()
	if numOfAttributes < 34 {
		assert.Equal(c.t, c.len+1, numOfAttributes)
	}
	return nil
}

func (c *testProfilesConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: true}
}

func TestProcessProfilesFunc(t *testing.T) {
	initProfilesProcessor()

	defaultRecord := func() pprofile.Profiles {
		pd := pprofile.NewProfiles()
		pd.ResourceProfiles().AppendEmpty().Resource().Attributes().PutStr("container.id", testcontainerID)
		return pd
	}

	tests := []struct {
		name     string
		config   *Config
		wantErr  bool
		len      int
		match    string
		record   func() pprofile.Profiles
		consumer *testProfilesConsumer
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

			consumer: &testProfilesConsumer{
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

			consumer: &testProfilesConsumer{
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

			consumer: &testProfilesConsumer{
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
			consumer: &testProfilesConsumer{
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

			record: func() pprofile.Profiles {
				pd := pprofile.NewProfiles()
				pd.ResourceProfiles().AppendEmpty().Resource().Attributes().PutStr("resource.container.id", testcontainerID)
				return pd
			},

			consumer: &testProfilesConsumer{
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
			consumer: &testProfilesConsumer{
				t:     t,
				match: ".*",
				len:   9,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testprofilesprocessor.cfg = tt.config
			testprofilesprocessor.setNextConsumer(tt.consumer)

			err := tt.config.init()
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			err = testprofilesprocessor.ConsumeProfiles(context.Background(), tt.record())
			require.NoError(t, err)
		})
	}
}

// TestProfilesDockerClientLifecycleBug demonstrates that defer cli.Close() in Start()
// closes the Docker client immediately after Start() returns, breaking the goroutine
// that watches for Docker events.
func TestProfilesDockerClientLifecycleBug(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Docker integration test in short mode")
	}

	server := httptest.NewServer(http.HandlerFunc(mockMetadataEndpoint))
	defer server.Close()

	syncCount := 0
	syncMutex := &sync.Mutex{}

	pp := newProfilesProcessor(
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
	err := pp.Start(ctx, nil)
	require.NoError(t, err)

	syncMutex.Lock()
	initialSyncCount := syncCount
	syncMutex.Unlock()
	assert.Greater(t, initialSyncCount, 0, "Initial sync should have occurred")

	time.Sleep(200 * time.Millisecond)

	err = pp.Shutdown(ctx)
	require.NoError(t, err)
}
