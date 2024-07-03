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
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scrapererror"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbreceiver/internal/metadata"
)

const (
	readmeURL            = "https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/receiver/mongodbreceiver/README.md"
	removeDatabaseAttrID = "receiver.mongodb.removeDatabaseAttr"
)

var (
	unknownVersion = func() *version.Version { return version.Must(version.NewVersion("0.0")) }

	removeDatabaseAttrFeatureGate = featuregate.GlobalRegistry().MustRegister(
		removeDatabaseAttrID,
		featuregate.StageBeta,
		featuregate.WithRegisterDescription("Remove duplicate database name attribute"),
		featuregate.WithRegisterReferenceURL("https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/24972"),
		featuregate.WithRegisterFromVersion("v0.90.0"))
)

type mongodbScraper struct {
	logger       *zap.Logger
	config       *Config
	client       client
	mongoVersion *version.Version
	mb           *metadata.MetricsBuilder

	// removeDatabaseAttr if enabled, will remove database attribute on database metrics
	removeDatabaseAttr bool
}

func newMongodbScraper(settings receiver.Settings, config *Config) *mongodbScraper {
	ms := &mongodbScraper{
		logger:             settings.Logger,
		config:             config,
		mb:                 metadata.NewMetricsBuilder(config.MetricsBuilderConfig, settings),
		mongoVersion:       unknownVersion(),
		removeDatabaseAttr: removeDatabaseAttrFeatureGate.IsEnabled(),
	}

	if !ms.removeDatabaseAttr {
		settings.Logger.Warn(
			fmt.Sprintf("Feature gate %s is not enabled. Please see the README for more information: %s", removeDatabaseAttrID, readmeURL),
		)
	}

	return ms
}

func (s *mongodbScraper) start(ctx context.Context, _ component.Host) error {
	c, err := newClient(ctx, s.config, s.logger)
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

	if s.mongoVersion.Equal(unknownVersion()) {
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

	s.mb.RecordMongodbDatabaseCountDataPoint(now, int64(len(dbNames)))
	s.collectAdminDatabase(ctx, now, errs)
	s.collectTopStats(ctx, now, errs)
	s.collectOplogStats(ctx, now, errs)
	s.collectReplSetStatus(ctx, now, errs)
	s.collectReplSetConfig(ctx, now, errs)
	s.collectFsyncLockStatus(ctx, now, errs)

	for _, dbName := range dbNames {
		s.collectDatabase(ctx, now, dbName, errs)
		s.collectJumboStats(ctx, now, dbName, errs)
		collectionNames, err := s.client.ListCollectionNames(ctx, dbName)
		if err != nil {
			errs.AddPartial(1, fmt.Errorf("failed to fetch collection names: %w", err))
			return
		}

		for _, collectionName := range collectionNames {
			s.collectIndexStats(ctx, now, dbName, collectionName, errs)
			s.collectCollectionStats(ctx, now, dbName, collectionName, errs)
		}
	}
}

func (s *mongodbScraper) collectDatabase(ctx context.Context, now pcommon.Timestamp, databaseName string, errs *scrapererror.ScrapeErrors) {
	dbStats, err := s.client.DBStats(ctx, databaseName)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf("failed to fetch database stats metrics: %w", err))
	} else {
		s.recordDBStats(now, dbStats, databaseName, errs)
	}

	serverStatus, err := s.client.ServerStatus(ctx, databaseName)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf("failed to fetch server status metrics: %w", err))
		return
	}

	connPoolStats, err := s.client.ConnPoolStats(ctx, databaseName)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf("failed to fetch connPoolStats metrics: %w", err))
	} else {
		s.recordConnPoolStats(now, connPoolStats, databaseName, errs)
	}

	s.recordNormalServerStats(now, serverStatus, databaseName, errs)

	rb := s.mb.NewResourceBuilder()
	rb.SetDatabase(databaseName)
	rb.SetMongodbDatabaseName(databaseName)
	s.mb.EmitForResource(
		metadata.WithResource(rb.Emit()),
	)
}

func (s *mongodbScraper) collectAdminDatabase(ctx context.Context, now pcommon.Timestamp, errs *scrapererror.ScrapeErrors) {
	serverStatus, err := s.client.ServerStatus(ctx, "admin")
	if err != nil {
		errs.AddPartial(1, fmt.Errorf("failed to fetch admin server status metrics: %w", err))
		return
	}
	s.recordAdminStats(now, serverStatus, errs)

	rb := s.mb.NewResourceBuilder()
	rb.SetDatabase("admin")
	rb.SetMongodbDatabaseName("admin")

	s.mb.EmitForResource(
		metadata.WithResource(rb.Emit()),
	)
}

