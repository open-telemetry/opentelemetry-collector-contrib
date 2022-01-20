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
	"time"

	"github.com/hashicorp/go-version"
	"go.mongodb.org/mongo-driver/bson"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/model/pdata"
	"go.opentelemetry.io/collector/receiver/scrapererror"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbreceiver/internal/metadata"
)

const instrumentationLibraryName = "otelcol/mongodb"

type mongodbScraper struct {
	logger       *zap.Logger
	config       *Config
	client       client
	mongoVersion *version.Version
	mb           *metadata.MetricsBuilder
}

func newMongodbScraper(logger *zap.Logger, config *Config) *mongodbScraper {
	return &mongodbScraper{
		logger: logger,
		config: config,
		mb:     metadata.NewMetricsBuilder(config.Metrics),
	}
}

func (s *mongodbScraper) start(ctx context.Context, _ component.Host) error {
	c, err := NewClient(ctx, s.config, s.logger)
	if err != nil {
		return fmt.Errorf("create mongo client: %w", err)
	}
	s.client = c

	version, err := s.client.GetVersion(ctx)
	if err != nil {
		s.logger.Error("failed to get version at start", zap.Error(err))
		// component should not fail to start if it cannot get the version
		return nil
	}
	s.logger.Debug(fmt.Sprintf("detected mongo server to be running version: %s", version.String()))
	s.mongoVersion = version
	return nil
}

func (s *mongodbScraper) shutdown(ctx context.Context) error {
	if s.client != nil {
		return s.client.Disconnect(ctx)
	}
	return nil
}

func (s *mongodbScraper) scrape(ctx context.Context) (pdata.Metrics, error) {
	metrics := pdata.NewMetrics()
	rms := metrics.ResourceMetrics()

	if s.client == nil {
		return metrics, errors.New("no client was initialized before calling scrape")
	}

	if s.mongoVersion == nil {
		version, err := s.client.GetVersion(ctx)
		if err != nil {
			return pdata.NewMetrics(), fmt.Errorf("unable to determine version of mongo scraping against: %w", err)
		}
		s.mongoVersion = version
	}

	var errors scrapererror.ScrapeErrors
	s.collectMetrics(ctx, rms, errors)
	return metrics, errors.Combine()
}

func (s *mongodbScraper) collectMetrics(ctx context.Context, rms pdata.ResourceMetricsSlice, errors scrapererror.ScrapeErrors) {
	dbNames, err := s.client.ListDatabaseNames(ctx, bson.D{})
	if err != nil {
		s.logger.Error("Failed to fetch database names", zap.Error(err))
		return
	}

	now := pdata.NewTimestampFromTime(time.Now())

	s.collectAdminDatabase(ctx, rms, now, errors)
	for _, dbName := range dbNames {
		s.collectDatabase(ctx, rms, now, dbName, errors)
	}
}

func (s *mongodbScraper) collectDatabase(ctx context.Context, rms pdata.ResourceMetricsSlice, now pdata.Timestamp, databaseName string, errors scrapererror.ScrapeErrors) {
	rm := rms.AppendEmpty()
	resourceAttrs := rm.Resource().Attributes()
	resourceAttrs.InsertString(metadata.A.Database, databaseName)

	ilms := rm.InstrumentationLibraryMetrics().AppendEmpty()
	ilms.InstrumentationLibrary().SetName(instrumentationLibraryName)

	dbStats, err := s.client.DBStats(ctx, databaseName)
	if err != nil {
		errors.AddPartial(1, err)
	} else {
		s.recordDBStats(now, dbStats, databaseName, errors)
	}

	serverStatus, err := s.client.ServerStatus(ctx, databaseName)
	if err != nil {
		errors.AddPartial(1, err)
		return
	}
	s.recordNormalServerStats(now, serverStatus, databaseName, errors)

	s.mb.EmitDatabase(ilms.Metrics())
}

func (s *mongodbScraper) collectAdminDatabase(ctx context.Context, rms pdata.ResourceMetricsSlice, now pdata.Timestamp, errors scrapererror.ScrapeErrors) {
	rm := rms.AppendEmpty()
	ilms := rm.InstrumentationLibraryMetrics().AppendEmpty()
	ilms.InstrumentationLibrary().SetName(instrumentationLibraryName)

	serverStatus, err := s.client.ServerStatus(ctx, "admin")
	if err != nil {
		errors.AddPartial(1, err)
		return
	}
	s.recordAdminStats(now, serverStatus, errors)
	s.mb.EmitAdmin(ilms.Metrics())
}

func (s *mongodbScraper) recordDBStats(now pdata.Timestamp, doc bson.M, dbName string, errors scrapererror.ScrapeErrors) {
	s.recordCollections(now, doc, dbName, errors)
	s.recordDataSize(now, doc, dbName, errors)
	s.recordExtentCount(now, doc, dbName, errors)
	s.recordIndexSize(now, doc, dbName, errors)
	s.recordIndexCount(now, doc, dbName, errors)
	s.recordObjectCount(now, doc, dbName, errors)
	s.recordStorageSize(now, doc, dbName, errors)
}

func (s *mongodbScraper) recordNormalServerStats(now pdata.Timestamp, doc bson.M, dbName string, errors scrapererror.ScrapeErrors) {
	s.recordConnections(now, doc, dbName, errors)
	s.recordMemoryUsage(now, doc, dbName, errors)
}

func (s *mongodbScraper) recordAdminStats(now pdata.Timestamp, document bson.M, errors scrapererror.ScrapeErrors) {
	s.recordGlobalLockTime(now, document, errors)
	s.recordCacheOperations(now, document, errors)
	s.recordOperations(now, document, errors)
}
