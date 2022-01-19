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

package mongodbreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbreceiver"

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"go.mongodb.org/mongo-driver/bson"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/model/pdata"
	"go.opentelemetry.io/collector/receiver/scrapererror"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbreceiver/internal/metadata"
)

const instrumentationLibraryName = "otelcol/mongodb"

type mongodbScraper struct {
	logger    *zap.Logger
	config    *Config
	client    client
	extractor *extractor
	mb        *metadata.MetricsBuilder
}

func newMongodbScraper(logger *zap.Logger, config *Config) *mongodbScraper {
	return &mongodbScraper{
		logger: logger,
		config: config,
		mb:     metadata.NewMetricsBuilder(config.Metrics),
	}
}

func (s *mongodbScraper) start(ctx context.Context, _ component.Host) error {
	clientLogger := s.logger.Named("mongo-scraper")
	c := NewClient(s.config, clientLogger)
	s.client = c

	if err := s.client.Connect(ctx); err != nil {
		s.logger.Error("unable to connect to mongo instance", zap.Error(err))
	}

	vr, err := s.client.GetVersion(ctx)
	if err != nil {
		return fmt.Errorf("unable to get a version from the mongo instance: %w", err)
	}

	s.extractor, err = newExtractor(*vr, s.logger)
	if err != nil {
		return err
	}

	return nil
}

func (s *mongodbScraper) shutdown(ctx context.Context) error {
	if s.client != nil {
		return s.client.Disconnect(ctx)
	}
	return nil
}

func (s *mongodbScraper) scrape(ctx context.Context) (pdata.Metrics, error) {
	s.logger.Debug("starting otelcol/mongodb scrape")
	if s.client == nil {
		return pdata.NewMetrics(), errors.New("no client was initialized before calling scrape")
	}

	metrics := pdata.NewMetrics()
	rms := metrics.ResourceMetrics().AppendEmpty()
	var errors scrapererror.ScrapeErrors

	s.collectMetrics(ctx, rms, errors)

	return metrics, errors.Combine()
}

func (s *mongodbScraper) collectMetrics(ctx context.Context, rms pdata.ResourceMetrics, errors scrapererror.ScrapeErrors) {
	ilm := rms.InstrumentationLibraryMetrics().AppendEmpty()
	ilm.InstrumentationLibrary().SetName(instrumentationLibraryName)

	dbNames, err := s.client.ListDatabaseNames(ctx, bson.D{})
	if err != nil {
		s.logger.Error("Failed to fetch database names", zap.Error(err))
		return
	}

	wg := &sync.WaitGroup{}

	wg.Add(1)
	go s.collectAdminDatabase(ctx, wg, errors)

	for _, dbName := range dbNames {
		wg.Add(1)
		go s.collectDatabase(ctx, wg, dbName, errors)
	}

	wg.Wait()

	s.mb.EmitCollection(ilm.Metrics())
}

func (s *mongodbScraper) collectDatabase(ctx context.Context, wg *sync.WaitGroup, databaseName string, errors scrapererror.ScrapeErrors) {
	defer wg.Done()
	dbStats, err := s.client.DBStats(ctx, databaseName)
	if err != nil {
		errors.AddPartial(1, err)
	} else {
		s.extractor.Extract(dbStats, s.mb, databaseName, normalDBStats)
	}

	serverStatus, err := s.client.ServerStatus(ctx, databaseName)
	if err != nil {
		errors.AddPartial(1, err)
	} else {
		s.extractor.Extract(serverStatus, s.mb, databaseName, normalServerStats)
	}
}

func (s *mongodbScraper) collectAdminDatabase(ctx context.Context, wg *sync.WaitGroup, errors scrapererror.ScrapeErrors) {
	defer wg.Done()
	serverStatus, err := s.client.ServerStatus(ctx, "admin")
	if err != nil {
		errors.AddPartial(1, err)
	} else {
		s.extractor.Extract(serverStatus, s.mb, "admin", adminServerStats)
	}
}
