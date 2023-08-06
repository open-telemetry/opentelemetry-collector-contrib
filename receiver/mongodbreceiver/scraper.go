// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mongodbreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbreceiver"

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/hashicorp/go-version"
	"go.mongodb.org/mongo-driver/bson"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scrapererror"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbreceiver/internal/metadata"
)

type mongodbScraper struct {
	logger       *zap.Logger
	config       *Config
	client       client
	mongoVersion *version.Version
	mb           *metadata.MetricsBuilder
}

func newMongodbScraper(settings receiver.CreateSettings, config *Config) *mongodbScraper {
	v, _ := version.NewVersion("0.0")
	return &mongodbScraper{
		logger:       settings.Logger,
		config:       config,
		mb:           metadata.NewMetricsBuilder(config.MetricsBuilderConfig, settings),
		mongoVersion: v,
	}
}

func (s *mongodbScraper) start(ctx context.Context, _ component.Host) error {
	c, err := NewClient(ctx, s.config, s.logger)
	if err != nil {
		return fmt.Errorf("create mongo client: %w", err)
	}
	s.client = c
	return nil
}

func (s *mongodbScraper) shutdown(ctx context.Context) error {
	if s.client != nil {
		return s.client.Disconnect(ctx)
	}
	return nil
}

func (s *mongodbScraper) scrape(ctx context.Context) (pmetric.Metrics, error) {
	if s.client == nil {
		return pmetric.NewMetrics(), errors.New("no client was initialized before calling scrape")
	}

	if s.mongoVersion == nil {
		version, err := s.client.GetVersion(ctx)
		if err == nil {
			s.mongoVersion = version
		} else {
			s.logger.Warn("determine mongo version", zap.Error(err))
		}
	}

	errs := &scrapererror.ScrapeErrors{}
	s.collectMetrics(ctx, errs)
	return s.mb.Emit(), errs.Combine()
}

func (s *mongodbScraper) collectMetrics(ctx context.Context, errs *scrapererror.ScrapeErrors) {
	dbNames, err := s.client.ListDatabaseNames(ctx, bson.D{})
	if err != nil {
		errs.AddPartial(1, fmt.Errorf("failed to fetch database names: %w", err))
		return
	}

	now := pcommon.NewTimestampFromTime(time.Now())

	rmb := s.mb.ResourceMetricsBuilder(pcommon.NewResource())
	rmb.RecordMongodbDatabaseCountDataPoint(now, int64(len(dbNames)))
	s.collectAdminDatabase(ctx, rmb, now, errs)
	s.collectTopStats(ctx, rmb, now, errs)

	for _, dbName := range dbNames {
		s.collectDatabase(ctx, now, dbName, errs)
		collectionNames, err := s.client.ListCollectionNames(ctx, dbName)
		if err != nil {
			errs.AddPartial(1, fmt.Errorf("failed to fetch collection names: %w", err))
			return
		}

		for _, collectionName := range collectionNames {
			s.collectIndexStats(ctx, now, dbName, collectionName, errs)
		}
	}
}

func (s *mongodbScraper) collectDatabase(ctx context.Context, now pcommon.Timestamp, databaseName string, errs *scrapererror.ScrapeErrors) {
	dbStats, err := s.client.DBStats(ctx, databaseName)
	rb := s.mb.NewResourceBuilder()
	rb.SetDatabase(databaseName)
	rmb := s.mb.ResourceMetricsBuilder(rb.Emit())
	if err != nil {
		errs.AddPartial(1, fmt.Errorf("failed to fetch database stats metrics: %w", err))
	} else {
		s.recordDBStats(rmb, now, dbStats, databaseName, errs)
	}

	serverStatus, err := s.client.ServerStatus(ctx, databaseName)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf("failed to fetch server status metrics: %w", err))
		return
	}
	s.recordNormalServerStats(rmb, now, serverStatus, databaseName, errs)
}

