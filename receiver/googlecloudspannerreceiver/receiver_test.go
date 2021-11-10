// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package googlecloudspannerreceiver

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.uber.org/zap/zaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver/internal/statsreader"
)

const (
	serviceAccountValidPath   = "testdata/serviceAccount.json"
	serviceAccountInvalidPath = "does not exist"
)

type mockErrorCompositeReader struct {
}

func (r mockErrorCompositeReader) Name() string {
	return "mockErrorCompositeReader"
}

func (r mockErrorCompositeReader) Read(_ context.Context) ([]*metadata.MetricsDataPoint, error) {
	return nil, errors.New("error")
}

func (r mockErrorCompositeReader) Shutdown() {
	// Do nothing
}

func TestNewGoogleCloudSpannerReceiver(t *testing.T) {
	logger := zaptest.NewLogger(t)
	cfg := createDefaultConfig().(*Config)
	receiver := newGoogleCloudSpannerReceiver(logger, cfg)

	require.NotNil(t, receiver)

	assert.Equal(t, logger, receiver.logger)
	assert.Equal(t, cfg, receiver.config)
	require.NotNil(t, receiver.metricsBuilder)
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

func TestInitializeProjectReaders(t *testing.T) {
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

			err := receiver.initializeProjectReaders(context.Background())

			if testCase.replaceMetadataConfig {
				metadataYaml = yaml
			}

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

func TestCollectData(t *testing.T) {
	logger := zaptest.NewLogger(t)

	testCases := map[string]struct {
		nextConsumer  consumer.Metrics
		projectReader statsreader.CompositeReader
		expectError   bool
	}{
		"Happy path": {consumertest.NewNop(), statsreader.NewProjectReader([]statsreader.CompositeReader{}, logger), false},
		"With error": {consumertest.NewErr(errors.New("an error")), mockErrorCompositeReader{}, true},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			cfg := createDefaultConfig().(*Config)
			receiver := newGoogleCloudSpannerReceiver(logger, cfg)

			require.NotNil(t, receiver)

			receiver.projectReaders = []statsreader.CompositeReader{testCase.projectReader}
			ctx := context.Background()

			_, err := receiver.Scrape(ctx)

			if testCase.expectError {
				require.NotNil(t, err)
			} else {
				require.Nil(t, err)
			}
		})
	}

}

func TestGoogleCloudSpannerReceiver_Shutdown(t *testing.T) {
	logger := zaptest.NewLogger(t)
	projectReader := statsreader.NewProjectReader([]statsreader.CompositeReader{}, logger)

	receiver := &googleCloudSpannerReceiver{
		projectReaders: []statsreader.CompositeReader{projectReader},
	}

	ctx := context.Background()
	ctx, receiver.cancel = context.WithCancel(ctx)

	err := receiver.Shutdown(ctx)

	require.NoError(t, err)
}