func (s *mongodbScraper) collectTopStats(ctx context.Context, now pcommon.Timestamp, errs *scrapererror.ScrapeErrors) {
	topStats, err := s.client.TopStats(ctx)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf("failed to fetch top stats metrics: %w", err))
		return
	}
	s.recordOperationTime(now, topStats, errs)
	s.recordTopStats(now, topStats, errs)
	rb := s.mb.NewResourceBuilder()
	rb.SetDatabase("admin")
	rb.SetMongodbDatabaseName("admin")

	s.mb.EmitForResource(
		metadata.WithResource(rb.Emit()),
	)
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
	s.recordIndexStats(now, indexStats, databaseName, collectionName, errs)

	rb := s.mb.NewResourceBuilder()
	rb.SetDatabase(databaseName)
	rb.SetMongodbDatabaseName(databaseName)

	s.mb.EmitForResource(
		metadata.WithResource(rb.Emit()),
	)

	if s.removeDatabaseAttr {
		rb := s.mb.NewResourceBuilder()
		rb.SetDatabase(databaseName)
		s.mb.EmitForResource(metadata.WithResource(rb.Emit()))
	} else {
		s.mb.EmitForResource()
	}
}

func (s *mongodbScraper) collectJumboStats(ctx context.Context, now pcommon.Timestamp, databaseName string, errs *scrapererror.ScrapeErrors) {
	jumboStats, err := s.client.JumboStats(ctx, databaseName)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf("failed to fetch jumbo stats metrics: %w", err))
		return
	}
	s.recordJumboStats(now, jumboStats, databaseName, errs)

	rb := s.mb.NewResourceBuilder()
	rb.SetDatabase(databaseName)
	rb.SetMongodbDatabaseName(databaseName)

	s.mb.EmitForResource(
		metadata.WithResource(rb.Emit()),
	)
}

func (s *mongodbScraper) collectOplogStats(ctx context.Context, now pcommon.Timestamp, errs *scrapererror.ScrapeErrors) {
	//Oplog stats are scraped using local database
	databaseName := "local"

	oplogStats, err := s.client.GetReplicationInfo(ctx)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf("failed to fetch oplog stats metrics: %w", err))
		return
	}
	s.recordOplogStats(now, oplogStats, databaseName, errs)

	rb := s.mb.NewResourceBuilder()
	rb.SetDatabase(databaseName)
	rb.SetMongodbDatabaseName(databaseName)

	s.mb.EmitForResource(
		metadata.WithResource(rb.Emit()),
	)
}

func (s *mongodbScraper) collectFsyncLockStatus(ctx context.Context, now pcommon.Timestamp, errs *scrapererror.ScrapeErrors) {
	//FsyncLock stats are scraped using admin database
	databaseName := "admin"

	fsyncLockStatus, err := s.client.GetFsyncLockInfo(ctx)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf("failed to fetch fsyncLockStatus metrics: %w", err))
		return
	}
	s.recordMongodbFsynclocked(now, fsyncLockStatus, databaseName, errs)

	rb := s.mb.NewResourceBuilder()
	rb.SetDatabase(databaseName)
	rb.SetMongodbDatabaseName(databaseName)

	s.mb.EmitForResource(
		metadata.WithResource(rb.Emit()),
	)
}

func (s *mongodbScraper) collectCollectionStats(ctx context.Context, now pcommon.Timestamp, databaseName string, collectionName string, errs *scrapererror.ScrapeErrors) {
	collStats, err := s.client.CollectionStats(ctx, databaseName, collectionName)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf("failed to fetch collection stats metrics: %w", err))
		return
	}
	s.recordCollectionStats(now, collStats, databaseName, collectionName, errs)

	rb := s.mb.NewResourceBuilder()
	rb.SetDatabase(databaseName)
	rb.SetMongodbDatabaseName(databaseName)

	s.mb.EmitForResource(
		metadata.WithResource(rb.Emit()),
	)
}

