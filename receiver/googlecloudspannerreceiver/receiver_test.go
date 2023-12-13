// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlecloudspannerreceiver

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap/zaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver/internal/statsreader"
)

const (
	serviceAccountValidPath   = "testdata/serviceAccount.json"
	serviceAccountInvalidPath = "does not exist"
)

type mockCompositeReader struct {
	mock.Mock
}

func (r *mockCompositeReader) Name() string {
	return "mockCompositeReader"
}

func (r *mockCompositeReader) Read(ctx context.Context) ([]*metadata.MetricsDataPoint, error) {
	args := r.Called(ctx)
	return args.Get(0).([]*metadata.MetricsDataPoint), args.Error(1)
}

func (r *mockCompositeReader) Shutdown() {
	// Do nothing
}

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

func TestNewGoogleCloudSpannerReceiver(t *testing.T) {
	logger := zaptest.NewLogger(t)
	cfg := createDefaultConfig().(*Config)
	receiver := newGoogleCloudSpannerReceiver(logger, cfg)

	require.NotNil(t, receiver)

	assert.Equal(t, logger, receiver.logger)
	assert.Equal(t, cfg, receiver.config)
}

func createConfig(serviceAccountPath string) *Config {
	cfg := createDefaultConfig().(*Config)

	instance := Instance{
		ID:        "instanceID",
		Databases: []string{"databaseName"},
	}

	project := Project{
		ID:                "projectID",
		Instances:         []Instance{instance},
		ServiceAccountKey: serviceAccountPath,
	}

	cfg.Projects = []Project{project}

	return cfg
}

func TestStart(t *testing.T) {
	testCases := map[string]struct {
		serviceAccountPath string
		expectError        bool
	}{
		"Happy path": {serviceAccountValidPath, false},
		"With project readers initialization error": {serviceAccountInvalidPath, true},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			logger := zaptest.NewLogger(t)
			cfg := createConfig(testCase.serviceAccountPath)
			host := componenttest.NewNopHost()

			receiver := newGoogleCloudSpannerReceiver(logger, cfg)

			require.NotNil(t, receiver)

			err := receiver.Start(context.Background(), host)

			if testCase.expectError {
				require.Error(t, err)
				assert.Equal(t, 0, len(receiver.projectReaders))
			} else {
				require.NoError(t, err)
				assert.Equal(t, 1, len(receiver.projectReaders))
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
		"With metadata config error": {serviceAccountInvalidPath, true, true},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			logger := zaptest.NewLogger(t)
			cfg := createConfig(testCase.serviceAccountPath)

			receiver := newGoogleCloudSpannerReceiver(logger, cfg)

			require.NotNil(t, receiver)

			yaml := metadataYaml

			if testCase.replaceMetadataConfig {
				metadataYaml = []byte{1}
			}

			err := receiver.initialize(context.Background())

			if testCase.replaceMetadataConfig {
				metadataYaml = yaml
			}

			if testCase.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestInitializeProjectReaders(t *testing.T) {
	testCases := map[string]struct {
		serviceAccountPath string
		expectError        bool
	}{
		"Happy path":                 {serviceAccountValidPath, false},
		"With error":                 {serviceAccountInvalidPath, true},
		"With metadata config error": {serviceAccountInvalidPath, true},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			logger := zaptest.NewLogger(t)
			cfg := createConfig(testCase.serviceAccountPath)

			receiver := newGoogleCloudSpannerReceiver(logger, cfg)

			require.NotNil(t, receiver)

			err := receiver.initializeProjectReaders(context.Background(), []*metadata.MetricsMetadata{})

			if testCase.expectError {
				require.Error(t, err)
				assert.Equal(t, 0, len(receiver.projectReaders))
			} else {
				require.NoError(t, err)
				assert.Equal(t, 1, len(receiver.projectReaders))
			}
		})
	}
}

func TestInitializeMetricsBuilder(t *testing.T) {
	logger := zaptest.NewLogger(t)
	cfg := createConfig(serviceAccountValidPath)
	testCases := map[string]struct {
		metadataItems []*metadata.MetricsMetadata
		expectError   bool
	}{
		"Happy path": {[]*metadata.MetricsMetadata{{}}, false},
		"With error": {[]*metadata.MetricsMetadata{}, true},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			receiver := newGoogleCloudSpannerReceiver(logger, cfg)

			require.NotNil(t, receiver)

			err := receiver.initializeMetricsBuilder(testCase.metadataItems)

			if testCase.expectError {
				require.Error(t, err)
				require.Nil(t, receiver.metricsBuilder)
			} else {
				require.NoError(t, err)
				require.NotNil(t, receiver.metricsBuilder)
			}
		})
	}
}

func TestNewProjectReader(t *testing.T) {
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
			cfg := createConfig(testCase.serviceAccountPath)
			var parsedMetadata []*metadata.MetricsMetadata

			reader, err := newProjectReader(context.Background(), logger, cfg.Projects[0], parsedMetadata,
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

func TestScrape(t *testing.T) {
	logger := zaptest.NewLogger(t)
	testCases := map[string]struct {
		metricsBuilder metadata.MetricsBuilder
		expectedError  error
	}{
		"Happy path": {newMetricsBuilder(false), nil},
		"With error": {newMetricsBuilder(true), errors.New("error")},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			cfg := createDefaultConfig().(*Config)
			receiver := newGoogleCloudSpannerReceiver(logger, cfg)
			mcr := &mockCompositeReader{}

			require.NotNil(t, receiver)

			receiver.projectReaders = []statsreader.CompositeReader{mcr}
			receiver.metricsBuilder = testCase.metricsBuilder
			ctx := context.Background()

			mcr.On("Read", ctx).Return([]*metadata.MetricsDataPoint{}, testCase.expectedError)

			_, err := receiver.Scrape(ctx)

			mcr.AssertExpectations(t)
			assert.Equal(t, testCase.expectedError, err)
		})
	}
}

func TestGoogleCloudSpannerReceiver_Shutdown(t *testing.T) {
	logger := zaptest.NewLogger(t)
	projectReader := statsreader.NewProjectReader([]statsreader.CompositeReader{}, logger)

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
				projectReaders: []statsreader.CompositeReader{projectReader},
				metricsBuilder: testCase.metricsBuilder,
			}
			ctx := context.Background()
			ctx, receiver.cancel = context.WithCancel(ctx)

			err := receiver.Shutdown(ctx)

			if testCase.expectError {
				require.NotNil(t, err)
			} else {
				require.Nil(t, err)
			}
		})
	}
}
