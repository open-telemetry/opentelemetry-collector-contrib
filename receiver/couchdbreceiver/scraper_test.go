// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package couchdbreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/couchdbreceiver"

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/confmap/xconfmap"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.opentelemetry.io/collector/scraper/scrapererror"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/couchdbreceiver/internal/metadata"
)

func TestScrape(t *testing.T) {
	f := NewFactory()
	cfg := f.CreateDefaultConfig().(*Config)
	cfg.Username = "otelu"
	cfg.Password = "otelp"
	require.NoError(t, xconfmap.Validate(cfg))

	t.Run("scrape from couchdb version 2.31", func(t *testing.T) {
		mockClient := new(mockClient)
		mockClient.On("GetStats", "_local").Return(getStats("response_2.31.json"))
		scraper := newCouchdbScraper(receivertest.NewNopSettings(metadata.Type), cfg)
		scraper.client = mockClient

		actualMetrics, err := scraper.scrape(context.Background())
		require.NoError(t, err)

		expectedFile := filepath.Join("testdata", "scraper", "expected.yaml")
		expectedMetrics, err := golden.ReadMetrics(expectedFile)
		require.NoError(t, err)

		require.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, actualMetrics,
			pmetrictest.IgnoreMetricDataPointsOrder(), pmetrictest.IgnoreStartTimestamp(), pmetrictest.IgnoreTimestamp()))
	})

	t.Run("scrape from couchdb 3.12", func(t *testing.T) {
		mockClient := new(mockClient)
		mockClient.On("GetStats", "_local").Return(getStats("response_3.12.json"))
		scraper := newCouchdbScraper(receivertest.NewNopSettings(metadata.Type), cfg)
		scraper.client = mockClient

		actualMetrics, err := scraper.scrape(context.Background())
		require.NoError(t, err)

		expectedFile := filepath.Join("testdata", "scraper", "expected.yaml")
		expectedMetrics, err := golden.ReadMetrics(expectedFile)
		require.NoError(t, err)

		require.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, actualMetrics,
			pmetrictest.IgnoreMetricDataPointsOrder(), pmetrictest.IgnoreStartTimestamp(), pmetrictest.IgnoreTimestamp()))
	})

	t.Run("scrape returns nothing", func(t *testing.T) {
		mockClient := new(mockClient)
		mockClient.On("GetStats", "_local").Return(map[string]any{}, nil)
		scraper := newCouchdbScraper(receivertest.NewNopSettings(metadata.Type), cfg)
		scraper.client = mockClient

		metrics, err := scraper.scrape(context.Background())
		require.Error(t, err)
		assert.Equal(t, 0, metrics.DataPointCount(), "Expected 0 datapoints to be collected")

		var partialScrapeErr scrapererror.PartialScrapeError
		require.ErrorAs(t, err, &partialScrapeErr, "returned error was not PartialScrapeError")
		require.Positive(t, partialScrapeErr.Failed, "Expected scrape failures, but none were recorded!")
	})

	t.Run("scrape error: failed to connect to client", func(t *testing.T) {
		scraper := newCouchdbScraper(receivertest.NewNopSettings(metadata.Type), cfg)

		_, err := scraper.scrape(context.Background())
		require.Error(t, err)
		require.Equal(t, err, errors.New("no client available"))
	})

	t.Run("scrape error: get stats endpoint error", func(t *testing.T) {
		obs, logs := observer.New(zap.ErrorLevel)
		settings := receivertest.NewNopSettings(metadata.Type)
		settings.Logger = zap.New(obs)
		mockClient := new(mockClient)
		mockClient.On("GetStats", "_local").Return(getStats(""))
		scraper := newCouchdbScraper(settings, cfg)
		scraper.client = mockClient

		_, err := scraper.scrape(context.Background())
		require.Error(t, err)
		require.Equal(t, 1, logs.Len())
		require.Equal(t, []observer.LoggedEntry{
			{
				Entry: zapcore.Entry{Level: zap.ErrorLevel, Message: "Failed to fetch couchdb stats"},
				Context: []zapcore.Field{
					zap.String("endpoint", cfg.Endpoint),
					zap.Error(errors.New("bad response")),
				},
			},
		}, logs.AllUntimed())
	})
}

