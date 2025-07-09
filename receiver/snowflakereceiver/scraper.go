// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package snowflakereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/snowflakereceiver"

import (
	"context"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/scraper/scrapererror"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/snowflakereceiver/internal/metadata"
)

type snowflakeMetricsScraper struct {
	client   *snowflakeClient
	settings component.TelemetrySettings
	conf     *Config
	mb       *metadata.MetricsBuilder
}

func newSnowflakeMetricsScraper(settings receiver.Settings, conf *Config) *snowflakeMetricsScraper {
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

func errorListener(eQueue <-chan error, eOut chan<- *scrapererror.ScrapeErrors) {
	errs := &scrapererror.ScrapeErrors{}

	for err := range eQueue {
		errs.Add(err)
	}

	eOut <- errs
}

// wrapper for all of the sub-scraping tasks, implements the scraper interface for
// snowflakeMetricsScraper
func (s *snowflakeMetricsScraper) scrape(ctx context.Context) (pmetric.Metrics, error) {
	var wg sync.WaitGroup
	var errs *scrapererror.ScrapeErrors

	metricScrapes := []func(context.Context, pcommon.Timestamp, chan<- error){
		s.scrapeBillingMetrics,
		s.scrapeWarehouseBillingMetrics,
		s.scrapeLoginMetrics,
		s.scrapeHighLevelQueryMetrics,
		s.scrapeDBMetrics,
		s.scrapeSessionMetrics,
		s.scrapeSnowpipeMetrics,
		s.scrapeStorageMetrics,
	}

	errChan := make(chan error, len(metricScrapes))
	errOut := make(chan *scrapererror.ScrapeErrors)

	go func() {
		errorListener(errChan, errOut)
	}()

	now := pcommon.NewTimestampFromTime(time.Now())

	for _, fn := range metricScrapes {
		wg.Add(1)
		go func(
			fn func(context.Context, pcommon.Timestamp, chan<- error),
			ctx context.Context,
			now pcommon.Timestamp,
			errs chan<- error,
		) {
			defer wg.Done()
			fn(ctx, now, errs)
		}(fn, ctx, now, errChan)
	}

	wg.Wait()
	close(errChan)
	errs = <-errOut
	rb := s.mb.NewResourceBuilder()
	rb.SetSnowflakeAccountName(s.conf.Account)
	return s.mb.Emit(metadata.WithResource(rb.Emit())), errs.Combine()
}

func (s *snowflakeMetricsScraper) scrapeBillingMetrics(ctx context.Context, t pcommon.Timestamp, errs chan<- error) {
	if !s.conf.Metrics.SnowflakeBillingCloudServiceTotal.Enabled && !s.conf.Metrics.SnowflakeBillingTotalCreditTotal.Enabled && !s.conf.Metrics.SnowflakeBillingVirtualWarehouseTotal.Enabled {
		return
	}

	billingMetrics, err := s.client.FetchBillingMetrics(ctx)
	if err != nil {
		errs <- err
		return
	}

	for _, row := range *billingMetrics {
		s.mb.RecordSnowflakeBillingCloudServiceTotalDataPoint(t, row.totalCloudService, row.serviceType.String)
		s.mb.RecordSnowflakeBillingTotalCreditTotalDataPoint(t, row.totalCredits, row.serviceType.String)
		s.mb.RecordSnowflakeBillingVirtualWarehouseTotalDataPoint(t, row.totalVirtualWarehouseCredits, row.serviceType.String)
	}
}

func (s *snowflakeMetricsScraper) scrapeWarehouseBillingMetrics(ctx context.Context, t pcommon.Timestamp, errs chan<- error) {
	if !s.conf.Metrics.SnowflakeBillingWarehouseTotalCreditTotal.Enabled && !s.conf.Metrics.SnowflakeBillingWarehouseCloudServiceTotal.Enabled && !s.conf.Metrics.SnowflakeBillingWarehouseVirtualWarehouseTotal.Enabled {
		return
	}

	warehouseBillingMetrics, err := s.client.FetchWarehouseBillingMetrics(ctx)
	if err != nil {
		errs <- err
		return
	}

	for _, row := range *warehouseBillingMetrics {
		s.mb.RecordSnowflakeBillingWarehouseTotalCreditTotalDataPoint(t, row.totalCredit, row.warehouseName.String)
		s.mb.RecordSnowflakeBillingWarehouseCloudServiceTotalDataPoint(t, row.totalCloudService, row.warehouseName.String)
		s.mb.RecordSnowflakeBillingWarehouseVirtualWarehouseTotalDataPoint(t, row.totalVirtualWarehouse, row.warehouseName.String)
	}
}

func (s *snowflakeMetricsScraper) scrapeLoginMetrics(ctx context.Context, t pcommon.Timestamp, errs chan<- error) {
	if !s.conf.Metrics.SnowflakeLoginsTotal.Enabled {
		return
	}

	loginMetrics, err := s.client.FetchLoginMetrics(ctx)
	if err != nil {
		errs <- err
		return
	}

	for _, row := range *loginMetrics {
		s.mb.RecordSnowflakeLoginsTotalDataPoint(t, row.loginsTotal, row.errorMessage.String, row.reportedClientType.String, row.isSuccess.String)
	}
}

func (s *snowflakeMetricsScraper) scrapeHighLevelQueryMetrics(ctx context.Context, t pcommon.Timestamp, errs chan<- error) {
	if !s.conf.Metrics.SnowflakeQueryExecuted.Enabled && !s.conf.Metrics.SnowflakeQueryBlocked.Enabled && !s.conf.Metrics.SnowflakeQueryQueuedOverload.Enabled && !s.conf.Metrics.SnowflakeQueryQueuedProvision.Enabled {
		return
	}
	highLevelQueryMetrics, err := s.client.FetchHighLevelQueryMetrics(ctx)
	if err != nil {
		errs <- err
		return
	}

	for _, row := range *highLevelQueryMetrics {
		s.mb.RecordSnowflakeQueryExecutedDataPoint(t, row.avgQueryExecuted, row.warehouseName.String)
		s.mb.RecordSnowflakeQueryBlockedDataPoint(t, row.avgQueryBlocked, row.warehouseName.String)
		s.mb.RecordSnowflakeQueryQueuedOverloadDataPoint(t, row.avgQueryQueuedOverload, row.warehouseName.String)
		s.mb.RecordSnowflakeQueryQueuedProvisionDataPoint(t, row.avgQueryQueuedProvision, row.warehouseName.String)
	}
}

func (s *snowflakeMetricsScraper) scrapeDBMetrics(ctx context.Context, t pcommon.Timestamp, errs chan<- error) {
	if !s.conf.Metrics.SnowflakeDatabaseQueryCount.Enabled && !s.conf.Metrics.SnowflakeDatabaseBytesScannedAvg.Enabled &&
		!s.conf.Metrics.SnowflakeQueryBytesDeletedAvg.Enabled && !s.conf.Metrics.SnowflakeQueryBytesSpilledLocalAvg.Enabled &&
		!s.conf.Metrics.SnowflakeQueryBytesSpilledRemoteAvg.Enabled && !s.conf.Metrics.SnowflakeQueryBytesWrittenAvg.Enabled &&
		!s.conf.Metrics.SnowflakeQueryCompilationTimeAvg.Enabled && !s.conf.Metrics.SnowflakeQueryDataScannedCacheAvg.Enabled &&
		!s.conf.Metrics.SnowflakeQueryExecutionTimeAvg.Enabled && !s.conf.Metrics.SnowflakeQueryPartitionsScannedAvg.Enabled &&
		!s.conf.Metrics.SnowflakeQueuedOverloadTimeAvg.Enabled && !s.conf.Metrics.SnowflakeQueuedProvisioningTimeAvg.Enabled &&
		!s.conf.Metrics.SnowflakeQueuedRepairTimeAvg.Enabled && !s.conf.Metrics.SnowflakeRowsInsertedAvg.Enabled &&
		!s.conf.Metrics.SnowflakeRowsDeletedAvg.Enabled && !s.conf.Metrics.SnowflakeRowsProducedAvg.Enabled &&
		!s.conf.Metrics.SnowflakeRowsUnloadedAvg.Enabled && !s.conf.Metrics.SnowflakeRowsUpdatedAvg.Enabled &&
		!s.conf.Metrics.SnowflakeTotalElapsedTimeAvg.Enabled {
		return
	}
	DBMetrics, err := s.client.FetchDbMetrics(ctx)
	if err != nil {
		errs <- err
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

func (s *snowflakeMetricsScraper) scrapeSessionMetrics(ctx context.Context, t pcommon.Timestamp, errs chan<- error) {
	if !s.conf.Metrics.SnowflakeSessionIDCount.Enabled {
		return
	}

	sessionMetrics, err := s.client.FetchSessionMetrics(ctx)
	if err != nil {
		errs <- err
		return
	}

	for _, row := range *sessionMetrics {
		s.mb.RecordSnowflakeSessionIDCountDataPoint(t, row.distinctSessionID, row.userName.String)
	}
}

func (s *snowflakeMetricsScraper) scrapeSnowpipeMetrics(ctx context.Context, t pcommon.Timestamp, errs chan<- error) {
	if !s.conf.Metrics.SnowflakePipeCreditsUsedTotal.Enabled {
		return
	}

	snowpipeMetrics, err := s.client.FetchSnowpipeMetrics(ctx)
	if err != nil {
		errs <- err
		return
	}

	for _, row := range *snowpipeMetrics {
		s.mb.RecordSnowflakePipeCreditsUsedTotalDataPoint(t, row.creditsUsed, row.pipeName.String)
	}
}

func (s *snowflakeMetricsScraper) scrapeStorageMetrics(ctx context.Context, t pcommon.Timestamp, errs chan<- error) {
	if !s.conf.Metrics.SnowflakeStorageStorageBytesTotal.Enabled && !s.conf.Metrics.SnowflakeStorageStageBytesTotal.Enabled && !s.conf.Metrics.SnowflakeStorageFailsafeBytesTotal.Enabled {
		return
	}

	storageMetrics, err := s.client.FetchStorageMetrics(ctx)
	if err != nil {
		errs <- err
		return
	}

	for _, row := range *storageMetrics {
		s.mb.RecordSnowflakeStorageStorageBytesTotalDataPoint(t, row.storageBytes)
		s.mb.RecordSnowflakeStorageStageBytesTotalDataPoint(t, row.stageBytes)
		s.mb.RecordSnowflakeStorageFailsafeBytesTotalDataPoint(t, row.failsafeBytes)
	}
}
