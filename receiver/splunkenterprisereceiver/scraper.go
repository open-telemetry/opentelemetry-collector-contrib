// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splunkenterprisereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/splunkenterprisereceiver"

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/scrapererror"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/splunkenterprisereceiver/internal/metadata"
)

var (
	errMaxSearchWaitTimeExceeded = errors.New("maximum search wait time exceeded for metric")
)

type splunkScraper struct {
	splunkClient *splunkEntClient
	settings     component.TelemetrySettings
	conf         *Config
	mb           *metadata.MetricsBuilder
}

func newSplunkMetricsScraper(params receiver.CreateSettings, cfg *Config) splunkScraper {
	return splunkScraper{
		settings: params.TelemetrySettings,
		conf:     cfg,
		mb:       metadata.NewMetricsBuilder(cfg.MetricsBuilderConfig, params),
	}
}

// Create a client instance and add to the splunkScraper
func (s *splunkScraper) start(_ context.Context, h component.Host) (err error) {
	client, err := newSplunkEntClient(s.conf, h, s.settings)
	if err != nil {
		return err
	}
	s.splunkClient = client
	return nil
}

// The big one: Describes how all scraping tasks should be performed. Part of the scraper interface
func (s *splunkScraper) scrape(ctx context.Context) (pmetric.Metrics, error) {
	errs := &scrapererror.ScrapeErrors{}
	now := pcommon.NewTimestampFromTime(time.Now())

	s.scrapeLicenseUsageByIndex(ctx, now, errs)
	s.scrapeIndexThroughput(ctx, now, errs)
	s.scrapeIndexesTotalSize(ctx, now, errs)
	s.scrapeIndexesEventCount(ctx, now, errs)
	s.scrapeIndexesBucketCount(ctx, now, errs)
	s.scrapeIndexesRawSize(ctx, now, errs)
	s.scrapeIndexesBucketEventCount(ctx, now, errs)
	s.scrapeIndexesBucketHotWarmCount(ctx, now, errs)
	s.scrapeIntrospectionQueues(ctx, now, errs)
	s.scrapeIntrospectionQueuesBytes(ctx, now, errs)
	return s.mb.Emit(), errs.Combine()
}

// Each metric has its own scrape function associated with it
func (s *splunkScraper) scrapeLicenseUsageByIndex(ctx context.Context, now pcommon.Timestamp, errs *scrapererror.ScrapeErrors) {
	var sr searchResponse
	// Because we have to utilize network resources for each KPI we should check that each metrics
	// is enabled before proceeding
	if !s.conf.MetricsBuilderConfig.Metrics.SplunkLicenseIndexUsage.Enabled {
		return
	}

	sr = searchResponse{
		search: searchDict[`SplunkLicenseIndexUsageSearch`],
	}

	var (
		req *http.Request
		res *http.Response
		err error
	)

	start := time.Now()

	for {
		req, err = s.splunkClient.createRequest(ctx, &sr)
		if err != nil {
			errs.Add(err)
			return
		}

		res, err = s.splunkClient.makeRequest(req)
		if err != nil {
			errs.Add(err)
			return
		}

		// if its a 204 the body will be empty because we are still waiting on search results
		err = unmarshallSearchReq(res, &sr)
		if err != nil {
			errs.Add(err)
		}
		res.Body.Close()

		// if no errors and 200 returned scrape was successful, return. Note we must make sure that
		// the 200 is coming after the first request which provides a jobId to retrieve results
		if sr.Return == 200 && sr.Jobid != nil {
			break
		}

		if sr.Return == 204 {
			time.Sleep(2 * time.Second)
		}

		if time.Since(start) > s.conf.ScraperControllerSettings.Timeout {
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
		case "By":
			v, err := strconv.ParseFloat(f.Value, 64)
			if err != nil {
				errs.Add(err)
				continue
			}
			s.mb.RecordSplunkLicenseIndexUsageDataPoint(now, int64(v), indexName)
		}
	}
}

// Helper function for unmarshaling search endpoint requests
func unmarshallSearchReq(res *http.Response, sr *searchResponse) error {
	sr.Return = res.StatusCode

	if res.ContentLength == 0 {
		return nil
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("Failed to read response: %w", err)
	}

	err = xml.Unmarshal(body, &sr)
	if err != nil {
		return fmt.Errorf("Failed to unmarshall response: %w", err)
	}

	return nil
}