func (s *mongodbScraper) collectReplSetStatus(ctx context.Context, now pcommon.Timestamp, errs *scrapererror.ScrapeErrors) {
	//ReplSetStatus are scraped using admin database
	database := "admin"
	status, err := s.client.ReplSetStatus(ctx)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf("failed to fetch repl set status metrics: %w", err))
		return
	}
	replset, ok := status["set"].(string)
	if ok {

		for _, mem := range status["members"].(bson.A) {
			member := mem.(bson.M)
			member_name := member["name"].(string)
			member_id := fmt.Sprint(member["_id"])
			member_state := member["stateStr"].(string)
			if member["state"].(int32) == 1 {
				s.recordMongodbReplsetOptimeLag(now, member, database, replset, member_name, member_id, errs)
			} else if member["state"].(int32) == 2 {
				s.recordMongodbReplsetReplicationlag(now, member, database, replset, member_name, member_id, errs)
			}
			s.recordMongodbReplsetHealth(now, member, database, replset, member_name, member_id, member_state, errs)
			s.recordMongodbReplsetState(now, member, database, replset, member_name, member_id, member_state, errs)
		}
	}

	rb := s.mb.NewResourceBuilder()
	rb.SetDatabase(database)
	rb.SetMongodbDatabaseName(database)

	s.mb.EmitForResource(
		metadata.WithResource(rb.Emit()),
	)
}
func (s *mongodbScraper) collectReplSetConfig(ctx context.Context, now pcommon.Timestamp, errs *scrapererror.ScrapeErrors) {
	//ReplSetConfig are scraped using admin database
	database := "admin"
	config, err := s.client.ReplSetConfig(ctx)
	if err != nil {
		errs.AddPartial(1, fmt.Errorf("failed to fetch repl set get config metrics: %w", err))
		return
	}
	config, ok := config["config"].(bson.M)
	if ok {

		replset := config["_id"].(string)

		for _, mem := range config["members"].(bson.A) {
			member := mem.(bson.M)
			member_name := member["host"].(string)
			member_id := fmt.Sprint(member["_id"])

			// replSetGetConfig
			s.recordMongodbReplsetVotefraction(now, member, database, replset, member_name, member_id, errs)
			s.recordMongodbReplsetVotes(now, member, database, replset, member_name, member_id, errs)
		}
	}

	rb := s.mb.NewResourceBuilder()
	rb.SetDatabase(database)
	rb.SetMongodbDatabaseName(database)

	s.mb.EmitForResource(
		metadata.WithResource(rb.Emit()),
	)
}

func (s *mongodbScraper) recordDBStats(now pcommon.Timestamp, doc bson.M, dbName string, errs *scrapererror.ScrapeErrors) {
	s.recordCollections(now, doc, dbName, errs)
	s.recordDataSize(now, doc, dbName, errs)
	s.recordExtentCount(now, doc, dbName, errs)
	s.recordIndexSize(now, doc, dbName, errs)
	s.recordIndexCount(now, doc, dbName, errs)
	s.recordObjectCount(now, doc, dbName, errs)
	s.recordStorageSize(now, doc, dbName, errs)

	// stats
	s.recordMongodbStatsAvgobjsize(now, doc, dbName, errs)
	s.recordMongodbStatsCollections(now, doc, dbName, errs)
	s.recordMongodbStatsDatasize(now, doc, dbName, errs)
	s.recordMongodbStatsFilesize(now, doc, dbName, errs) //mmapv1 only
	s.recordMongodbStatsIndexes(now, doc, dbName, errs)
	s.recordMongodbStatsIndexsize(now, doc, dbName, errs)
	s.recordMongodbStatsNumextents(now, doc, dbName, errs) //mmapv1 only
	s.recordMongodbStatsObjects(now, doc, dbName, errs)
	s.recordMongodbStatsStoragesize(now, doc, dbName, errs)
}

