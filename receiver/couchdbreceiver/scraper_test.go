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

package couchdbreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/couchdbreceiver"

import (
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/scrapertest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/scrapertest/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/couchdbreceiver/internal/metadata"
)

func TestScrape(t *testing.T) {
	f := NewFactory()
	cfg := f.CreateDefaultConfig().(*Config)
	cfg.Username = "otelu"
	cfg.Password = "otelp"
	require.NoError(t, cfg.Validate())

	t.Run("scrape from couchdb version 2.31", func(t *testing.T) {
		mockClient := new(MockClient)
		mockClient.On("GetStats", "_local").Return(getStats("response_2.31.json"))
		scraper := newCouchdbScraper(componenttest.NewNopReceiverCreateSettings(), cfg)
		scraper.client = mockClient

		actualMetrics, err := scraper.scrape(context.Background())
		require.NoError(t, err)

		expectedFile := filepath.Join("testdata", "scraper", "expected.json")
		expectedMetrics, err := golden.ReadMetrics(expectedFile)
		require.NoError(t, err)

		require.NoError(t, scrapertest.CompareMetrics(expectedMetrics, actualMetrics))
	})

	t.Run("scrape from couchdb 3.12", func(t *testing.T) {
		mockClient := new(MockClient)
		mockClient.On("GetStats", "_local").Return(getStats("response_3.12.json"))
		scraper := newCouchdbScraper(componenttest.NewNopReceiverCreateSettings(), cfg)
		scraper.client = mockClient

		actualMetrics, err := scraper.scrape(context.Background())
		require.NoError(t, err)

		expectedFile := filepath.Join("testdata", "scraper", "expected.json")
		expectedMetrics, err := golden.ReadMetrics(expectedFile)
		require.NoError(t, err)

		require.NoError(t, scrapertest.CompareMetrics(expectedMetrics, actualMetrics))
	})

	t.Run("scrape error: failed to connect to client", func(t *testing.T) {
		scraper := newCouchdbScraper(componenttest.NewNopReceiverCreateSettings(), cfg)

		_, err := scraper.scrape(context.Background())
		require.NotNil(t, err)
		require.Equal(t, err, errors.New("no client available"))
	})

	t.Run("scrape error: get stats endpoint error", func(t *testing.T) {
		obs, logs := observer.New(zap.ErrorLevel)
		settings := componenttest.NewNopReceiverCreateSettings()
		settings.Logger = zap.New(obs)
		mockClient := new(MockClient)
		mockClient.On("GetStats", "_local").Return(getStats(""))
		scraper := newCouchdbScraper(settings, cfg)
		scraper.client = mockClient

		_, err := scraper.scrape(context.Background())
		require.NotNil(t, err)
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
		require.NoError(t, cfg.Validate())

		scraper := newCouchdbScraper(componenttest.NewNopReceiverCreateSettings(), cfg)
		err := scraper.start(context.Background(), componenttest.NewNopHost())
		require.NoError(t, err)
	})
	t.Run("start fail", func(t *testing.T) {
		f := NewFactory()
		cfg := f.CreateDefaultConfig().(*Config)
		cfg.HTTPClientSettings.TLSSetting.CAFile = "/non/existent"
		cfg.Username = "otelu"
		cfg.Password = "otelp"
		require.NoError(t, cfg.Validate())

		scraper := newCouchdbScraper(componenttest.NewNopReceiverCreateSettings(), cfg)
		err := scraper.start(context.Background(), componenttest.NewNopHost())
		require.NotNil(t, err)
	})
}

func TestMetricSettings(t *testing.T) {
	mockClient := new(MockClient)
	mockClient.On("GetStats", "_local").Return(getStats("response_2.31.json"))
	cfg := &Config{
		HTTPClientSettings: confighttp.HTTPClientSettings{},
		Metrics: metadata.MetricsSettings{
			CouchdbAverageRequestTime: metadata.MetricSettings{Enabled: false},
			CouchdbDatabaseOpen:       metadata.MetricSettings{Enabled: false},
			CouchdbDatabaseOperations: metadata.MetricSettings{Enabled: true},
			CouchdbFileDescriptorOpen: metadata.MetricSettings{Enabled: false},
			CouchdbHttpdBulkRequests:  metadata.MetricSettings{Enabled: false},
			CouchdbHttpdRequests:      metadata.MetricSettings{Enabled: false},
			CouchdbHttpdResponses:     metadata.MetricSettings{Enabled: false},
			CouchdbHttpdViews:         metadata.MetricSettings{Enabled: false},
		},
	}
	scraper := newCouchdbScraper(componenttest.NewNopReceiverCreateSettings(), cfg)
	scraper.client = mockClient

	metrics, err := scraper.scrape(context.Background())
	require.NoError(t, err)

	expected, err := golden.ReadMetrics(filepath.Join("testdata", "scraper", "only_db_ops.json"))
	require.NoError(t, err)

	require.NoError(t, scrapertest.CompareMetrics(expected, metrics))
	require.Equal(t, metrics.MetricCount(), 1)
}

func getStats(filename string) (map[string]interface{}, error) {
	var stats map[string]interface{}

	if filename == "" {
		return nil, errors.New("bad response")
	}
	if filename == "empty" {
		_ = json.Unmarshal([]byte{}, &stats)
		return stats, nil
	}

	file, err := os.Open(path.Join("testdata", "scraper", filename))
	if err != nil {
		return nil, err
	}
	defer file.Close()

	body, err := ioutil.ReadAll(file)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(body, &stats)
	if err != nil {
		return nil, err
	}

	return stats, nil
}

// MockClient is an autogenerated mock type for the client type
type MockClient struct {
	mock.Mock
}

// Get provides a mock function with given fields: path
func (_m *MockClient) Get(path string) ([]byte, error) {
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
func (_m *MockClient) GetStats(nodeName string) (map[string]interface{}, error) {
	ret := _m.Called(nodeName)

	var r0 map[string]interface{}
	if rf, ok := ret.Get(0).(func(string) map[string]interface{}); ok {
		r0 = rf(nodeName)
	} else if ret.Get(0) != nil {
		r0 = ret.Get(0).(map[string]interface{})
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string) error); ok {
		r1 = rf(nodeName)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}
