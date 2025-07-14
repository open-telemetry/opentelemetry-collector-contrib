// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlecloudspannerreceiver

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver/internal/datasource"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver/internal/statsreader"
)

const (
	serviceAccountValidPath   = "testdata/serviceAccount.json"
	serviceAccountInvalidPath = "does not exist"
)

// mockCompositeReader is a mock implementation of statsreader.CompositeReader for testing.
type mockCompositeReader struct {
	mock.Mock
}

func (r *mockCompositeReader) Name() string {
	args := r.Called()
	return args.String(0)
}

func (r *mockCompositeReader) Read(ctx context.Context) ([]*metadata.MetricsDataPoint, error) {
	args := r.Called(ctx)
	if dataPoints, ok := args.Get(0).([]*metadata.MetricsDataPoint); ok {
		return dataPoints, args.Error(1)
	}
	return nil, args.Error(1)
}

func (r *mockCompositeReader) Shutdown() {
	r.Called()
}

// metricsBuilder is a mock implementation of metadata.MetricsBuilder for testing.
type metricsBuilder struct {
	throwErrorOnShutdown bool
}

func newMetricsBuilder(throwErrorOnShutdown bool) metadata.MetricsBuilder {
	return &metricsBuilder{
		throwErrorOnShutdown: throwErrorOnShutdown,
	}
}

func (b *metricsBuilder) Build([]*metadata.MetricsDataPoint) (pmetric.Metrics, error) {
	return pmetric.NewMetrics(), nil
}

func (b *metricsBuilder) Shutdown() error {
	if b.throwErrorOnShutdown {
		return errors.New("error on shutdown")
	}
	return nil
}

// createStaticConfig creates a config with only statically defined databases.
func createStaticConfig() *Config {
	cfg := createDefaultConfig().(*Config)
	cfg.Projects = []Project{
		{
			ID: "project-static",
			Instances: []Instance{
				{
					ID:        "instance-static",
					Databases: []string{"db-static-1"},
				},
			},
			ServiceAccountKey: serviceAccountValidPath,
		},
	}
	return cfg
}

// createDynamicConfig creates a config with only dynamically defined instances.
func createDynamicConfig() *Config {
	cfg := createDefaultConfig().(*Config)
	cfg.Projects = []Project{
		{
			ID: "project-dynamic",
			Instances: []Instance{
				{
					ID:                 "instance-dynamic",
					ScrapeAllDatabases: true,
				},
			},
		},
	}
	return cfg
}

func TestNewGoogleCloudSpannerReceiver(t *testing.T) {
	logger := zaptest.NewLogger(t)
	cfg := createDefaultConfig().(*Config)
	receiver := newGoogleCloudSpannerReceiver(logger, cfg)

	require.NotNil(t, receiver)
	assert.Equal(t, logger, receiver.logger)
	assert.Equal(t, cfg, receiver.config)
}

func TestStart(t *testing.T) {
	testCases := map[string]struct {
		serviceAccountPath string
		expectError        bool
	}{
		"Happy path":                {serviceAccountValidPath, false},
		"With initialization error": {serviceAccountInvalidPath, true},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			logger := zaptest.NewLogger(t)
			cfg := createStaticConfig()
			cfg.Projects[0].ServiceAccountKey = testCase.serviceAccountPath
			host := componenttest.NewNopHost()

			receiver := newGoogleCloudSpannerReceiver(logger, cfg)
			require.NotNil(t, receiver)

			err := receiver.Start(context.Background(), host)

			if testCase.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Len(t, receiver.projectReaders, 1)
			}
		})
	}
}