func (s *mongodbScraper) recordNormalServerStats(now pcommon.Timestamp, doc bson.M, dbName string, errs *scrapererror.ScrapeErrors) {
	s.recordConnections(now, doc, dbName, errs)
	s.recordDocumentOperations(now, doc, dbName, errs)
	s.recordMemoryUsage(now, doc, dbName, errs)
	s.recordLockAcquireCounts(now, doc, dbName, errs)
	s.recordLockAcquireWaitCounts(now, doc, dbName, errs)
	s.recordLockTimeAcquiringMicros(now, doc, dbName, errs)
	s.recordLockDeadlockCount(now, doc, dbName, errs)

	// asserts
	s.recordMongodbAssertsMsgps(now, doc, dbName, errs)       // ps
	s.recordMongodbAssertsRegularps(now, doc, dbName, errs)   // ps
	s.recordMongodbAssertsRolloversps(now, doc, dbName, errs) // ps
	s.recordMongodbAssertsUserps(now, doc, dbName, errs)      // ps
	s.recordMongodbAssertsWarningps(now, doc, dbName, errs)   // ps

	// backgroundflushing
	// Mongo version 4.4+ no longer returns backgroundflushing since it is part of the obsolete MMAPv1
	mongo44, _ := version.NewVersion("4.4")
	if s.mongoVersion != nil && s.mongoVersion.LessThan(mongo44) {
		s.recordMongodbBackgroundflushingAverageMs(now, doc, dbName, errs)
		s.recordMongodbBackgroundflushingFlushesps(now, doc, dbName, errs)
		s.recordMongodbBackgroundflushingLastMs(now, doc, dbName, errs)
		s.recordMongodbBackgroundflushingTotalMs(now, doc, dbName, errs)
	}

	// connections
	s.recordMongodbConnectionsActive(now, doc, dbName, errs)
	s.recordMongodbConnectionsAvailable(now, doc, dbName, errs)
	s.recordMongodbConnectionsAwaitingtopologychanges(now, doc, dbName, errs)
	s.recordMongodbConnectionsCurrent(now, doc, dbName, errs)
	s.recordMongodbConnectionsExhausthello(now, doc, dbName, errs)
	s.recordMongodbConnectionsExhaustismaster(now, doc, dbName, errs)
	s.recordMongodbConnectionsLoadbalanced(now, doc, dbName, errs)
	s.recordMongodbConnectionsRejected(now, doc, dbName, errs)
	s.recordMongodbConnectionsThreaded(now, doc, dbName, errs)
	s.recordMongodbConnectionsTotalcreated(now, doc, dbName, errs)

	// cursors
	s.recordMongodbCursorsTimedout(now, doc, dbName, errs)
	s.recordMongodbCursorsTotalopen(now, doc, dbName, errs)

	// dur
	// Mongo version 4.4+ no longer returns dur since it is part of the obsolete MMAPv1
	if s.mongoVersion != nil && s.mongoVersion.LessThan(mongo44) {
		s.recordMongodbDurCommits(now, doc, dbName, errs)
		s.recordMongodbDurCommitsinwritelock(now, doc, dbName, errs)
		s.recordMongodbDurCompression(now, doc, dbName, errs)
		s.recordMongodbDurEarlycommits(now, doc, dbName, errs)
		s.recordMongodbDurJournaledmb(now, doc, dbName, errs)
		s.recordMongodbDurTimemsCommits(now, doc, dbName, errs)
		s.recordMongodbDurTimemsCommitsinwritelock(now, doc, dbName, errs)
		s.recordMongodbDurTimemsDt(now, doc, dbName, errs)
		s.recordMongodbDurTimemsPreplogbuffer(now, doc, dbName, errs)
		s.recordMongodbDurTimemsRemapprivateview(now, doc, dbName, errs)
		s.recordMongodbDurTimemsWritetodatafiles(now, doc, dbName, errs)
		s.recordMongodbDurTimemsWritetojournal(now, doc, dbName, errs)
		s.recordMongodbDurWritetodatafilesmb(now, doc, dbName, errs)

		// extra_info
		s.recordMongodbExtraInfoHeapUsageBytesps(now, doc, dbName, errs)
	}

	// extra_info
	s.recordMongodbExtraInfoPageFaultsps(now, doc, dbName, errs) // ps

	// globallock
	s.recordMongodbGloballockActiveclientsReaders(now, doc, dbName, errs)
	s.recordMongodbGloballockActiveclientsTotal(now, doc, dbName, errs)
	s.recordMongodbGloballockActiveclientsWriters(now, doc, dbName, errs)
	s.recordMongodbGloballockCurrentqueueReaders(now, doc, dbName, errs)
	s.recordMongodbGloballockCurrentqueueTotal(now, doc, dbName, errs)
	s.recordMongodbGloballockCurrentqueueWriters(now, doc, dbName, errs)
	// Mongo version 4.4+ no longer returns locktime and ratio since it is part of the obsolete MMAPv1
	if s.mongoVersion != nil && s.mongoVersion.LessThan(mongo44) {
		s.recordMongodbGloballockLocktime(now, doc, dbName, errs)
		s.recordMongodbGloballockRatio(now, doc, dbName, errs)
	}
	s.recordMongodbGloballockTotaltime(now, doc, dbName, errs)

	// indexcounters
	// Mongo version 4.4+ no longer returns indexcounters since it is part of the obsolete MMAPv1
	if s.mongoVersion != nil && s.mongoVersion.LessThan(mongo44) {
		s.recordMongodbIndexcountersAccessesps(now, doc, dbName, errs) //ps
		s.recordMongodbIndexcountersHitsps(now, doc, dbName, errs)     //ps
		s.recordMongodbIndexcountersMissesps(now, doc, dbName, errs)   //ps
		s.recordMongodbIndexcountersMissratio(now, doc, dbName, errs)
		s.recordMongodbIndexcountersResetsps(now, doc, dbName, errs) //ps
	}

	// locks
	s.recordMongodbLocksCollectionAcquirecountExclusiveps(now, doc, dbName, errs)            // ps
	s.recordMongodbLocksCollectionAcquirecountIntentExclusiveps(now, doc, dbName, errs)      // ps
	s.recordMongodbLocksCollectionAcquirecountIntentSharedps(now, doc, dbName, errs)         // ps
	s.recordMongodbLocksCollectionAcquirecountSharedps(now, doc, dbName, errs)               // ps
	s.recordMongodbLocksCollectionAcquirewaitcountExclusiveps(now, doc, dbName, errs)        // ps
	s.recordMongodbLocksCollectionAcquirewaitcountSharedps(now, doc, dbName, errs)           // ps
	s.recordMongodbLocksCollectionTimeacquiringmicrosExclusiveps(now, doc, dbName, errs)     // ps
	s.recordMongodbLocksCollectionTimeacquiringmicrosSharedps(now, doc, dbName, errs)        // ps
	s.recordMongodbLocksDatabaseAcquirecountExclusiveps(now, doc, dbName, errs)              // ps
	s.recordMongodbLocksDatabaseAcquirecountIntentExclusiveps(now, doc, dbName, errs)        // ps
	s.recordMongodbLocksDatabaseAcquirecountIntentSharedps(now, doc, dbName, errs)           // ps
	s.recordMongodbLocksDatabaseAcquirecountSharedps(now, doc, dbName, errs)                 // ps
	s.recordMongodbLocksDatabaseAcquirewaitcountExclusiveps(now, doc, dbName, errs)          // ps
	s.recordMongodbLocksDatabaseAcquirewaitcountIntentExclusiveps(now, doc, dbName, errs)    // ps
	s.recordMongodbLocksDatabaseAcquirewaitcountIntentSharedps(now, doc, dbName, errs)       // ps
	s.recordMongodbLocksDatabaseAcquirewaitcountSharedps(now, doc, dbName, errs)             // ps
	s.recordMongodbLocksDatabaseTimeacquiringmicrosExclusiveps(now, doc, dbName, errs)       // ps
	s.recordMongodbLocksDatabaseTimeacquiringmicrosIntentExclusiveps(now, doc, dbName, errs) // ps
	s.recordMongodbLocksDatabaseTimeacquiringmicrosIntentSharedps(now, doc, dbName, errs)    // ps
	s.recordMongodbLocksDatabaseTimeacquiringmicrosSharedps(now, doc, dbName, errs)          // ps
	s.recordMongodbLocksGlobalAcquirecountExclusiveps(now, doc, dbName, errs)                // ps
	s.recordMongodbLocksGlobalAcquirecountIntentExclusiveps(now, doc, dbName, errs)          // ps
	s.recordMongodbLocksGlobalAcquirecountIntentSharedps(now, doc, dbName, errs)             // ps
	s.recordMongodbLocksGlobalAcquirecountSharedps(now, doc, dbName, errs)                   // ps
	s.recordMongodbLocksGlobalAcquirewaitcountExclusiveps(now, doc, dbName, errs)            // ps
	s.recordMongodbLocksGlobalAcquirewaitcountIntentExclusiveps(now, doc, dbName, errs)      // ps
	s.recordMongodbLocksGlobalAcquirewaitcountIntentSharedps(now, doc, dbName, errs)         // ps
	s.recordMongodbLocksGlobalAcquirewaitcountSharedps(now, doc, dbName, errs)               // ps
	s.recordMongodbLocksGlobalTimeacquiringmicrosExclusiveps(now, doc, dbName, errs)         // ps
	s.recordMongodbLocksGlobalTimeacquiringmicrosIntentExclusiveps(now, doc, dbName, errs)   // ps
	s.recordMongodbLocksGlobalTimeacquiringmicrosIntentSharedps(now, doc, dbName, errs)      // ps
	s.recordMongodbLocksGlobalTimeacquiringmicrosSharedps(now, doc, dbName, errs)            // ps
	s.recordMongodbLocksMetadataAcquirecountExclusiveps(now, doc, dbName, errs)              // ps
	s.recordMongodbLocksMetadataAcquirecountSharedps(now, doc, dbName, errs)                 // ps

	// since it is part of the obsolete MMAPv1
	if s.mongoVersion != nil && s.mongoVersion.LessThan(mongo44) {
		s.recordMongodbLocksMmapv1journalAcquirecountIntentExclusiveps(now, doc, dbName, errs)        // ps
		s.recordMongodbLocksMmapv1journalAcquirecountIntentSharedps(now, doc, dbName, errs)           // ps
		s.recordMongodbLocksMmapv1journalAcquirewaitcountIntentExclusiveps(now, doc, dbName, errs)    // ps
		s.recordMongodbLocksMmapv1journalAcquirewaitcountIntentSharedps(now, doc, dbName, errs)       // ps
		s.recordMongodbLocksMmapv1journalTimeacquiringmicrosIntentExclusiveps(now, doc, dbName, errs) // ps
		s.recordMongodbLocksMmapv1journalTimeacquiringmicrosIntentSharedps(now, doc, dbName, errs)    // ps
	}
	s.recordMongodbLocksOplogAcquirecountIntentExclusiveps(now, doc, dbName, errs)        // ps
	s.recordMongodbLocksOplogAcquirecountSharedps(now, doc, dbName, errs)                 // ps
	s.recordMongodbLocksOplogAcquirewaitcountIntentExclusiveps(now, doc, dbName, errs)    // ps
	s.recordMongodbLocksOplogAcquirewaitcountSharedps(now, doc, dbName, errs)             // ps
	s.recordMongodbLocksOplogTimeacquiringmicrosIntentExclusiveps(now, doc, dbName, errs) // ps
	s.recordMongodbLocksOplogTimeacquiringmicrosSharedps(now, doc, dbName, errs)          // ps

	// mem
	s.recordMongodbMemBits(now, doc, dbName, errs)
	// since it is part of the obsolete MMAPv1
	if s.mongoVersion != nil && s.mongoVersion.LessThan(mongo44) {
		s.recordMongodbMemMapped(now, doc, dbName, errs)
		s.recordMongodbMemMappedwithjournal(now, doc, dbName, errs)
	}
	s.recordMongodbMemResident(now, doc, dbName, errs)
	s.recordMongodbMemVirtual(now, doc, dbName, errs)

	// metrics
	s.recordMongodbMetricsCommandsCountFailedps(now, doc, dbName, errs) // ps
	s.recordMongodbMetricsCommandsCountTotal(now, doc, dbName, errs)
	s.recordMongodbMetricsCommandsCreateindexesFailedps(now, doc, dbName, errs) // ps
	s.recordMongodbMetricsCommandsCreateindexesTotal(now, doc, dbName, errs)
	s.recordMongodbMetricsCommandsDeleteFailedps(now, doc, dbName, errs) // ps
	s.recordMongodbMetricsCommandsDeleteTotal(now, doc, dbName, errs)
	s.recordMongodbMetricsCommandsEvalFailedps(now, doc, dbName, errs) // ps
	s.recordMongodbMetricsCommandsEvalTotal(now, doc, dbName, errs)
	s.recordMongodbMetricsCommandsFindandmodifyFailedps(now, doc, dbName, errs) // ps
	s.recordMongodbMetricsCommandsFindandmodifyTotal(now, doc, dbName, errs)
	s.recordMongodbMetricsCommandsInsertFailedps(now, doc, dbName, errs) // ps
	s.recordMongodbMetricsCommandsInsertTotal(now, doc, dbName, errs)
	s.recordMongodbMetricsCommandsUpdateFailedps(now, doc, dbName, errs) // ps
	s.recordMongodbMetricsCommandsUpdateTotal(now, doc, dbName, errs)

	s.recordMongodbMetricsCursorOpenNotimeout(now, doc, dbName, errs)
	s.recordMongodbMetricsCursorOpenPinned(now, doc, dbName, errs)
	s.recordMongodbMetricsCursorOpenTotal(now, doc, dbName, errs)
	s.recordMongodbMetricsCursorTimedoutps(now, doc, dbName, errs) // ps

	s.recordMongodbMetricsDocumentDeletedps(now, doc, dbName, errs)  // ps
	s.recordMongodbMetricsDocumentInsertedps(now, doc, dbName, errs) // ps
	s.recordMongodbMetricsDocumentReturnedps(now, doc, dbName, errs) // ps
	s.recordMongodbMetricsDocumentUpdatedps(now, doc, dbName, errs)  // ps

	s.recordMongodbMetricsGetlasterrorWtimeNumps(now, doc, dbName, errs)         // ps
	s.recordMongodbMetricsGetlasterrorWtimeTotalmillisps(now, doc, dbName, errs) // ps
	s.recordMongodbMetricsGetlasterrorWtimeoutsps(now, doc, dbName, errs)        // ps

	s.recordMongodbMetricsOperationFastmodps(now, doc, dbName, errs)        // ps
	s.recordMongodbMetricsOperationIdhackps(now, doc, dbName, errs)         // ps
	s.recordMongodbMetricsOperationScanandorderps(now, doc, dbName, errs)   // ps
	s.recordMongodbMetricsOperationWriteconflictsps(now, doc, dbName, errs) // ps

	s.recordMongodbMetricsQueryexecutorScannedobjectsps(now, doc, dbName, errs) // ps
	s.recordMongodbMetricsQueryexecutorScannedps(now, doc, dbName, errs)        // ps

	s.recordMongodbMetricsRecordMovesps(now, doc, dbName, errs) // ps

	s.recordMongodbMetricsReplApplyBatchesNumps(now, doc, dbName, errs)         // ps
	s.recordMongodbMetricsReplApplyBatchesTotalmillisps(now, doc, dbName, errs) // ps
	s.recordMongodbMetricsReplApplyOpsps(now, doc, dbName, errs)                // ps

	s.recordMongodbMetricsReplBufferCount(now, doc, dbName, errs)
	s.recordMongodbMetricsReplBufferMaxsizebytes(now, doc, dbName, errs)
	s.recordMongodbMetricsReplBufferSizebytes(now, doc, dbName, errs)

	s.recordMongodbMetricsReplNetworkBytesps(now, doc, dbName, errs)               // ps
	s.recordMongodbMetricsReplNetworkGetmoresNumps(now, doc, dbName, errs)         // ps
	s.recordMongodbMetricsReplNetworkGetmoresTotalmillisps(now, doc, dbName, errs) // ps
	s.recordMongodbMetricsReplNetworkOpsps(now, doc, dbName, errs)                 // ps
	s.recordMongodbMetricsReplNetworkReaderscreatedps(now, doc, dbName, errs)      // ps

	s.recordMongodbMetricsReplPreloadDocsNumps(now, doc, dbName, errs)            // ps
	s.recordMongodbMetricsReplPreloadDocsTotalmillisps(now, doc, dbName, errs)    // ps
	s.recordMongodbMetricsReplPreloadIndexesNumps(now, doc, dbName, errs)         // ps
	s.recordMongodbMetricsReplPreloadIndexesTotalmillisps(now, doc, dbName, errs) // ps

	s.recordMongodbMetricsTtlDeleteddocumentsps(now, doc, dbName, errs) // ps
	s.recordMongodbMetricsTtlPassesps(now, doc, dbName, errs)           // ps

	// network
	s.recordMongodbNetworkBytesinps(now, doc, dbName, errs)     // ps
	s.recordMongodbNetworkBytesoutps(now, doc, dbName, errs)    // ps
	s.recordMongodbNetworkNumrequestsps(now, doc, dbName, errs) // ps
	// opcounters
	s.recordMongodbOpcountersCommandps(now, doc, dbName, errs) // ps
	s.recordMongodbOpcountersDeleteps(now, doc, dbName, errs)  // ps
	s.recordMongodbOpcountersGetmoreps(now, doc, dbName, errs) // ps
	s.recordMongodbOpcountersInsertps(now, doc, dbName, errs)  // ps
	s.recordMongodbOpcountersQueryps(now, doc, dbName, errs)   // ps
	s.recordMongodbOpcountersUpdateps(now, doc, dbName, errs)  // ps
	// opcountersrepl
	s.recordMongodbOpcountersreplCommandps(now, doc, dbName, errs) // ps
	s.recordMongodbOpcountersreplDeleteps(now, doc, dbName, errs)  // ps
	s.recordMongodbOpcountersreplGetmoreps(now, doc, dbName, errs) // ps
	s.recordMongodbOpcountersreplInsertps(now, doc, dbName, errs)  // ps
	s.recordMongodbOpcountersreplQueryps(now, doc, dbName, errs)   // ps
	s.recordMongodbOpcountersreplUpdateps(now, doc, dbName, errs)  // ps
	// oplatencies
	s.recordMongodbOplatenciesCommandsLatency(now, doc, dbName, errs) //with ps
	s.recordMongodbOplatenciesReadsLatency(now, doc, dbName, errs)    //with ps
	s.recordMongodbOplatenciesWritesLatency(now, doc, dbName, errs)   //with ps

	// tcmalloc
	s.recordMongodbTcmallocGenericCurrentAllocatedBytes(now, doc, dbName, errs)
	s.recordMongodbTcmallocGenericHeapSize(now, doc, dbName, errs)
	s.recordMongodbTcmallocTcmallocAggressiveMemoryDecommit(now, doc, dbName, errs)
	s.recordMongodbTcmallocTcmallocCentralCacheFreeBytes(now, doc, dbName, errs)
	s.recordMongodbTcmallocTcmallocCurrentTotalThreadCacheBytes(now, doc, dbName, errs)
	s.recordMongodbTcmallocTcmallocMaxTotalThreadCacheBytes(now, doc, dbName, errs)
	s.recordMongodbTcmallocTcmallocPageheapFreeBytes(now, doc, dbName, errs)
	s.recordMongodbTcmallocTcmallocPageheapUnmappedBytes(now, doc, dbName, errs)
	s.recordMongodbTcmallocTcmallocSpinlockTotalDelayNs(now, doc, dbName, errs)
	s.recordMongodbTcmallocTcmallocThreadCacheFreeBytes(now, doc, dbName, errs)
	s.recordMongodbTcmallocTcmallocTransferCacheFreeBytes(now, doc, dbName, errs)

	// wiredtiger
	s.recordMongodbWiredtigerCacheBytesCurrentlyInCache(now, doc, dbName, errs)
	s.recordMongodbWiredtigerCacheFailedEvictionOfPagesExceedingTheInMemoryMaximumps(now, doc, dbName, errs)
	s.recordMongodbWiredtigerCacheInMemoryPageSplits(now, doc, dbName, errs)
	s.recordMongodbWiredtigerCacheMaximumBytesConfigured(now, doc, dbName, errs)
	s.recordMongodbWiredtigerCacheMaximumPageSizeAtEviction(now, doc, dbName, errs)
	s.recordMongodbWiredtigerCacheModifiedPagesEvicted(now, doc, dbName, errs)
	s.recordMongodbWiredtigerCachePagesCurrentlyHeldInCache(now, doc, dbName, errs)
	s.recordMongodbWiredtigerCachePagesEvictedByApplicationThreadsps(now, doc, dbName, errs)
	s.recordMongodbWiredtigerCachePagesEvictedExceedingTheInMemoryMaximumps(now, doc, dbName, errs)
	s.recordMongodbWiredtigerCachePagesReadIntoCache(now, doc, dbName, errs)
	s.recordMongodbWiredtigerCachePagesWrittenFromCache(now, doc, dbName, errs)
	s.recordMongodbWiredtigerCacheTrackedDirtyBytesInCache(now, doc, dbName, errs)
	s.recordMongodbWiredtigerCacheUnmodifiedPagesEvicted(now, doc, dbName, errs)
	s.recordMongodbWiredtigerConcurrenttransactionsReadAvailable(now, doc, dbName, errs)
	s.recordMongodbWiredtigerConcurrenttransactionsReadOut(now, doc, dbName, errs)
	s.recordMongodbWiredtigerConcurrenttransactionsReadTotaltickets(now, doc, dbName, errs)
	s.recordMongodbWiredtigerConcurrenttransactionsWriteAvailable(now, doc, dbName, errs)
	s.recordMongodbWiredtigerConcurrenttransactionsWriteOut(now, doc, dbName, errs)
	s.recordMongodbWiredtigerConcurrenttransactionsWriteTotaltickets(now, doc, dbName, errs)
}