// Scrape index throughput introspection endpoint
func (s *splunkScraper) scrapeIndexThroughput(ctx context.Context, now pcommon.Timestamp, errs *scrapererror.ScrapeErrors) {
	var it indexThroughput
	var ept string

	if !s.conf.MetricsBuilderConfig.Metrics.SplunkIndexerThroughput.Enabled {
		return
	}

	ept = apiDict[`SplunkIndexerThroughput`]

	req, err := s.splunkClient.createAPIRequest(ctx, ept)
	if err != nil {
		errs.Add(err)
		return
	}

	res, err := s.splunkClient.makeRequest(req)
	if err != nil {
		errs.Add(err)
		return
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		errs.Add(err)
		return
	}

	err = json.Unmarshal(body, &it)
	if err != nil {
		errs.Add(err)
		return
	}

	for _, entry := range it.Entries {
		s.mb.RecordSplunkIndexerThroughputDataPoint(now, 1000*entry.Content.AvgKb, entry.Content.Status)
	}
}

// Scrape indexes extended total size
func (s *splunkScraper) scrapeIndexesTotalSize(ctx context.Context, now pcommon.Timestamp, errs *scrapererror.ScrapeErrors) {
	var it IndexesExtended
	var ept string

	if !s.conf.MetricsBuilderConfig.Metrics.SplunkDataIndexesExtendedTotalSize.Enabled {
		return
	}

	ept = apiDict[`SplunkDataIndexesExtended`]

	req, err := s.splunkClient.createAPIRequest(ctx, ept)
	if err != nil {
		errs.Add(err)
		return
	}

	res, err := s.splunkClient.makeRequest(req)
	if err != nil {
		errs.Add(err)
		return
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		errs.Add(err)
		return
	}

	err = json.Unmarshal(body, &it)
	if err != nil {
		errs.Add(err)
		return
	}

	var name string
	var totalSize int64
	for _, f := range it.Entries {
		if f.Name != "" {
			name = f.Name
		}
		if f.Content.TotalSize != "" {
			mb, err := strconv.ParseFloat(f.Content.TotalSize, 64)
			totalSize = int64(mb * 1024 * 1024)
			if err != nil {
				errs.Add(err)
			}
		}

		s.mb.RecordSplunkDataIndexesExtendedTotalSizeDataPoint(now, totalSize, name)
	}
}

// Scrape indexes extended total event count
func (s *splunkScraper) scrapeIndexesEventCount(ctx context.Context, now pcommon.Timestamp, errs *scrapererror.ScrapeErrors) {
	var it IndexesExtended
	var ept string

	if !s.conf.MetricsBuilderConfig.Metrics.SplunkDataIndexesExtendedEventCount.Enabled {
		return
	}

	ept = apiDict[`SplunkDataIndexesExtended`]

	req, err := s.splunkClient.createAPIRequest(ctx, ept)
	if err != nil {
		errs.Add(err)
		return
	}

	res, err := s.splunkClient.makeRequest(req)
	if err != nil {
		errs.Add(err)
		return
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		errs.Add(err)
		return
	}

	err = json.Unmarshal(body, &it)
	if err != nil {
		errs.Add(err)
		return
	}

	var name string
	for _, f := range it.Entries {
		if f.Name != "" {
			name = f.Name
		}
		totalEventCount := int64(f.Content.TotalEventCount)

		s.mb.RecordSplunkDataIndexesExtendedEventCountDataPoint(now, totalEventCount, name)
	}
}

// Scrape indexes extended total bucket count
func (s *splunkScraper) scrapeIndexesBucketCount(ctx context.Context, now pcommon.Timestamp, errs *scrapererror.ScrapeErrors) {
	var it IndexesExtended
	var ept string

	if !s.conf.MetricsBuilderConfig.Metrics.SplunkDataIndexesExtendedBucketCount.Enabled {
		return
	}

	ept = apiDict[`SplunkDataIndexesExtended`]

	req, err := s.splunkClient.createAPIRequest(ctx, ept)
	if err != nil {
		errs.Add(err)
		return
	}

	res, err := s.splunkClient.makeRequest(req)
	if err != nil {
		errs.Add(err)
		return
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		errs.Add(err)
		return
	}

	err = json.Unmarshal(body, &it)
	if err != nil {
		errs.Add(err)
		return
	}

	var name string
	var totalBucketCount int64
	for _, f := range it.Entries {
		if f.Name != "" {
			name = f.Name
		}
		if f.Content.TotalBucketCount != "" {
			totalBucketCount, err = strconv.ParseInt(f.Content.TotalBucketCount, 10, 64)
			if err != nil {
				errs.Add(err)
			}
		}

		s.mb.RecordSplunkDataIndexesExtendedBucketCountDataPoint(now, totalBucketCount, name)
	}
}

