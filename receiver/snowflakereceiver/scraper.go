// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package snowflakereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/snowflakereceiver"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scrapererror"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/snowflakereceiver/internal/metadata"
)

type snowflakeMetricsScraper struct {
	client   *snowflakeClient
	settings component.TelemetrySettings
	conf     *Config
	rb       *metadata.ResourceBuilder
	mb       *metadata.MetricsBuilder
}

func newSnowflakeMetricsScraper(settings receiver.CreateSettings, conf *Config) *snowflakeMetricsScraper {
	return &snowflakeMetricsScraper{
		settings: settings.TelemetrySettings,
		conf:     conf,
		rb:       metadata.NewResourceBuilder(conf.ResourceAttributes),
		mb:       metadata.NewMetricsBuilder(conf.MetricsBuilderConfig, settings),
	}
}

// for use with receiver.scraperhelper
func (s *snowflakeMetricsScraper) start(_ context.Context, _ component.Host) (err error) {
	s.client, err = newDefaultClient(s.settings, *s.conf)
	if err != nil {
		return err
	}
	return nil
}

func (s *snowflakeMetricsScraper) shutdown(_ context.Context) (err error) {
	if s.client == nil {
		return nil
	}
	err = s.client.client.Close()
	return err
}

// wrapper for all of the sub-scraping tasks, implements the scraper interface for
// snowflakeMetricsScraper
func (s *snowflakeMetricsScraper) scrape(ctx context.Context) (pmetric.Metrics, error) {
	errs := &scrapererror.ScrapeErrors{}

	now := pcommon.NewTimestampFromTime(time.Now())

	// each client call has its own scrape function

	s.scrapeBillingMetrics(ctx, now, *errs)
	s.scrapeWarehouseBillingMetrics(ctx, now, *errs)
	s.scrapeLoginMetrics(ctx, now, *errs)
	s.scrapeHighLevelQueryMetrics(ctx, now, *errs)
	s.scrapeDBMetrics(ctx, now, *errs)
	s.scrapeSessionMetrics(ctx, now, *errs)
	s.scrapeSnowpipeMetrics(ctx, now, *errs)
	s.scrapeStorageMetrics(ctx, now, *errs)

	s.rb.SetSnowflakeAccountName(s.conf.Account)
	return s.mb.Emit(metadata.WithResource(s.rb.Emit())), errs.Combine()
}

func (s *snowflakeMetricsScraper) scrapeBillingMetrics(ctx context.Context, t pcommon.Timestamp, errs scrapererror.ScrapeErrors) {
	billingMetrics, err := s.client.FetchBillingMetrics(ctx)

	if err != nil {
		errs.Add(err)
		return
	}

	for _, row := range *billingMetrics {
		s.mb.RecordSnowflakeBillingCloudServiceTotalDataPoint(t, row.totalCloudService, row.serviceType.String)
		s.mb.RecordSnowflakeBillingTotalCreditTotalDataPoint(t, row.totalCredits, row.serviceType.String)
		s.mb.RecordSnowflakeBillingVirtualWarehouseTotalDataPoint(t, row.totalVirtualWarehouseCredits, row.serviceType.String)
	}
}

func (s *snowflakeMetricsScraper) scrapeWarehouseBillingMetrics(ctx context.Context, t pcommon.Timestamp, errs scrapererror.ScrapeErrors) {
	warehouseBillingMetrics, err := s.client.FetchWarehouseBillingMetrics(ctx)

	if err != nil {
		errs.Add(err)
		return
	}

	for _, row := range *warehouseBillingMetrics {
		s.mb.RecordSnowflakeBillingWarehouseTotalCreditTotalDataPoint(t, row.totalCredit, row.warehouseName.String)
		s.mb.RecordSnowflakeBillingWarehouseCloudServiceTotalDataPoint(t, row.totalCloudService, row.warehouseName.String)
		s.mb.RecordSnowflakeBillingWarehouseVirtualWarehouseTotalDataPoint(t, row.totalVirtualWarehouse, row.warehouseName.String)
	}
}

func (s *snowflakeMetricsScraper) scrapeLoginMetrics(ctx context.Context, t pcommon.Timestamp, errs scrapererror.ScrapeErrors) {
	loginMetrics, err := s.client.FetchLoginMetrics(ctx)

	if err != nil {
		errs.Add(err)
		return
	}

	for _, row := range *loginMetrics {
		s.mb.RecordSnowflakeLoginsTotalDataPoint(t, row.loginsTotal, row.errorMessage.String, row.reportedClientType.String, row.isSuccess.String)
	}
}