func (s *mongodbScraper) recordAdminStats(now pcommon.Timestamp, document bson.M, errs *scrapererror.ScrapeErrors) {
	s.recordCacheOperations(now, document, errs)
	s.recordCursorCount(now, document, errs)
	s.recordCursorTimeoutCount(now, document, errs)
	s.recordGlobalLockTime(now, document, errs)
	s.recordNetworkCount(now, document, errs)
	s.recordOperations(now, document, errs)
	s.recordOperationsRepl(now, document, errs)
	s.recordSessionCount(now, document, errs)
	s.recordLatencyTime(now, document, errs)
	s.recordUptime(now, document, errs)
	s.recordHealth(now, document, errs)
}

func (s *mongodbScraper) recordIndexStats(now pcommon.Timestamp, indexStats []bson.M, databaseName string, collectionName string, errs *scrapererror.ScrapeErrors) {
	s.recordIndexAccess(now, indexStats, databaseName, collectionName, errs)
}

func (s *mongodbScraper) recordJumboStats(now pcommon.Timestamp, doc bson.M, dbName string, errs *scrapererror.ScrapeErrors) {
	// chunks
	s.recordMongodbChunksJumbo(now, doc, dbName, errs)
	s.recordMongodbChunksTotal(now, doc, dbName, errs)
}
func (s *mongodbScraper) recordOplogStats(now pcommon.Timestamp, doc bson.M, dbName string, errs *scrapererror.ScrapeErrors) {
	// oplog
	s.recordMongodbOplogLogsizemb(now, doc, dbName, errs)
	s.recordMongodbOplogTimediff(now, doc, dbName, errs)
	s.recordMongodbOplogUsedsizemb(now, doc, dbName, errs)
}