// Scrape indexes extended raw size
func (s *splunkScraper) scrapeIndexesRawSize(ctx context.Context, now pcommon.Timestamp, errs *scrapererror.ScrapeErrors) {
	var it IndexesExtended
	var ept string

	if !s.conf.MetricsBuilderConfig.Metrics.SplunkDataIndexesExtendedRawSize.Enabled {
		return
	}

	ept = apiDict[`SplunkDataIndexesExtended`]

	req, err := s.splunkClient.createAPIRequest(ctx, ept)
	if err != nil {
		errs.Add(err)
		return
	}

	res, err := s.splunkClient.makeRequest(req)
	if err != nil {
		errs.Add(err)
		return
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		errs.Add(err)
		return
	}

	err = json.Unmarshal(body, &it)
	if err != nil {
		errs.Add(err)
		return
	}

	var name string
	var totalRawSize int64
	for _, f := range it.Entries {
		if f.Name != "" {
			name = f.Name
		}
		if f.Content.TotalRawSize != "" {
			mb, err := strconv.ParseFloat(f.Content.TotalRawSize, 64)
			totalRawSize = int64(mb * 1024 * 1024)
			if err != nil {
				errs.Add(err)
			}
		}
		s.mb.RecordSplunkDataIndexesExtendedRawSizeDataPoint(now, totalRawSize, name)
	}
}

// Scrape indexes extended bucket event count
func (s *splunkScraper) scrapeIndexesBucketEventCount(ctx context.Context, now pcommon.Timestamp, errs *scrapererror.ScrapeErrors) {
	var it IndexesExtended
	var ept string

	if !s.conf.MetricsBuilderConfig.Metrics.SplunkDataIndexesExtendedBucketEventCount.Enabled {
		return
	}

	ept = apiDict[`SplunkDataIndexesExtended`]

	req, err := s.splunkClient.createAPIRequest(ctx, ept)
	if err != nil {
		errs.Add(err)
		return
	}

	res, err := s.splunkClient.makeRequest(req)
	if err != nil {
		errs.Add(err)
		return
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		errs.Add(err)
		return
	}

	err = json.Unmarshal(body, &it)
	if err != nil {
		errs.Add(err)
		return
	}

	var name string
	var bucketDir string
	var bucketEventCount int64
	for _, f := range it.Entries {
		if f.Name != "" {
			name = f.Name
		}
		if f.Content.BucketDirs.Cold.EventCount != "" {
			bucketDir = "cold"
			bucketEventCount, err = strconv.ParseInt(f.Content.BucketDirs.Cold.EventCount, 10, 64)
			if err != nil {
				errs.Add(err)
			}
			s.mb.RecordSplunkDataIndexesExtendedBucketEventCountDataPoint(now, bucketEventCount, name, bucketDir)
		}
		if f.Content.BucketDirs.Home.EventCount != "" {
			bucketDir = "home"
			bucketEventCount, err = strconv.ParseInt(f.Content.BucketDirs.Home.EventCount, 10, 64)
			if err != nil {
				errs.Add(err)
			}
			s.mb.RecordSplunkDataIndexesExtendedBucketEventCountDataPoint(now, bucketEventCount, name, bucketDir)
		}
		if f.Content.BucketDirs.Thawed.EventCount != "" {
			bucketDir = "thawed"
			bucketEventCount, err = strconv.ParseInt(f.Content.BucketDirs.Thawed.EventCount, 10, 64)
			if err != nil {
				errs.Add(err)
			}
			s.mb.RecordSplunkDataIndexesExtendedBucketEventCountDataPoint(now, bucketEventCount, name, bucketDir)
		}
	}
}