func TestStart(t *testing.T) {
	t.Run("start success", func(t *testing.T) {
		f := NewFactory()
		cfg := f.CreateDefaultConfig().(*Config)
		cfg.Username = "otelu"
		cfg.Password = "otelp"
		require.NoError(t, xconfmap.Validate(cfg))

		scraper := newCouchdbScraper(receivertest.NewNopSettings(metadata.Type), cfg)
		err := scraper.start(context.Background(), componenttest.NewNopHost())
		require.NoError(t, err)
	})
	t.Run("start fail", func(t *testing.T) {
		f := NewFactory()
		cfg := f.CreateDefaultConfig().(*Config)
		cfg.TLS.CAFile = "/non/existent"
		cfg.Username = "otelu"
		cfg.Password = "otelp"
		require.NoError(t, xconfmap.Validate(cfg))

		scraper := newCouchdbScraper(receivertest.NewNopSettings(metadata.Type), cfg)
		err := scraper.start(context.Background(), componenttest.NewNopHost())
		require.Error(t, err)
	})
}

func TestMetricSettings(t *testing.T) {
	mockClient := new(mockClient)
	mockClient.On("GetStats", "_local").Return(getStats("response_2.31.json"))
	mbc := metadata.DefaultMetricsBuilderConfig()
	mbc.Metrics = metadata.MetricsConfig{
		CouchdbAverageRequestTime: metadata.MetricConfig{Enabled: false},
		CouchdbDatabaseOpen:       metadata.MetricConfig{Enabled: false},
		CouchdbDatabaseOperations: metadata.MetricConfig{Enabled: true},
		CouchdbFileDescriptorOpen: metadata.MetricConfig{Enabled: false},
		CouchdbHttpdBulkRequests:  metadata.MetricConfig{Enabled: false},
		CouchdbHttpdRequests:      metadata.MetricConfig{Enabled: false},
		CouchdbHttpdResponses:     metadata.MetricConfig{Enabled: false},
		CouchdbHttpdViews:         metadata.MetricConfig{Enabled: false},
	}
	cfg := &Config{
		ClientConfig:         confighttp.NewDefaultClientConfig(),
		MetricsBuilderConfig: mbc,
	}
	scraper := newCouchdbScraper(receivertest.NewNopSettings(metadata.Type), cfg)
	scraper.client = mockClient

	metrics, err := scraper.scrape(context.Background())
	require.NoError(t, err)

	expected, err := golden.ReadMetrics(filepath.Join("testdata", "scraper", "only_db_ops.yaml"))
	require.NoError(t, err)

	require.NoError(t, pmetrictest.CompareMetrics(expected, metrics, pmetrictest.IgnoreMetricDataPointsOrder(),
		pmetrictest.IgnoreStartTimestamp(), pmetrictest.IgnoreTimestamp()))
	require.Equal(t, 1, metrics.MetricCount())
}

func getStats(filename string) (map[string]any, error) {
	var stats map[string]any

	if filename == "" {
		return nil, errors.New("bad response")
	}
	if filename == "empty" {
		_ = json.Unmarshal([]byte{}, &stats)
		return stats, nil
	}

	body, err := os.ReadFile(path.Join("testdata", "scraper", filename))
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(body, &stats)
	if err != nil {
		return nil, err
	}

	return stats, nil
}

// mockClient is an autogenerated mock type for the client type
type mockClient struct {
	mock.Mock
}

// Get provides a mock function with given fields: path
func (_m *mockClient) Get(path string) ([]byte, error) {
	ret := _m.Called(path)

	var r0 []byte
	if rf, ok := ret.Get(0).(func(string) []byte); ok {
		r0 = rf(path)
	} else if ret.Get(0) != nil {
		r0 = ret.Get(0).([]byte)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string) error); ok {
		r1 = rf(path)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetStats provides a mock function with given fields: nodeName
func (_m *mockClient) GetStats(nodeName string) (map[string]any, error) {
	ret := _m.Called(nodeName)

	var r0 map[string]any
	if rf, ok := ret.Get(0).(func(string) map[string]any); ok {
		r0 = rf(nodeName)
	} else if ret.Get(0) != nil {
		r0 = ret.Get(0).(map[string]any)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string) error); ok {
		r1 = rf(nodeName)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}
