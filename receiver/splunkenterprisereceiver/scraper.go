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
	errMaxSearchWaitTimeExceeded = errors.New("Maximum search wait time exceeded for metric")
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
	s.scrapeIndexThroughput(ctx, now, errs)
	return s.mb.Emit(), errs.Combine()
}

// Each metric has its own scrape function associated with it
func (s *splunkScraper) scrapeLicenseUsageByIndex(ctx context.Context, now pcommon.Timestamp, errs *scrapererror.ScrapeErrors) {
	var sr searchResponse
	// Because we have to utilize network resources for each KPI we should check that each metrics
	// is enabled before proceeding
	if !s.conf.MetricsBuilderConfig.Metrics.SplunkLicenseIndexUsage.Enabled {
		return
	} else {
		sr = searchResponse{
			search: searchDict[`SplunkLicenseIndexUsageSearch`],
		}
	}

	start := time.Now()
	req, err := s.splunkClient.createRequest(ctx, &sr)
	if err != nil {
		errs.Add(err)
	}

	res, err := s.splunkClient.makeRequest(req)
	if err != nil {
		errs.Add(err)
	}
	defer res.Body.Close()

	err = unmarshallSearchReq(res, &sr)
	if err != nil {
		errs.Add(err)
	}

	for ok := true; ok; ok = (sr.Return == 204) {
		req, err = s.splunkClient.createRequest(ctx, &sr)
		if err != nil {
			errs.Add(err)
		}

		res, err = s.splunkClient.makeRequest(req)
		if err != nil {
			errs.Add(err)
		}
		defer res.Body.Close()

		// if its a 204 the body will be empty because we are still waiting on search results

		err = unmarshallSearchReq(res, &sr)
		if err != nil {
			errs.Add(err)
			return
		}

		if sr.Return == 204 {
			time.Sleep(2 * time.Second)
			continue
		}

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

	if !s.conf.MetricsBuilderConfig.Metrics.SplunkServerIntrospectionIndexerThroughput.Enabled {
		return
	} else {
		ept = apiDict[`SplunkIndexerThroughput`]
	}

	req, err := s.splunkClient.createAPIRequest(ctx, ept)
	if err != nil {
		errs.Add(err)
	}

	res, err := s.splunkClient.makeRequest(req)
	if err != nil {
		errs.Add(err)
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		fmt.Println("Failed to read response")
		errs.Add(err)
	}

	err = json.Unmarshal(body, &it)
	if err != nil {
		errs.Add(err)
	}

	fmt.Printf("\n%v\n", it.Entries)

	s.mb.RecordSplunkServerIntrospectionIndexerThroughputDataPoint(now, it.Entries[0].Content.AvgKb, it.Entries[0].Content.Status)
}
