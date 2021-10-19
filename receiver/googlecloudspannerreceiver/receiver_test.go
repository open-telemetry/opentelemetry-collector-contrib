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
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver/internal/statsreader"
)

const (
	serviceAccountValidPath   = "testdata/serviceAccount.json"
	serviceAccountInvalidPath = "does not exist"
)

type mockCompositeReader struct {
}

func (r mockCompositeReader) Name() string {
	return "mockCompositeReader"
}

func (r mockCompositeReader) Read(_ context.Context) []pdata.Metrics {
	md := pdata.NewMetrics()
	rms := md.ResourceMetrics()
	rm := rms.AppendEmpty()

	ilms := rm.InstrumentationLibraryMetrics()
	ilm := ilms.AppendEmpty()
	metric := ilm.Metrics().AppendEmpty()
	metric.SetName("testMetric")

	return []pdata.Metrics{md}
}

func (r mockCompositeReader) Shutdown() {
	// Do nothing
}

func TestNewGoogleCloudSpannerReceiver(t *testing.T) {
	logger := zap.NewNop()
	nextConsumer := consumertest.NewNop()
	cfg := createDefaultConfig().(*Config)

	receiver, err := newGoogleCloudSpannerReceiver(logger, cfg, nextConsumer, componenttest.NewNopReceiverCreateSettings())
	receiverCasted := receiver.(*googleCloudSpannerReceiver)

	require.NoError(t, err)
	require.NotNil(t, receiver)

	assert.Equal(t, logger, receiverCasted.logger)
	assert.Equal(t, nextConsumer, receiverCasted.nextConsumer)
	assert.Equal(t, cfg, receiverCasted.config)
}

func TestNewGoogleCloudSpannerReceiver_NilConsumer(t *testing.T) {
	cfg := createDefaultConfig().(*Config)

	metricsReceiver, err := newGoogleCloudSpannerReceiver(zap.NewNop(), cfg, nil, componenttest.NewNopReceiverCreateSettings())

	require.NotNil(t, err)
	require.Nil(t, metricsReceiver)
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
			logger := zap.NewNop()
			nextConsumer := consumertest.NewNop()
			cfg := createConfig(testCase.serviceAccountPath)

			host := componenttest.NewNopHost()

			receiver, err := newGoogleCloudSpannerReceiver(logger, cfg, nextConsumer, componenttest.NewNopReceiverCreateSettings())
			receiverCasted := receiver.(*googleCloudSpannerReceiver)

			require.NoError(t, err)
			require.NotNil(t, receiver)

			err = receiverCasted.Start(context.Background(), host)

			if testCase.expectError {
				require.Error(t, err)
				assert.Equal(t, 0, len(receiverCasted.projectReaders))
			} else {
				require.NoError(t, err)
				assert.Equal(t, 1, len(receiverCasted.projectReaders))
			}
		})
	}
}

func TestStartInitializesDataCollectionWithCollectDataError(t *testing.T) {
	logger := zap.NewNop()
	nextConsumer := consumertest.NewErr(errors.New("error"))
	cfg := createConfig(serviceAccountValidPath)
	cfg.CollectionInterval = 10 * time.Millisecond
	host := componenttest.NewNopHost()
	receiver, err := newGoogleCloudSpannerReceiver(logger, cfg, nextConsumer, componenttest.NewNopReceiverCreateSettings())
	receiverCasted := receiver.(*googleCloudSpannerReceiver)

	require.NoError(t, err)
	require.NotNil(t, receiver)

	errs := make(chan error)

	receiverCasted.onCollectData = append(receiverCasted.onCollectData, func(err error) {
		errs <- err
	})

	err = receiverCasted.Start(context.Background(), host)

	assert.NoError(t, <-errs)

	require.NoError(t, err)
}

func TestStartWithContextDone(t *testing.T) {
	logger := zap.NewNop()
	nextConsumer := consumertest.NewNop()
	cfg := createConfig(serviceAccountValidPath)
	ctx := context.Background()
	host := componenttest.NewNopHost()

	receiver, err := newGoogleCloudSpannerReceiver(logger, cfg, nextConsumer, componenttest.NewNopReceiverCreateSettings())
	receiverCasted := receiver.(*googleCloudSpannerReceiver)

	require.NoError(t, err)
	require.NotNil(t, receiver)

	err = receiverCasted.Start(ctx, host)

	require.NoError(t, err)
	receiverCasted.cancel()
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
			logger := zap.NewNop()
			nextConsumer := consumertest.NewNop()
			cfg := createConfig(testCase.serviceAccountPath)

			receiver, err := newGoogleCloudSpannerReceiver(logger, cfg, nextConsumer, componenttest.NewNopReceiverCreateSettings())
			receiverCasted := receiver.(*googleCloudSpannerReceiver)

			require.NoError(t, err)
			require.NotNil(t, receiver)

			yaml := metadataYaml

			if testCase.replaceMetadataConfig {
				metadataYaml = []byte{1}
			}

			err = receiverCasted.initializeProjectReaders(context.Background())

			if testCase.replaceMetadataConfig {
				metadataYaml = yaml
			}

			if testCase.expectError {
				require.Error(t, err)
				assert.Equal(t, 0, len(receiverCasted.projectReaders))
			} else {
				require.NoError(t, err)
				assert.Equal(t, 1, len(receiverCasted.projectReaders))
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
			logger := zap.NewNop()
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
	logger := zap.NewNop()

	testCases := map[string]struct {
		nextConsumer  consumer.Metrics
		projectReader statsreader.CompositeReader
		expectError   bool
	}{
		"Happy path": {consumertest.NewNop(), statsreader.NewProjectReader([]statsreader.CompositeReader{}, logger), false},
		"With error": {consumertest.NewErr(errors.New("an error")), mockCompositeReader{}, true},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			cfg := createDefaultConfig().(*Config)

			receiver, err := newGoogleCloudSpannerReceiver(logger, cfg, testCase.nextConsumer, componenttest.NewNopReceiverCreateSettings())

			require.NoError(t, err)
			require.NotNil(t, receiver)

			r := receiver.(*googleCloudSpannerReceiver)
			r.projectReaders = []statsreader.CompositeReader{testCase.projectReader}
			ctx := context.Background()

			err = r.collectData(ctx)

			if testCase.expectError {
				require.NotNil(t, err)
			} else {
				require.Nil(t, err)
			}
		})
	}

}

func TestGoogleCloudSpannerReceiver_Shutdown(t *testing.T) {
	logger := zap.NewNop()
	projectReader := statsreader.NewProjectReader([]statsreader.CompositeReader{}, logger)

	receiver := &googleCloudSpannerReceiver{
		projectReaders: []statsreader.CompositeReader{projectReader},
	}

	ctx := context.Background()
	ctx, receiver.cancel = context.WithCancel(ctx)

	err := receiver.Shutdown(ctx)

	require.NoError(t, err)
}