// Scrape indexes extended bucket hot/warm count
func (s *splunkScraper) scrapeIndexesBucketHotWarmCount(ctx context.Context, now pcommon.Timestamp, errs *scrapererror.ScrapeErrors) {
	var it IndexesExtended
	var ept string

	if !s.conf.MetricsBuilderConfig.Metrics.SplunkDataIndexesExtendedBucketHotCount.Enabled {
		return
	}

	ept = apiDict[`SplunkDataIndexesExtended`]

	req, err := s.splunkClient.createAPIRequest(ctx, ept)
	if err != nil {
		errs.Add(err)
		return
	}

	res, err := s.splunkClient.makeRequest(req)
	if err != nil {
		errs.Add(err)
		return
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		errs.Add(err)
		return
	}

	err = json.Unmarshal(body, &it)
	if err != nil {
		errs.Add(err)
		return
	}

	var name string
	var bucketDir string
	var bucketHotCount int64
	var bucketWarmCount int64
	for _, f := range it.Entries {
		if f.Name != "" {
			name = f.Name
		}
		if f.Content.BucketDirs.Home.HotBucketCount != "" {
			bucketHotCount, err = strconv.ParseInt(f.Content.BucketDirs.Home.HotBucketCount, 10, 64)
			bucketDir = "hot"
			if err != nil {
				errs.Add(err)
			}
			s.mb.RecordSplunkDataIndexesExtendedBucketHotCountDataPoint(now, bucketHotCount, name, bucketDir)
		}
		if f.Content.BucketDirs.Home.WarmBucketCount != "" {
			bucketWarmCount, err = strconv.ParseInt(f.Content.BucketDirs.Home.WarmBucketCount, 10, 64)
			bucketDir = "warm"
			if err != nil {
				errs.Add(err)
			}
			s.mb.RecordSplunkDataIndexesExtendedBucketWarmCountDataPoint(now, bucketWarmCount, name, bucketDir)
		}
	}
}

// Scrape introspection queues
func (s *splunkScraper) scrapeIntrospectionQueues(ctx context.Context, now pcommon.Timestamp, errs *scrapererror.ScrapeErrors) {
	var it IntrospectionQueues
	var ept string

	if !s.conf.MetricsBuilderConfig.Metrics.SplunkServerIntrospectionQueuesCurrent.Enabled {
		return
	}

	ept = apiDict[`SplunkIntrospectionQueues`]

	req, err := s.splunkClient.createAPIRequest(ctx, ept)
	if err != nil {
		errs.Add(err)
		return
	}

	res, err := s.splunkClient.makeRequest(req)
	if err != nil {
		errs.Add(err)
		return
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		errs.Add(err)
		return
	}

	err = json.Unmarshal(body, &it)
	if err != nil {
		errs.Add(err)
		return
	}

	var name string
	for _, f := range it.Entries {
		if f.Name != "" {
			name = f.Name
		}

		currentQueuesSize := int64(f.Content.CurrentSize)

		s.mb.RecordSplunkServerIntrospectionQueuesCurrentDataPoint(now, currentQueuesSize, name)
	}
}

// Scrape introspection queues bytes
func (s *splunkScraper) scrapeIntrospectionQueuesBytes(ctx context.Context, now pcommon.Timestamp, errs *scrapererror.ScrapeErrors) {
	var it IntrospectionQueues
	var ept string

	if !s.conf.MetricsBuilderConfig.Metrics.SplunkServerIntrospectionQueuesCurrentBytes.Enabled {
		return
	}

	ept = apiDict[`SplunkIntrospectionQueues`]

	req, err := s.splunkClient.createAPIRequest(ctx, ept)
	if err != nil {
		errs.Add(err)
		return
	}

	res, err := s.splunkClient.makeRequest(req)
	if err != nil {
		errs.Add(err)
		return
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		errs.Add(err)
		return
	}

	err = json.Unmarshal(body, &it)
	if err != nil {
		errs.Add(err)
		return
	}
	var name string
	for _, f := range it.Entries {
		if f.Name != "" {
			name = f.Name
		}

		currentQueueSizeBytes := int64(f.Content.CurrentSizeBytes)

		s.mb.RecordSplunkServerIntrospectionQueuesCurrentBytesDataPoint(now, currentQueueSizeBytes, name)
	}
}
