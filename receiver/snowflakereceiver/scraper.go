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
	mb       *metadata.MetricsBuilder
}

func newSnowflakeMetricsScraper(settings receiver.CreateSettings, conf *Config) *snowflakeMetricsScraper {
	return &snowflakeMetricsScraper{
		settings: settings.TelemetrySettings,
		conf:     conf,
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

	rb := s.mb.NewResourceBuilder()
	rb.SetSnowflakeAccountName(s.conf.Account)
	rmb := s.mb.ResourceMetricsBuilder(rb.Emit())

	// each client call has its own scrape function
	s.scrapeBillingMetrics(ctx, rmb, now, *errs)
	s.scrapeWarehouseBillingMetrics(ctx, rmb, now, *errs)
	s.scrapeLoginMetrics(ctx, rmb, now, *errs)
	s.scrapeHighLevelQueryMetrics(ctx, rmb, now, *errs)
	s.scrapeDBMetrics(ctx, rmb, now, *errs)
	s.scrapeSessionMetrics(ctx, rmb, now, *errs)
	s.scrapeSnowpipeMetrics(ctx, rmb, now, *errs)
	s.scrapeStorageMetrics(ctx, rmb, now, *errs)

	return s.mb.Emit(), errs.Combine()
}

func (s *snowflakeMetricsScraper) scrapeBillingMetrics(ctx context.Context, rmb *metadata.ResourceMetricsBuilder,
	t pcommon.Timestamp, errs scrapererror.ScrapeErrors) {
	billingMetrics, err := s.client.FetchBillingMetrics(ctx)

	if err != nil {
		errs.Add(err)
		return
	}

	for _, row := range *billingMetrics {
		rmb.RecordSnowflakeBillingCloudServiceTotalDataPoint(t, row.totalCloudService, row.serviceType.String)
		rmb.RecordSnowflakeBillingTotalCreditTotalDataPoint(t, row.totalCredits, row.serviceType.String)
		rmb.RecordSnowflakeBillingVirtualWarehouseTotalDataPoint(t, row.totalVirtualWarehouseCredits,
			row.serviceType.String)
	}
}

func (s *snowflakeMetricsScraper) scrapeWarehouseBillingMetrics(ctx context.Context, rmb *metadata.ResourceMetricsBuilder,
	t pcommon.Timestamp, errs scrapererror.ScrapeErrors) {
	warehouseBillingMetrics, err := s.client.FetchWarehouseBillingMetrics(ctx)

	if err != nil {
		errs.Add(err)
		return
	}

	for _, row := range *warehouseBillingMetrics {
		rmb.RecordSnowflakeBillingWarehouseTotalCreditTotalDataPoint(t, row.totalCredit, row.warehouseName.String)
		rmb.RecordSnowflakeBillingWarehouseCloudServiceTotalDataPoint(t, row.totalCloudService,
			row.warehouseName.String)
		rmb.RecordSnowflakeBillingWarehouseVirtualWarehouseTotalDataPoint(t, row.totalVirtualWarehouse,
			row.warehouseName.String)
	}
}

func (s *snowflakeMetricsScraper) scrapeLoginMetrics(ctx context.Context, rmb *metadata.ResourceMetricsBuilder,
	t pcommon.Timestamp, errs scrapererror.ScrapeErrors) {
	loginMetrics, err := s.client.FetchLoginMetrics(ctx)

	if err != nil {
		errs.Add(err)
		return
	}

	for _, row := range *loginMetrics {
		rmb.RecordSnowflakeLoginsTotalDataPoint(t, row.loginsTotal, row.errorMessage.String,
			row.reportedClientType.String, row.isSuccess.String)
	}
}

func (s *snowflakeMetricsScraper) scrapeHighLevelQueryMetrics(ctx context.Context, rmb *metadata.ResourceMetricsBuilder,
	t pcommon.Timestamp, errs scrapererror.ScrapeErrors) {
	highLevelQueryMetrics, err := s.client.FetchHighLevelQueryMetrics(ctx)

	if err != nil {
		errs.Add(err)
		return
	}

	for _, row := range *highLevelQueryMetrics {
		rmb.RecordSnowflakeQueryExecutedDataPoint(t, row.avgQueryExecuted, row.warehouseName.String)
		rmb.RecordSnowflakeQueryBlockedDataPoint(t, row.avgQueryBlocked, row.warehouseName.String)
		rmb.RecordSnowflakeQueryQueuedOverloadDataPoint(t, row.avgQueryQueuedOverload, row.warehouseName.String)
		rmb.RecordSnowflakeQueryQueuedProvisionDataPoint(t, row.avgQueryQueuedProvision, row.warehouseName.String)
	}
}

func (s *snowflakeMetricsScraper) scrapeDBMetrics(ctx context.Context, rmb *metadata.ResourceMetricsBuilder,
	t pcommon.Timestamp, errs scrapererror.ScrapeErrors) {
	DBMetrics, err := s.client.FetchDbMetrics(ctx)

	if err != nil {
		errs.Add(err)
		return
	}

	for _, row := range *DBMetrics {
		rmb.RecordSnowflakeDatabaseQueryCountDataPoint(t, row.databaseQueryCount, row.attributes.schemaName.String,
			row.attributes.executionStatus.String, row.attributes.errorMessage.String, row.attributes.queryType.String, row.attributes.warehouseName.String, row.attributes.databaseName.String, row.attributes.warehouseSize.String)
		rmb.RecordSnowflakeDatabaseBytesScannedAvgDataPoint(t, row.avgBytesScanned, row.attributes.schemaName.String,
			row.attributes.executionStatus.String, row.attributes.errorMessage.String, row.attributes.queryType.String, row.attributes.warehouseName.String, row.attributes.databaseName.String, row.attributes.warehouseSize.String)
		rmb.RecordSnowflakeQueryBytesDeletedAvgDataPoint(t, row.avgBytesDeleted, row.attributes.schemaName.String,
			row.attributes.executionStatus.String, row.attributes.errorMessage.String, row.attributes.queryType.String, row.attributes.warehouseName.String, row.attributes.databaseName.String, row.attributes.warehouseSize.String)
		rmb.RecordSnowflakeQueryBytesSpilledRemoteAvgDataPoint(t, row.avgBytesSpilledRemote,
			row.attributes.schemaName.String, row.attributes.executionStatus.String, row.attributes.errorMessage.String, row.attributes.queryType.String, row.attributes.warehouseName.String, row.attributes.databaseName.String, row.attributes.warehouseSize.String)
		rmb.RecordSnowflakeQueryBytesSpilledLocalAvgDataPoint(t, row.avgBytesSpilledLocal,
			row.attributes.schemaName.String, row.attributes.executionStatus.String, row.attributes.errorMessage.String, row.attributes.queryType.String, row.attributes.warehouseName.String, row.attributes.databaseName.String, row.attributes.warehouseSize.String)
		rmb.RecordSnowflakeQueryBytesWrittenAvgDataPoint(t, row.avgBytesWritten, row.attributes.schemaName.String,
			row.attributes.executionStatus.String, row.attributes.errorMessage.String, row.attributes.queryType.String, row.attributes.warehouseName.String, row.attributes.databaseName.String, row.attributes.warehouseSize.String)
		rmb.RecordSnowflakeQueryCompilationTimeAvgDataPoint(t, row.avgCompilationTime,
			row.attributes.schemaName.String, row.attributes.executionStatus.String, row.attributes.errorMessage.String, row.attributes.queryType.String, row.attributes.warehouseName.String, row.attributes.databaseName.String, row.attributes.warehouseSize.String)
		rmb.RecordSnowflakeQueryDataScannedCacheAvgDataPoint(t, row.avgDataScannedCache,
			row.attributes.schemaName.String, row.attributes.executionStatus.String, row.attributes.errorMessage.String, row.attributes.queryType.String, row.attributes.warehouseName.String, row.attributes.databaseName.String, row.attributes.warehouseSize.String)
		rmb.RecordSnowflakeQueryExecutionTimeAvgDataPoint(t, row.avgExecutionTime, row.attributes.schemaName.String,
			row.attributes.executionStatus.String, row.attributes.errorMessage.String, row.attributes.queryType.String, row.attributes.warehouseName.String, row.attributes.databaseName.String, row.attributes.warehouseSize.String)
		rmb.RecordSnowflakeQueryPartitionsScannedAvgDataPoint(t, row.avgPartitionsScanned,
			row.attributes.schemaName.String, row.attributes.executionStatus.String, row.attributes.errorMessage.String, row.attributes.queryType.String, row.attributes.warehouseName.String, row.attributes.databaseName.String, row.attributes.warehouseSize.String)
		rmb.RecordSnowflakeQueuedOverloadTimeAvgDataPoint(t, row.avgQueuedOverloadTime,
			row.attributes.schemaName.String, row.attributes.executionStatus.String, row.attributes.errorMessage.String, row.attributes.queryType.String, row.attributes.warehouseName.String, row.attributes.databaseName.String, row.attributes.warehouseSize.String)
		rmb.RecordSnowflakeQueuedProvisioningTimeAvgDataPoint(t, row.avgQueuedProvisioningTime,
			row.attributes.schemaName.String, row.attributes.executionStatus.String, row.attributes.errorMessage.String, row.attributes.queryType.String, row.attributes.warehouseName.String, row.attributes.databaseName.String, row.attributes.warehouseSize.String)
		rmb.RecordSnowflakeQueuedRepairTimeAvgDataPoint(t, row.avgQueuedRepairTime, row.attributes.schemaName.String,
			row.attributes.executionStatus.String, row.attributes.errorMessage.String, row.attributes.queryType.String, row.attributes.warehouseName.String, row.attributes.databaseName.String, row.attributes.warehouseSize.String)
		rmb.RecordSnowflakeRowsInsertedAvgDataPoint(t, row.avgRowsInserted, row.attributes.schemaName.String,
			row.attributes.executionStatus.String, row.attributes.errorMessage.String, row.attributes.queryType.String, row.attributes.warehouseName.String, row.attributes.databaseName.String, row.attributes.warehouseSize.String)
		rmb.RecordSnowflakeRowsDeletedAvgDataPoint(t, row.avgRowsDeleted, row.attributes.schemaName.String,
			row.attributes.executionStatus.String, row.attributes.errorMessage.String, row.attributes.queryType.String, row.attributes.warehouseName.String, row.attributes.databaseName.String, row.attributes.warehouseSize.String)
		rmb.RecordSnowflakeRowsProducedAvgDataPoint(t, row.avgRowsProduced, row.attributes.schemaName.String,
			row.attributes.executionStatus.String, row.attributes.errorMessage.String, row.attributes.queryType.String, row.attributes.warehouseName.String, row.attributes.databaseName.String, row.attributes.warehouseSize.String)
		rmb.RecordSnowflakeRowsUnloadedAvgDataPoint(t, row.avgRowsUnloaded, row.attributes.schemaName.String,
			row.attributes.executionStatus.String, row.attributes.errorMessage.String, row.attributes.queryType.String, row.attributes.warehouseName.String, row.attributes.databaseName.String, row.attributes.warehouseSize.String)
		rmb.RecordSnowflakeRowsUpdatedAvgDataPoint(t, row.avgRowsUpdated, row.attributes.schemaName.String,
			row.attributes.executionStatus.String, row.attributes.errorMessage.String, row.attributes.queryType.String, row.attributes.warehouseName.String, row.attributes.databaseName.String, row.attributes.warehouseSize.String)
		rmb.RecordSnowflakeTotalElapsedTimeAvgDataPoint(t, row.avgTotalElapsedTime, row.attributes.schemaName.String,
			row.attributes.executionStatus.String, row.attributes.errorMessage.String, row.attributes.queryType.String, row.attributes.warehouseName.String, row.attributes.databaseName.String, row.attributes.warehouseSize.String)
	}
}

func (s *snowflakeMetricsScraper) scrapeSessionMetrics(ctx context.Context, rmb *metadata.ResourceMetricsBuilder,
	t pcommon.Timestamp, errs scrapererror.ScrapeErrors) {
	sessionMetrics, err := s.client.FetchSessionMetrics(ctx)

	if err != nil {
		errs.Add(err)
		return
	}

	for _, row := range *sessionMetrics {
		rmb.RecordSnowflakeSessionIDCountDataPoint(t, row.distinctSessionID, row.userName.String)
	}
}

func (s *snowflakeMetricsScraper) scrapeSnowpipeMetrics(ctx context.Context, rmb *metadata.ResourceMetricsBuilder,
	t pcommon.Timestamp, errs scrapererror.ScrapeErrors) {
	snowpipeMetrics, err := s.client.FetchSnowpipeMetrics(ctx)

	if err != nil {
		errs.Add(err)
		return
	}

	for _, row := range *snowpipeMetrics {
		rmb.RecordSnowflakePipeCreditsUsedTotalDataPoint(t, row.creditsUsed, row.pipeName.String)
	}
}

func (s *snowflakeMetricsScraper) scrapeStorageMetrics(ctx context.Context, rmb *metadata.ResourceMetricsBuilder,
	t pcommon.Timestamp, errs scrapererror.ScrapeErrors) {
	storageMetrics, err := s.client.FetchStorageMetrics(ctx)

	if err != nil {
		errs.Add(err)
		return
	}

	for _, row := range *storageMetrics {
		rmb.RecordSnowflakeStorageStorageBytesTotalDataPoint(t, row.storageBytes)
		rmb.RecordSnowflakeStorageStageBytesTotalDataPoint(t, row.stageBytes)
		rmb.RecordSnowflakeStorageFailsafeBytesTotalDataPoint(t, row.failsafeBytes)
	}
}
