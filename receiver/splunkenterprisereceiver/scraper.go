// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splunkenterprisereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/splunkenterprisereceiver"

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/splunkenterprisereceiver/internal/metadata"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scrapererror"
)

var (
    errMaxSearchWaitTimeExceeded = errors.New("Maximum search wait time exceeded for metric")
)

type splunkScraper struct {
    splunkClient *splunkEntClient
    settings     component.TelemetrySettings
    conf         *Config
    mb           *metadata.MetricsBuilder
    mfg          metadata.MetricsConfig
}

func newSplunkMetricsScraper(params receiver.CreateSettings, cfg *Config) splunkScraper {
    return splunkScraper{
        settings: params.TelemetrySettings,
        conf:     cfg,
        mb:       metadata.NewMetricsBuilder(cfg.MetricsBuilderConfig, params),
        mfg:      metadata.DefaultMetricsConfig(),
    }
}

// Create a client instance and add to the splunkScraper
func (s *splunkScraper) start(_ context.Context, _ component.Host) (err error) {
    c := newSplunkEntClient(s.conf) 
    s.splunkClient = &c
    return nil 
}

// The big one: Describes how all scraping tasks should be performed. Part of the scraper interface
func (s *splunkScraper) scrape(ctx context.Context) (pmetric.Metrics, error) {
    errs := &scrapererror.ScrapeErrors{}
    now := pcommon.NewTimestampFromTime(time.Now())

    s.scrapeLicenseUsageByIndex(ctx, now, errs)
    return s.mb.Emit(), errs.Combine()
}

// Each metric has its own scrape function associated with it
func (s *splunkScraper) scrapeLicenseUsageByIndex(ctx context.Context, now pcommon.Timestamp, errs *scrapererror.ScrapeErrors) {
    var sr searchResponse
    // Because we have to utilize network resources for each KPI we should check that each metrics
    // is enabled before proceeding
    if s.mfg.SplunkLicenseIndexUsage.Enabled {
        sr = searchResponse{
            search: searchDict[`SplunkLicenseIndexUsageSearch`],
        }
    } else {
        return
    }

    start := time.Now()
    _, err := s.splunkClient.makeRequest(&sr)
    if err != nil {
        errs.Add(err)
    }
    for ok := true; ok; ok = (sr.Return == 204) {
        _, err := s.splunkClient.makeRequest(&sr)
        if err != nil {
            errs.Add(err)
        }
        time.Sleep(2 * time.Second)
        if time.Since(start) > s.conf.MaxSearchWaitTime {
            errs.Add(errMaxSearchWaitTimeExceeded)
            return
        }
    }

    // Record the results
    var indexName string
    for _, f := range sr.Fields {
        switch fieldName := f.FieldName; fieldName {
        case "indexname":
            indexName = f.Value
            continue
        case "GB":
            v, err := strconv.ParseFloat(f.Value, 64)
            if err != nil {
                errs.Add(err)
                continue
            }
            s.mb.RecordSplunkLicenseIndexUsageDataPoint(now, v, indexName)
        }
    }
}

func (s *splunkScraper) scrapeAverageDiskIO(ctx context.Context, now pcommon.Timestamp, errs *scrapererror.ScrapeErrors) {
    var sr searchResponse

    if s.mfg.SplunkLicenseIndexUsage.Enabled {
        sr = searchResponse{
            search: searchDict[`SplunkLicenseIndexUsageSearch`],
        }
    } else {
        return
    }

    start := time.Now()
    _, err := s.splunkClient.makeRequest(&sr)
    if err != nil {
        errs.Add(err)
    }
    for ok := true; ok; ok = (sr.Return == 204) {
        _, err := s.splunkClient.makeRequest(&sr)
        if err != nil {
            errs.Add(err)
        }
        time.Sleep(2 * time.Second)
        if time.Since(start) > s.conf.MaxSearchWaitTime {
            errs.Add(errMaxSearchWaitTimeExceeded)
            return
        }
    }
}