func (s *snowflakeMetricsScraper) scrapeHighLevelQueryMetrics(ctx context.Context, t pcommon.Timestamp, errs scrapererror.ScrapeErrors) {
	highLevelQueryMetrics, err := s.client.FetchHighLevelQueryMetrics(ctx)

	if err != nil {
		errs.Add(err)
		return
	}

	for _, row := range *highLevelQueryMetrics {
		s.mb.RecordSnowflakeQueryExecutedDataPoint(t, row.avgQueryExecuted, row.warehouseName.String)
		s.mb.RecordSnowflakeQueryBlockedDataPoint(t, row.avgQueryBlocked, row.warehouseName.String)
		s.mb.RecordSnowflakeQueryQueuedOverloadDataPoint(t, row.avgQueryQueuedOverload, row.warehouseName.String)
		s.mb.RecordSnowflakeQueryQueuedProvisionDataPoint(t, row.avgQueryQueuedProvision, row.warehouseName.String)
	}
}

func (s *snowflakeMetricsScraper) scrapeDBMetrics(ctx context.Context, t pcommon.Timestamp, errs scrapererror.ScrapeErrors) {
	DBMetrics, err := s.client.FetchDbMetrics(ctx)

	if err != nil {
		errs.Add(err)
		return
	}

	for _, row := range *DBMetrics {
		s.mb.RecordSnowflakeDatabaseQueryCountDataPoint(t, row.databaseQueryCount, row.attributes.schemaName.String, row.attributes.executionStatus.String, row.attributes.errorMessage.String, row.attributes.queryType.String, row.attributes.warehouseName.String, row.attributes.databaseName.String, row.attributes.warehouseSize.String)
		s.mb.RecordSnowflakeDatabaseBytesScannedAvgDataPoint(t, row.avgBytesScanned, row.attributes.schemaName.String, row.attributes.executionStatus.String, row.attributes.errorMessage.String, row.attributes.queryType.String, row.attributes.warehouseName.String, row.attributes.databaseName.String, row.attributes.warehouseSize.String)
		s.mb.RecordSnowflakeQueryBytesDeletedAvgDataPoint(t, row.avgBytesDeleted, row.attributes.schemaName.String, row.attributes.executionStatus.String, row.attributes.errorMessage.String, row.attributes.queryType.String, row.attributes.warehouseName.String, row.attributes.databaseName.String, row.attributes.warehouseSize.String)
		s.mb.RecordSnowflakeQueryBytesSpilledRemoteAvgDataPoint(t, row.avgBytesSpilledRemote, row.attributes.schemaName.String, row.attributes.executionStatus.String, row.attributes.errorMessage.String, row.attributes.queryType.String, row.attributes.warehouseName.String, row.attributes.databaseName.String, row.attributes.warehouseSize.String)
		s.mb.RecordSnowflakeQueryBytesSpilledLocalAvgDataPoint(t, row.avgBytesSpilledLocal, row.attributes.schemaName.String, row.attributes.executionStatus.String, row.attributes.errorMessage.String, row.attributes.queryType.String, row.attributes.warehouseName.String, row.attributes.databaseName.String, row.attributes.warehouseSize.String)
		s.mb.RecordSnowflakeQueryBytesWrittenAvgDataPoint(t, row.avgBytesWritten, row.attributes.schemaName.String, row.attributes.executionStatus.String, row.attributes.errorMessage.String, row.attributes.queryType.String, row.attributes.warehouseName.String, row.attributes.databaseName.String, row.attributes.warehouseSize.String)
		s.mb.RecordSnowflakeQueryCompilationTimeAvgDataPoint(t, row.avgCompilationTime, row.attributes.schemaName.String, row.attributes.executionStatus.String, row.attributes.errorMessage.String, row.attributes.queryType.String, row.attributes.warehouseName.String, row.attributes.databaseName.String, row.attributes.warehouseSize.String)
		s.mb.RecordSnowflakeQueryDataScannedCacheAvgDataPoint(t, row.avgDataScannedCache, row.attributes.schemaName.String, row.attributes.executionStatus.String, row.attributes.errorMessage.String, row.attributes.queryType.String, row.attributes.warehouseName.String, row.attributes.databaseName.String, row.attributes.warehouseSize.String)
		s.mb.RecordSnowflakeQueryExecutionTimeAvgDataPoint(t, row.avgExecutionTime, row.attributes.schemaName.String, row.attributes.executionStatus.String, row.attributes.errorMessage.String, row.attributes.queryType.String, row.attributes.warehouseName.String, row.attributes.databaseName.String, row.attributes.warehouseSize.String)
		s.mb.RecordSnowflakeQueryPartitionsScannedAvgDataPoint(t, row.avgPartitionsScanned, row.attributes.schemaName.String, row.attributes.executionStatus.String, row.attributes.errorMessage.String, row.attributes.queryType.String, row.attributes.warehouseName.String, row.attributes.databaseName.String, row.attributes.warehouseSize.String)
		s.mb.RecordSnowflakeQueuedOverloadTimeAvgDataPoint(t, row.avgQueuedOverloadTime, row.attributes.schemaName.String, row.attributes.executionStatus.String, row.attributes.errorMessage.String, row.attributes.queryType.String, row.attributes.warehouseName.String, row.attributes.databaseName.String, row.attributes.warehouseSize.String)
		s.mb.RecordSnowflakeQueuedProvisioningTimeAvgDataPoint(t, row.avgQueuedProvisioningTime, row.attributes.schemaName.String, row.attributes.executionStatus.String, row.attributes.errorMessage.String, row.attributes.queryType.String, row.attributes.warehouseName.String, row.attributes.databaseName.String, row.attributes.warehouseSize.String)
		s.mb.RecordSnowflakeQueuedRepairTimeAvgDataPoint(t, row.avgQueuedRepairTime, row.attributes.schemaName.String, row.attributes.executionStatus.String, row.attributes.errorMessage.String, row.attributes.queryType.String, row.attributes.warehouseName.String, row.attributes.databaseName.String, row.attributes.warehouseSize.String)
		s.mb.RecordSnowflakeRowsInsertedAvgDataPoint(t, row.avgRowsInserted, row.attributes.schemaName.String, row.attributes.executionStatus.String, row.attributes.errorMessage.String, row.attributes.queryType.String, row.attributes.warehouseName.String, row.attributes.databaseName.String, row.attributes.warehouseSize.String)
		s.mb.RecordSnowflakeRowsDeletedAvgDataPoint(t, row.avgRowsDeleted, row.attributes.schemaName.String, row.attributes.executionStatus.String, row.attributes.errorMessage.String, row.attributes.queryType.String, row.attributes.warehouseName.String, row.attributes.databaseName.String, row.attributes.warehouseSize.String)
		s.mb.RecordSnowflakeRowsProducedAvgDataPoint(t, row.avgRowsProduced, row.attributes.schemaName.String, row.attributes.executionStatus.String, row.attributes.errorMessage.String, row.attributes.queryType.String, row.attributes.warehouseName.String, row.attributes.databaseName.String, row.attributes.warehouseSize.String)
		s.mb.RecordSnowflakeRowsUnloadedAvgDataPoint(t, row.avgRowsUnloaded, row.attributes.schemaName.String, row.attributes.executionStatus.String, row.attributes.errorMessage.String, row.attributes.queryType.String, row.attributes.warehouseName.String, row.attributes.databaseName.String, row.attributes.warehouseSize.String)
		s.mb.RecordSnowflakeRowsUpdatedAvgDataPoint(t, row.avgRowsUpdated, row.attributes.schemaName.String, row.attributes.executionStatus.String, row.attributes.errorMessage.String, row.attributes.queryType.String, row.attributes.warehouseName.String, row.attributes.databaseName.String, row.attributes.warehouseSize.String)
		s.mb.RecordSnowflakeTotalElapsedTimeAvgDataPoint(t, row.avgTotalElapsedTime, row.attributes.schemaName.String, row.attributes.executionStatus.String, row.attributes.errorMessage.String, row.attributes.queryType.String, row.attributes.warehouseName.String, row.attributes.databaseName.String, row.attributes.warehouseSize.String)
	}
}