func (s *mongodbScraper) recordCollectionStats(now pcommon.Timestamp, doc bson.M, database string, collection string, errs *scrapererror.ScrapeErrors) {
	// collectiondbName
	s.recordMongodbCollectionAvgobjsize(now, doc, database, collection, errs)
	s.recordMongodbCollectionCapped(now, doc, database, collection, errs)
	s.recordMongodbCollectionObjects(now, doc, database, collection, errs)
	// s.recordMongodbCollectionIndexesAccessesOps(now, doc, database, collection, errs)
	s.recordMongodbCollectionIndexsizes(now, doc, database, collection, errs)
	s.recordMongodbCollectionMax(now, doc, database, collection, errs)
	s.recordMongodbCollectionMaxsize(now, doc, database, collection, errs)
	s.recordMongodbCollectionNindexes(now, doc, database, collection, errs)
	s.recordMongodbCollectionSize(now, doc, database, collection, errs)
	s.recordMongodbCollectionStoragesize(now, doc, database, collection, errs)
}

func (s *mongodbScraper) recordConnPoolStats(now pcommon.Timestamp, doc bson.M, database string, errs *scrapererror.ScrapeErrors) {
	// connection_pool
	s.recordMongodbConnectionPoolNumascopedconnections(now, doc, database, errs)
	s.recordMongodbConnectionPoolNumclientconnections(now, doc, database, errs)
	s.recordMongodbConnectionPoolTotalavailable(now, doc, database, errs)
	s.recordMongodbConnectionPoolTotalcreatedps(now, doc, database, errs)
	s.recordMongodbConnectionPoolTotalinuse(now, doc, database, errs)
	s.recordMongodbConnectionPoolTotalrefreshing(now, doc, database, errs)
}