func TestInitialize(t *testing.T) {
	testCases := map[string]struct {
		serviceAccountPath    string
		expectError           bool
		replaceMetadataConfig bool
	}{
		"Happy path":                 {serviceAccountValidPath, false, false},
		"With error":                 {serviceAccountInvalidPath, true, false},
		"With metadata config error": {serviceAccountValidPath, true, true},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			logger := zaptest.NewLogger(t)
			cfg := createStaticConfig()
			cfg.Projects[0].ServiceAccountKey = testCase.serviceAccountPath

			receiver := newGoogleCloudSpannerReceiver(logger, cfg)
			require.NotNil(t, receiver)

			originalMetadataYaml := metadataYaml
			if testCase.replaceMetadataConfig {
				metadataYaml = []byte("invalid-yaml")
			}

			err := receiver.initialize(context.Background())

			metadataYaml = originalMetadataYaml // Restore original yaml

			if testCase.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestInitializeStaticProjectReaders(t *testing.T) {
	testCases := map[string]struct {
		serviceAccountPath string
		expectError        bool
	}{
		"Happy path": {serviceAccountValidPath, false},
		"With error": {serviceAccountInvalidPath, true},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			logger := zaptest.NewLogger(t)
			cfg := createStaticConfig()
			cfg.Projects[0].ServiceAccountKey = testCase.serviceAccountPath

			receiver := newGoogleCloudSpannerReceiver(logger, cfg)
			require.NotNil(t, receiver)

			err := receiver.initializeStaticProjectReaders(context.Background(), []*metadata.MetricsMetadata{})

			if testCase.expectError {
				require.Error(t, err)
				assert.Empty(t, receiver.projectReaders)
			} else {
				require.NoError(t, err)
				assert.Len(t, receiver.projectReaders, 1)
			}
		})
	}
}

func TestNewStaticProjectReader(t *testing.T) {
	testCases := map[string]struct {
		serviceAccountPath string
		expectError        bool
	}{
		"Happy path": {serviceAccountValidPath, false},
		"With error": {serviceAccountInvalidPath, true},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			logger := zaptest.NewLogger(t)
			cfg := createStaticConfig()
			cfg.Projects[0].ServiceAccountKey = testCase.serviceAccountPath
			var parsedMetadata []*metadata.MetricsMetadata

			reader, err := newStaticProjectReader(context.Background(), logger, cfg.Projects[0], parsedMetadata,
				statsreader.ReaderConfig{})

			if testCase.expectError {
				require.Error(t, err)
				assert.Nil(t, reader)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, reader)
			}
		})
	}
}

func TestScrape_StaticMode(t *testing.T) {
	logger := zaptest.NewLogger(t)
	cfg := createStaticConfig()
	receiver := newGoogleCloudSpannerReceiver(logger, cfg)
	require.NotNil(t, receiver)

	mockReader := &mockCompositeReader{}
	mockPr := statsreader.NewProjectReader([]statsreader.CompositeReader{mockReader}, logger)
	receiver.projectReaders = []*statsreader.ProjectReader{mockPr}
	receiver.metricsBuilder = newMetricsBuilder(false)

	ctx := context.Background()
	mockReader.On("Read", ctx).Return([]*metadata.MetricsDataPoint{}, nil).Once()

	_, err := receiver.Scrape(ctx)

	require.NoError(t, err)
	mockReader.AssertExpectations(t)
}

func TestShutdown(t *testing.T) {
	logger := zaptest.NewLogger(t)
	mockReader := &mockCompositeReader{}
	projectReader := statsreader.NewProjectReader([]statsreader.CompositeReader{mockReader}, logger)

	testCases := map[string]struct {
		metricsBuilder metadata.MetricsBuilder
		expectError    bool
	}{
		"Happy path": {newMetricsBuilder(false), false},
		"With error": {newMetricsBuilder(true), true},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			receiver := &googleCloudSpannerReceiver{
				projectReaders:  []*statsreader.ProjectReader{projectReader},
				databaseReaders: map[string]statsreader.CompositeReader{"dynamic/reader": mockReader},
				metricsBuilder:  testCase.metricsBuilder,
			}
			ctx, cancel := context.WithCancel(context.Background())
			receiver.cancel = cancel

			mockReader.On("Shutdown").Return().Twice() // Called for both static and dynamic lists

			err := receiver.Shutdown(ctx)

			if testCase.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			mockReader.AssertExpectations(t)
		})
	}
}