func (s *mongodbScraper) collectAdminDatabase(ctx context.Context, rmb *metadata.ResourceMetricsBuilder, now pcommon.Timestamp, errs *scrapererror.ScrapeErrors) {
	serverStatus, err := s.client.ServerStatus(ctx, "admin")
	if err != nil {
		errs.AddPartial(1, fmt.Errorf("failed to fetch admin server status metrics: %w", err))
		return
	}
	s.recordAdminStats(rmb, now, serverStatus, errs)
}

func (s *mongodbScraper) collectTopStats(ctx context.Context, rmb *metadata.ResourceMetricsBuilder, now pcommon.Timestamp, errs *scrapererror.ScrapeErrors) {
	topStats, err := s.client.TopStats(ctx)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf("failed to fetch top stats metrics: %w", err))
		return
	}
	s.recordOperationTime(rmb, now, topStats, errs)
}

func (s *mongodbScraper) collectIndexStats(ctx context.Context, now pcommon.Timestamp, databaseName string, collectionName string, errs *scrapererror.ScrapeErrors) {
	if databaseName == "local" {
		return
	}
	indexStats, err := s.client.IndexStats(ctx, databaseName, collectionName)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf("failed to fetch index stats metrics: %w", err))
		return
	}
	s.recordIndexStats(s.mb.ResourceMetricsBuilder(pcommon.NewResource()), now, indexStats, databaseName, collectionName, errs)
}

func (s *mongodbScraper) recordDBStats(rmb *metadata.ResourceMetricsBuilder, now pcommon.Timestamp, doc bson.M, dbName string, errs *scrapererror.ScrapeErrors) {
	s.recordCollections(rmb, now, doc, dbName, errs)
	s.recordDataSize(rmb, now, doc, dbName, errs)
	s.recordExtentCount(rmb, now, doc, dbName, errs)
	s.recordIndexSize(rmb, now, doc, dbName, errs)
	s.recordIndexCount(rmb, now, doc, dbName, errs)
	s.recordObjectCount(rmb, now, doc, dbName, errs)
	s.recordStorageSize(rmb, now, doc, dbName, errs)
}

func (s *mongodbScraper) recordNormalServerStats(rmb *metadata.ResourceMetricsBuilder, now pcommon.Timestamp, doc bson.M, dbName string, errs *scrapererror.ScrapeErrors) {
	s.recordConnections(rmb, now, doc, dbName, errs)
	s.recordDocumentOperations(rmb, now, doc, dbName, errs)
	s.recordMemoryUsage(rmb, now, doc, dbName, errs)
	s.recordLockAcquireCounts(rmb, now, doc, dbName, errs)
	s.recordLockAcquireWaitCounts(rmb, now, doc, dbName, errs)
	s.recordLockTimeAcquiringMicros(rmb, now, doc, dbName, errs)
	s.recordLockDeadlockCount(rmb, now, doc, dbName, errs)
}

func (s *mongodbScraper) recordAdminStats(rmb *metadata.ResourceMetricsBuilder, now pcommon.Timestamp, document bson.M, errs *scrapererror.ScrapeErrors) {
	s.recordCacheOperations(rmb, now, document, errs)
	s.recordCursorCount(rmb, now, document, errs)
	s.recordCursorTimeoutCount(rmb, now, document, errs)
	s.recordGlobalLockTime(rmb, now, document, errs)
	s.recordNetworkCount(rmb, now, document, errs)
	s.recordOperations(rmb, now, document, errs)
	s.recordOperationsRepl(rmb, now, document, errs)
	s.recordSessionCount(rmb, now, document, errs)
	s.recordLatencyTime(rmb, now, document, errs)
	s.recordUptime(rmb, now, document, errs)
	s.recordHealth(rmb, now, document, errs)
}

func (s *mongodbScraper) recordIndexStats(rmb *metadata.ResourceMetricsBuilder, now pcommon.Timestamp, indexStats []bson.M, databaseName string, collectionName string, errs *scrapererror.ScrapeErrors) {
	s.recordIndexAccess(rmb, now, indexStats, databaseName, collectionName, errs)
}
