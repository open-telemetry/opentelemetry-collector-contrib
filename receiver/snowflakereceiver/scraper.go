// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the License);
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an AS IS BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package snowflakereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/snowflakereceiver"

import (
	"context"
	"fmt"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/snowflakereceiver/internal/metadata"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scrapererror"
)

type snowflakeMetricsScraper struct {
    client   *snowflakeClient
    settings component.TelemetrySettings
    conf     *Config
    mb       *metadata.MetricsBuilder 
}

func newSnowflakeMetricsScraper(settings receiver.CreateSettings, conf *Config) (*snowflakeMetricsScraper) {
    return &snowflakeMetricsScraper{
        settings: settings.TelemetrySettings,
        conf: conf,
        mb: metadata.NewMetricsBuilder(conf.Metrics, settings.BuildInfo),
    }
}

// for use with receiver.scraperhelper
func (s *snowflakeMetricsScraper) start(_ context.Context, _ component.Host) (err error){
    s.client, err = newDefaultClient(s.settings, *s.conf)
    if err != nil {
        return err 
    }
    return nil
}

// wrapper for all of the sub-scraping tasks, implements the scraper interface for 
// snowflakeMetricsScraper
func (s *snowflakeMetricsScraper) scrape(ctx context.Context) (pmetric.Metrics, error) {
    errs := &scrapererror.ScrapeErrors{}

    now := pcommon.NewTimestampFromTime(time.Now())

    // each client call has its own scrape function

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

        s.mb.EmitForResource(
            metadata.WithSnowflakeAccountName(s.conf.Account),
            metadata.WithSnowflakeUsername(s.conf.Username),
            metadata.WithSnowflakeWarehouseName(s.conf.Warehouse),
            )
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

        s.mb.EmitForResource(
            metadata.WithSnowflakeAccountName(s.conf.Account),
            metadata.WithSnowflakeUsername(s.conf.Username),
            metadata.WithSnowflakeWarehouseName(s.conf.Warehouse),
            )

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

        s.mb.EmitForResource(
            metadata.WithSnowflakeAccountName(s.conf.Account),
            metadata.WithSnowflakeUsername(s.conf.Username),
            metadata.WithSnowflakeWarehouseName(s.conf.Warehouse),
            )
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

        s.mb.EmitForResource(
            metadata.WithSnowflakeAccountName(s.conf.Account),
            metadata.WithSnowflakeUsername(s.conf.Username),
            metadata.WithSnowflakeWarehouseName(s.conf.Warehouse),
            )
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
        s.mb.RecordSnowflakeQueryBytesDeletedTotalDataPoint(t, row.avgBytesDeleted, row.attributes.schemaName.String, row.attributes.executionStatus.String, row.attributes.errorMessage.String, row.attributes.queryType.String, row.attributes.warehouseName.String, row.attributes.databaseName.String, row.attributes.warehouseSize.String)

        s.mb.EmitForResource(
            metadata.WithSnowflakeAccountName(s.conf.Account),
            metadata.WithSnowflakeUsername(s.conf.Username),
            metadata.WithSnowflakeWarehouseName(s.conf.Warehouse),
            )
    }
}

func (s *snowflakeMetricsScraper) scrapeSessionMetrics(ctx context.Context, t pcommon.Timestamp, errs scrapererror.ScrapeErrors) {
    sessionMetrics, err := s.client.FetchSessionMetrics(ctx)
    
    if err != nil {
        errs.Add(err)
        return
    }

    for _, row := range *sessionMetrics {

        s.mb.EmitForResource(
            metadata.WithSnowflakeAccountName(s.conf.Account),
            metadata.WithSnowflakeUsername(s.conf.Username),
            metadata.WithSnowflakeWarehouseName(s.conf.Warehouse),
            )
    }
}

func (s *snowflakeMetricsScraper) scrapeSnowpipeMetrics(ctx context.Context, t pcommon.Timestamp, errs scrapererror.ScrapeErrors) {
    snowpipeMetrics, err := s.client.FetchSnowpipeMetrics(ctx)

    if err != nil {
        errs.Add(err)
        return
    }

    for _, row := range *snowpipeMetrics {

        s.mb.EmitForResource(
            metadata.WithSnowflakeAccountName(s.conf.Account),
            metadata.WithSnowflakeUsername(s.conf.Username),
            metadata.WithSnowflakeWarehouseName(s.conf.Warehouse),
            )
    }
}

func (s *snowflakeMetricsScraper) scrapeStorageMetrics(ctx context.Context, t pcommon.Timestamp, errs scrapererror.ScrapeErrors) {
    storageMetrics, err := s.client.FetchStorageMetrics(ctx)

    if err != nil {
        errs.Add(err)
        return
    }

    for _, row := range *storageMetrics {

        s.mb.EmitForResource(
            metadata.WithSnowflakeAccountName(s.conf.Account),
            metadata.WithSnowflakeUsername(s.conf.Username),
            metadata.WithSnowflakeWarehouseName(s.conf.Warehouse),
            )
    }
}