func TestInitialize_WithDynamicDiscovery(t *testing.T) {
	logger := zap.NewNop()
	cfg := createDynamicConfig()
	receiver := newGoogleCloudSpannerReceiver(logger, cfg)
	receiver.metricsBuilder = newMetricsBuilder(false)

	// Mock the list function to return two databases
	receiver.listDatabasesFunc = func(ctx context.Context, projectID, instanceID, serviceAccountKey string) ([]string, error) {
		return []string{"db-dynamic-1", "db-dynamic-2"}, nil
	}

	// Mock the reader creation function
	createdCount := 0
	receiver.newDbReaderFunc = func(ctx context.Context, md []*metadata.MetricsMetadata, id *datasource.DatabaseID, sap string, rc statsreader.ReaderConfig, l *zap.Logger) (statsreader.CompositeReader, error) {
		createdCount++
		return &mockCompositeReader{}, nil
	}

	err := receiver.initialize(context.Background())
	require.NoError(t, err)

	// Verify that two readers were created and stored, and the timer was set.
	assert.Equal(t, 2, createdCount, "Expected two readers to be created")
	assert.Len(t, receiver.databaseReaders, 2, "Expected two readers in the map")
	assert.False(t, receiver.lastDiscoveryTime.IsZero(), "Expected lastDiscoveryTime to be set")
}

func TestScrapeDynamicInstances_Lifecycle(t *testing.T) {
	logger := zap.NewNop()
	cfg := createDynamicConfig()
	cfg.DiscoveryInterval = 5 * time.Minute // Set a long discovery interval
	receiver := newGoogleCloudSpannerReceiver(logger, cfg)
	receiver.metricsBuilder = newMetricsBuilder(false)

	// Mock initial state after initialize would have run.
	initialReader := &mockCompositeReader{}
	receiver.databaseReaders["project-dynamic/instance-dynamic/db-initial"] = initialReader
	receiver.lastDiscoveryTime = time.Now()

	// --- Scrape 1: Before discovery interval has passed ---
	t.Run("Scrape before interval", func(t *testing.T) {
		listCalled := false
		receiver.listDatabasesFunc = func(ctx context.Context, projectID, instanceID, serviceAccountKey string) ([]string, error) {
			listCalled = true
			return nil, nil
		}
		initialReader.On("Read", mock.Anything).Return([]*metadata.MetricsDataPoint{}, nil).Once()

		_, err := receiver.Scrape(context.Background())
		require.NoError(t, err)

		assert.False(t, listCalled, "ListDatabases should not be called before interval")
		initialReader.AssertExpectations(t)
	})

	// --- Scrape 2: After discovery interval has passed ---
	t.Run("Scrape after interval", func(t *testing.T) {
		// Set the last discovery time to the past so the interval is exceeded.
		receiver.lastDiscoveryTime = time.Now().Add(-10 * time.Minute)

		// Mock the API to return a new list of databases
		receiver.listDatabasesFunc = func(ctx context.Context, projectID, instanceID, serviceAccountKey string) ([]string, error) {
			return []string{"db-retained", "db-new"}, nil
		}

		// Mock reader creation for the new database
		newReader := &mockCompositeReader{}
		receiver.newDbReaderFunc = func(ctx context.Context, md []*metadata.MetricsMetadata, id *datasource.DatabaseID, sap string, rc statsreader.ReaderConfig, l *zap.Logger) (statsreader.CompositeReader, error) {
			if id.DatabaseName() == "db-new" {
				return newReader, nil
			}
			return nil, fmt.Errorf("unexpected creation of reader for %s", id.DatabaseName())
		}

		// The original 'db-initial' reader is now stale and should be shut down.
		retainedReader := &mockCompositeReader{}
		receiver.databaseReaders["project-dynamic/instance-dynamic/db-retained"] = retainedReader
		staleReader := receiver.databaseReaders["project-dynamic/instance-dynamic/db-initial"].(*mockCompositeReader)
		staleReader.On("Shutdown").Return().Once()

		// The 'db-retained' reader should not be re-created but its Read should be called.
		retainedReader.On("Read", mock.Anything).Return([]*metadata.MetricsDataPoint{}, nil).Once()

		// The new reader for 'db-new' should be read from.
		newReader.On("Read", mock.Anything).Return([]*metadata.MetricsDataPoint{}, nil).Once()

		_, err := receiver.Scrape(context.Background())
		require.NoError(t, err)

		// Verify the final state of the readers map
		assert.Len(t, receiver.databaseReaders, 2)
		assert.NotContains(t, receiver.databaseReaders, "project-dynamic/instance-dynamic/db-initial")
		assert.Contains(t, receiver.databaseReaders, "project-dynamic/instance-dynamic/db-retained")
		assert.Contains(t, receiver.databaseReaders, "project-dynamic/instance-dynamic/db-new")

		staleReader.AssertExpectations(t)
		retainedReader.AssertExpectations(t)
		newReader.AssertExpectations(t)
	})
}