func (s *snowflakeMetricsScraper) scrapeSessionMetrics(ctx context.Context, t pcommon.Timestamp, errs scrapererror.ScrapeErrors) {
	sessionMetrics, err := s.client.FetchSessionMetrics(ctx)

	if err != nil {
		errs.Add(err)
		return
	}

	for _, row := range *sessionMetrics {
		s.mb.RecordSnowflakeSessionIDCountDataPoint(t, row.distinctSessionID, row.userName.String)
	}
}

func (s *snowflakeMetricsScraper) scrapeSnowpipeMetrics(ctx context.Context, t pcommon.Timestamp, errs scrapererror.ScrapeErrors) {
	snowpipeMetrics, err := s.client.FetchSnowpipeMetrics(ctx)

	if err != nil {
		errs.Add(err)
		return
	}

	for _, row := range *snowpipeMetrics {
		s.mb.RecordSnowflakePipeCreditsUsedTotalDataPoint(t, row.creditsUsed, row.pipeName.String)
	}
}

func (s *snowflakeMetricsScraper) scrapeStorageMetrics(ctx context.Context, t pcommon.Timestamp, errs scrapererror.ScrapeErrors) {
	storageMetrics, err := s.client.FetchStorageMetrics(ctx)

	if err != nil {
		errs.Add(err)
		return
	}

	for _, row := range *storageMetrics {
		s.mb.RecordSnowflakeStorageStorageBytesTotalDataPoint(t, row.storageBytes)
		s.mb.RecordSnowflakeStorageStageBytesTotalDataPoint(t, row.stageBytes)
		s.mb.RecordSnowflakeStorageFailsafeBytesTotalDataPoint(t, row.failsafeBytes)
	}
}
