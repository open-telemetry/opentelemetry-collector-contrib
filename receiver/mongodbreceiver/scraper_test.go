// Copyright The OpenTelemetry Authors
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

package mongodbreceiver

import (
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/config/configtls"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/scrapertest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/scrapertest/golden"
)

func TestScrape(t *testing.T) {
	f := NewFactory()
	cfg := f.CreateDefaultConfig().(*Config)
	cfg.Hosts = []confignet.TCPAddr{
		{
			Endpoint: net.JoinHostPort("localhost", "37017"),
		},
	}
	cfg.Username = "otel"
	cfg.Password = "otel"
	databaseList := []string{"fakedatabase"}

	client := &fakeDriver{}
	client.On("Connect", mock.Anything).Return(nil)
	client.On("Disconnect", mock.Anything).Return(nil)
	client.On("Ping", mock.Anything, mock.Anything).Return(nil)
	client.On("ListDatabaseNames", mock.Anything, mock.Anything, mock.Anything).Return(databaseList, nil)
	client.On("Query", mock.Anything, "admin", mock.Anything).Return(loadAdminData())
	client.On("Query", mock.Anything, mock.Anything, bson.M{"serverStatus": 1}).Return(loadServerStatus())
	client.On("Query", mock.Anything, mock.Anything, bson.M{"dbStats": 1}).Return(loadDBStats())

	extractor, err := newExtractor("4.0", zap.NewNop())
	require.NoError(t, err)

	scraper := mongodbScraper{
		logger:    zap.NewNop(),
		config:    cfg,
		client:    client,
		extractor: extractor,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	expectedFile := filepath.Join("testdata", "scraper", "expected.json")

	actual, err := scraper.scrape(context.Background())
	require.NoError(t, err)
	actualMetrics := actual.ResourceMetrics().At(0).InstrumentationLibraryMetrics().At(0).Metrics()

	expected, err := golden.ReadMetrics(expectedFile)
	require.NoError(t, err)
	expectedMetrics := expected.ResourceMetrics().At(0).InstrumentationLibraryMetrics().At(0).Metrics()

	err = scrapertest.CompareMetricSlices(actualMetrics, expectedMetrics)
	require.NoError(t, err)

	err = scraper.shutdown(ctx)
	require.NoError(t, err)

	client.AssertExpectations(t)
}

func TestCreateNewScraper(t *testing.T) {
	f := NewFactory()
	cfg := f.CreateDefaultConfig().(*Config)
	cfg.Hosts = []confignet.TCPAddr{
		{
			Endpoint: net.JoinHostPort("localhost", "37017"),
		},
	}
	cfg.Username = "otel"
	cfg.Password = "otel"
	cfg.TLSClientSetting = configtls.TLSClientSetting{
		InsecureSkipVerify: true,
	}

	client := &fakeDriver{}
	fakeResponse := &MongoTestConnectionResponse{Version: Mongo50.String()}
	client.On("TestConnection", mock.Anything).Return(fakeResponse, nil)

	scraper := mongodbScraper{
		config: cfg,
		client: client,
		logger: zap.NewNop(),
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := scraper.start(ctx, nil)
	require.NoError(t, err)

	require.True(t, scraper.config.TLSClientSetting.InsecureSkipVerify)
}

func TestScraperNoClient(t *testing.T) {
	f := NewFactory()
	cfg := f.CreateDefaultConfig().(*Config)
	cfg.Hosts = []confignet.TCPAddr{
		{
			Endpoint: net.JoinHostPort("localhost", "37017"),
		},
	}
	cfg.Username = "otel"
	cfg.Password = "otel"

	scraper := mongodbScraper{
		logger: zap.NewNop(),
		config: cfg,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, err := scraper.scrape(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "no client was initialized before calling scrape")

	err = scraper.shutdown(ctx)
	require.NoError(t, err)
}

func TestScrapeClientConnectionFailure(t *testing.T) {
	f := NewFactory()
	cfg := f.CreateDefaultConfig().(*Config)
	cfg.Hosts = []confignet.TCPAddr{
		{
			Endpoint: net.JoinHostPort("localhost", "37017"),
		},
	}
	cfg.Username = "otel"
	cfg.Password = "otel"

	fakeMongoClient := &fakeDriver{}
	fakeMongoClient.On("Connect", mock.Anything).Return(fmt.Errorf("Could not connect"))

	scraper := &mongodbScraper{
		config: cfg,
		logger: zap.NewNop(),
		client: fakeMongoClient,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	resources, err := scraper.scrape(ctx)
	require.Error(t, err)
	require.Equal(t, resources.MetricCount(), 0)
	fakeMongoClient.AssertExpectations(t)
}

func TestScrapeClientPingFailure(t *testing.T) {
	f := NewFactory()
	cfg := f.CreateDefaultConfig().(*Config)
	cfg.Hosts = []confignet.TCPAddr{
		{
			Endpoint: net.JoinHostPort("localhost", "37017"),
		},
	}
	cfg.Username = "otel"
	cfg.Password = "otel"

	fakeMongoClient := &fakeDriver{}
	fakeMongoClient.On("Connect", mock.Anything).Return(nil)
	fakeMongoClient.On("Ping", mock.Anything, mock.Anything).Return(fmt.Errorf("could not ping"))

	scraper := &mongodbScraper{
		config: cfg,
		logger: zap.NewNop(),
		client: fakeMongoClient,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	resources, err := scraper.scrape(ctx)
	require.Error(t, err)
	require.Equal(t, resources.MetricCount(), 0)
	fakeMongoClient.AssertExpectations(t)
}

func TestStart(t *testing.T) {
	f := NewFactory()
	cfg := f.CreateDefaultConfig().(*Config)
	cfg.Hosts = []confignet.TCPAddr{
		{
			Endpoint: net.JoinHostPort("localhost", "37017"),
		},
	}
	cfg.Username = "otel"
	cfg.Password = "otel"
}

func BenchmarkScrape(b *testing.B) {
	f := NewFactory()
	cfg := f.CreateDefaultConfig().(*Config)
	cfg.Hosts = []confignet.TCPAddr{
		{
			Endpoint: net.JoinHostPort("localhost", "37017"),
		},
	}
	cfg.Username = "otel"
	cfg.Password = "otel"

	databaseList := []string{"test-database"}

	client := &fakeDriver{}
	client.On("Connect", mock.Anything).Return(nil)
	client.On("Disconnect", mock.Anything).Return(nil)
	client.On("Ping", mock.Anything, mock.Anything).Return(nil)
	client.On("ListDatabaseNames", mock.Anything, mock.Anything, mock.Anything).Return(databaseList, nil)
	client.On("Query", mock.Anything, "admin", mock.Anything).Return(loadAdminData())
	client.On("Query", mock.Anything, mock.Anything, bson.M{"serverStatus": 1}).Return(loadServerStatus())
	client.On("Query", mock.Anything, mock.Anything, bson.M{"dbStats": 1}).Return(loadDBStats())

	scraper := mongodbScraper{
		logger: zap.NewNop(),
		config: cfg,
		client: client,
		extractor: &extractor{
			version: Mongo42,
			logger:  zap.NewNop(),
		},
	}

	for n := 0; n < b.N; n++ {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		_, _ = scraper.scrape(ctx)
	}
}

func loadAdminData() (bson.M, error) {
	return loadTestFile("./testdata/admin.json")
}

func loadDBStats() (bson.M, error) {
	var doc bson.M
	dbStats, err := ioutil.ReadFile("./testdata/dbstats.json")
	if err != nil {
		return nil, err
	}
	err = bson.UnmarshalExtJSON(dbStats, true, &doc)
	if err != nil {
		return nil, err
	}
	return doc, nil
}

func loadServerStatus() (bson.M, error) {
	return loadTestFile("./testdata/serverstatus.json")
}

func loadTestFile(filePath string) (bson.M, error) {
	var doc bson.M
	testFile, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	err = bson.UnmarshalExtJSON(testFile, true, &doc)
	if err != nil {
		return nil, err
	}
	return doc, nil
}
