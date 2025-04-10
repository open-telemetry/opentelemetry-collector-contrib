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
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/scraper/scrapererror"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/splunkenterprisereceiver/internal/metadata"
)

var errMaxSearchWaitTimeExceeded = errors.New("maximum search wait time exceeded for metric")

type splunkScraper struct {
	splunkClient *splunkEntClient
	settings     component.TelemetrySettings
	conf         *Config
	mb           *metadata.MetricsBuilder
}

func newSplunkMetricsScraper(params receiver.Settings, cfg *Config) splunkScraper {
	return splunkScraper{
		settings: params.TelemetrySettings,
		conf:     cfg,
		mb:       metadata.NewMetricsBuilder(cfg.MetricsBuilderConfig, params),
	}
}

// Create a client instance and add to the splunkScraper
func (s *splunkScraper) start(ctx context.Context, h component.Host) (err error) {
	client, err := newSplunkEntClient(ctx, s.conf, h, s.settings)
	if err != nil {
		return err
	}
	s.splunkClient = client
	return nil
}

// listens to the error channel and combines errors sent from different metric scrape functions,
// returning the combined error list should context timeout or a nil error value is sent in the
// channel signifying the end of a scrape cycle
func errorListener(ctx context.Context, eQueue <-chan error, eOut chan<- *scrapererror.ScrapeErrors) {
	errs := &scrapererror.ScrapeErrors{}

	for {
		select {
		case <-ctx.Done():
			eOut <- errs
			return
		case err, ok := <-eQueue:
			if !ok {
				eOut <- errs
				return
			}
			errs.Add(err)
		}
	}
}

// The big one: Describes how all scraping tasks should be performed. Part of the scraper interface
func (s *splunkScraper) scrape(ctx context.Context) (pmetric.Metrics, error) {
	var wg sync.WaitGroup
	var errs *scrapererror.ScrapeErrors

	now := pcommon.NewTimestampFromTime(time.Now())
	errOut := make(chan *scrapererror.ScrapeErrors)
	metricScrapes := []func(context.Context, pcommon.Timestamp, infoDict, chan error){
		s.scrapeLicenseUsageByIndex,
		s.scrapeIndexThroughput,
		s.scrapeIndexesTotalSize,
		s.scrapeIndexesEventCount,
		s.scrapeIndexesBucketCount,
		s.scrapeIndexesRawSize,
		s.scrapeIndexesBucketEventCount,
		s.scrapeIndexesBucketHotWarmCount,
		s.scrapeIntrospectionQueues,
		s.scrapeIntrospectionQueuesBytes,
		s.scrapeAvgExecLatencyByHost,
		s.scrapeIndexerPipelineQueues,
		s.scrapeBucketsSearchableStatus,
		s.scrapeIndexesBucketCountAdHoc,
		s.scrapeSchedulerCompletionRatioByHost,
		s.scrapeIndexerRawWriteSecondsByHost,
		s.scrapeIndexerCPUSecondsByHost,
		s.scrapeAvgIopsByHost,
		s.scrapeSchedulerRunTimeByHost,
		s.scrapeIndexerAvgRate,
		s.scrapeKVStoreStatus,
		s.scrapeSearchArtifacts,
		s.scrapeHealth,
	}
	errChan := make(chan error, len(metricScrapes))

	go func() {
		errorListener(ctx, errChan, errOut)
	}()

	// if the build and version info has been configured that is pulled here
	var info infoDict
	if s.conf.VersionInfo {
		info = s.scrapeInfo(ctx, now, errChan)
	} else {
		info = make(infoDict)
		nullInfo := Info{Host: "", Entries: make([]InfoEntry, 1)}
		info[typeCm] = nullInfo
		info[typeSh] = nullInfo
		info[typeIdx] = nullInfo
	}

	for _, fn := range metricScrapes {
		wg.Add(1)
		go func(
			fn func(ctx context.Context, now pcommon.Timestamp, info infoDict, errs chan error),
			ctx context.Context,
			now pcommon.Timestamp,
			info infoDict,
			errs chan error,
		) {
			// actual function body
			defer wg.Done()
			fn(ctx, now, info, errs)
		}(fn, ctx, now, info, errChan)
	}

	wg.Wait()
	close(errChan)
	errs = <-errOut
	return s.mb.Emit(), errs.Combine()
}

// Each metric has its own scrape function associated with it
func (s *splunkScraper) scrapeLicenseUsageByIndex(_ context.Context, now pcommon.Timestamp, info infoDict, errs chan error) {
	// Because we have to utilize network resources for each KPI we should check that each metrics
	// is enabled before proceeding
	if !s.conf.Metrics.SplunkLicenseIndexUsage.Enabled || !s.splunkClient.isConfigured(typeCm) {
		return
	}
	i := info[typeCm].Entries[0].Content

	sr := searchResponse{
		search: searchDict[`SplunkLicenseIndexUsageSearch`],
	}

	var (
		req *http.Request
		res *http.Response
		err error
	)

	start := time.Now()

	for {
		req, err = s.splunkClient.createRequest(typeCm, &sr)
		if err != nil {
			errs <- err
			return
		}

		res, err = s.splunkClient.makeRequest(req)
		if err != nil {
			errs <- err
			return
		}

		// if its a 204 the body will be empty because we are still waiting on search results
		err = unmarshallSearchReq(res, &sr)
		if err != nil {
			errs <- err
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

		if time.Since(start) > s.conf.Timeout {
			errs <- errMaxSearchWaitTimeExceeded
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
				errs <- err
				continue
			}
			s.mb.RecordSplunkLicenseIndexUsageDataPoint(now, int64(v), indexName, i.Build, i.Version)
		}
	}
}

func (s *splunkScraper) scrapeAvgExecLatencyByHost(_ context.Context, now pcommon.Timestamp, info infoDict, errs chan error) {
	// Because we have to utilize network resources for each KPI we should check that each metrics
	// is enabled before proceeding
	if !s.conf.Metrics.SplunkSchedulerAvgExecutionLatency.Enabled {
		return
	}
	i := info[typeCm].Entries[0].Content

	sr := searchResponse{
		search: searchDict[`SplunkSchedulerAvgExecLatencySearch`],
	}

	var (
		req *http.Request
		res *http.Response
		err error
	)

	start := time.Now()

	for {
		req, err = s.splunkClient.createRequest(typeCm, &sr)
		if err != nil {
			errs <- err
			return
		}

		res, err = s.splunkClient.makeRequest(req)
		if err != nil {
			errs <- err
			return
		}

		// if its a 204 the body will be empty because we are still waiting on search results
		err = unmarshallSearchReq(res, &sr)
		if err != nil {
			errs <- err
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

		if sr.Return == 400 {
			break
		}

		if time.Since(start) > s.conf.Timeout {
			errs <- errMaxSearchWaitTimeExceeded
			return
		}
	}

	// Record the results
	var host string
	for _, f := range sr.Fields {
		switch fieldName := f.FieldName; fieldName {
		case "host":
			host = f.Value
			continue
		case "latency_avg_exec":
			v, err := strconv.ParseFloat(f.Value, 64)
			if err != nil {
				errs <- err
				continue
			}
			s.mb.RecordSplunkSchedulerAvgExecutionLatencyDataPoint(now, v, host, i.Build, i.Version)
		}
	}
}

func (s *splunkScraper) scrapeIndexerAvgRate(_ context.Context, now pcommon.Timestamp, info infoDict, errs chan error) {
	// Because we have to utilize network resources for each KPI we should check that each metrics
	// is enabled before proceeding
	if !s.conf.Metrics.SplunkIndexerAvgRate.Enabled {
		return
	}
	i := info[typeCm].Entries[0].Content

	sr := searchResponse{
		search: searchDict[`SplunkIndexerAvgRate`],
	}

	var (
		req *http.Request
		res *http.Response
		err error
	)

	start := time.Now()

	for {
		req, err = s.splunkClient.createRequest(typeCm, &sr)
		if err != nil {
			errs <- err
			return
		}

		res, err = s.splunkClient.makeRequest(req)
		if err != nil {
			errs <- err
			return
		}

		// if its a 204 the body will be empty because we are still waiting on search results
		err = unmarshallSearchReq(res, &sr)
		if err != nil {
			errs <- err
		}
		res.Body.Close()

		// if no errors and 200 returned scrape was successful, return. Note we must make sure that
		// the 200 is coming after the first request which provides a jobId to retrieve results
		if sr.Return == 200 && sr.Jobid != nil {
			break
		}

		if sr.Return == 200 {
			break
		}

		if sr.Return == 204 {
			time.Sleep(2 * time.Second)
		}

		if sr.Return == 400 {
			break
		}

		if time.Since(start) > s.conf.Timeout {
			errs <- errMaxSearchWaitTimeExceeded
			return
		}
	}
	// Record the results
	var host string
	for _, f := range sr.Fields {
		switch fieldName := f.FieldName; fieldName {
		case "host":
			host = f.Value
			continue
		case "indexer_avg_kbps":
			v, err := strconv.ParseFloat(f.Value, 64)
			if err != nil {
				errs <- err
				continue
			}
			s.mb.RecordSplunkIndexerAvgRateDataPoint(now, v, host, i.Build, i.Version)
		}
	}
}

func (s *splunkScraper) scrapeIndexerPipelineQueues(_ context.Context, now pcommon.Timestamp, info infoDict, errs chan error) {
	// Because we have to utilize network resources for each KPI we should check that each metrics
	// is enabled before proceeding
	if !s.conf.Metrics.SplunkAggregationQueueRatio.Enabled {
		return
	}
	i := info[typeCm].Entries[0].Content

	sr := searchResponse{
		search: searchDict[`SplunkPipelineQueues`],
	}

	var (
		req *http.Request
		res *http.Response
		err error
	)

	start := time.Now()

	for {
		req, err = s.splunkClient.createRequest(typeCm, &sr)
		if err != nil {
			errs <- err
			return
		}

		res, err = s.splunkClient.makeRequest(req)
		if err != nil {
			errs <- err
			return
		}

		// if its a 204 the body will be empty because we are still waiting on search results
		err = unmarshallSearchReq(res, &sr)
		if err != nil {
			errs <- err
		}

		res.Body.Close()

		// if no errors and 200 returned scrape was successful, return. Note we must make sure that
		// the 200 is coming after the first request which provides a jobId to retrieve results
		if sr.Return == 200 && sr.Jobid != nil {
			break
		}

		if sr.Return == 200 {
			break
		}

		if sr.Return == 204 {
			time.Sleep(2 * time.Second)
		}

		if sr.Return == 400 {
			break
		}

		if time.Since(start) > s.conf.Timeout {
			errs <- errMaxSearchWaitTimeExceeded
			return
		}
	}
	// Record the results
	var host string
	var ps int64
	for _, f := range sr.Fields {
		switch fieldName := f.FieldName; fieldName {
		case "host":
			host = f.Value
			continue
		case "agg_queue_ratio":
			v, err := strconv.ParseFloat(f.Value, 64)
			if err != nil {
				errs <- err
				continue
			}
			s.mb.RecordSplunkAggregationQueueRatioDataPoint(now, v, host, i.Build, i.Version)
		case "index_queue_ratio":
			v, err := strconv.ParseFloat(f.Value, 64)
			if err != nil {
				errs <- err
				continue
			}
			s.mb.RecordSplunkIndexerQueueRatioDataPoint(now, v, host, i.Build, i.Version)
		case "parse_queue_ratio":
			v, err := strconv.ParseFloat(f.Value, 64)
			if err != nil {
				errs <- err
				continue
			}
			s.mb.RecordSplunkParseQueueRatioDataPoint(now, v, host, i.Build, i.Version)
		case "pipeline_sets":
			v, err := strconv.ParseInt(f.Value, 10, 64)
			ps = v
			if err != nil {
				errs <- err
				continue
			}
			s.mb.RecordSplunkPipelineSetCountDataPoint(now, ps, host, i.Build, i.Version)
		case "typing_queue_ratio":
			v, err := strconv.ParseFloat(f.Value, 64)
			if err != nil {
				errs <- err
				continue
			}
			s.mb.RecordSplunkTypingQueueRatioDataPoint(now, v, host, i.Build, i.Version)
		}
	}
}

func (s *splunkScraper) scrapeBucketsSearchableStatus(_ context.Context, now pcommon.Timestamp, info infoDict, errs chan error) {
	// Because we have to utilize network resources for each KPI we should check that each metrics
	// is enabled before proceeding
	if !s.conf.Metrics.SplunkBucketsSearchableStatus.Enabled {
		return
	}
	i := info[typeCm].Entries[0].Content

	sr := searchResponse{
		search: searchDict[`SplunkBucketsSearchableStatus`],
	}

	var (
		req *http.Request
		res *http.Response
		err error
	)

	start := time.Now()

	for {
		req, err = s.splunkClient.createRequest(typeCm, &sr)
		if err != nil {
			errs <- err
			return
		}

		res, err = s.splunkClient.makeRequest(req)
		if err != nil {
			errs <- err
			return
		}

		// if its a 204 the body will be empty because we are still waiting on search results
		err = unmarshallSearchReq(res, &sr)
		if err != nil {
			errs <- err
		}

		res.Body.Close()

		// if no errors and 200 returned scrape was successful, return. Note we must make sure that
		// the 200 is coming after the first request which provides a jobId to retrieve results
		if sr.Return == 200 && sr.Jobid != nil {
			break
		}

		if sr.Return == 200 {
			break
		}

		if sr.Return == 204 {
			time.Sleep(2 * time.Second)
		}

		if sr.Return == 400 {
			break
		}

		if time.Since(start) > s.conf.Timeout {
			errs <- errMaxSearchWaitTimeExceeded
			return
		}
	}
	// Record the results
	var host string
	var searchable string
	var bc int64
	for _, f := range sr.Fields {
		switch fieldName := f.FieldName; fieldName {
		case "host":
			host = f.Value
			continue
		case "is_searchable":
			searchable = f.Value
			continue
		case "bucket_count":
			v, err := strconv.ParseInt(f.Value, 10, 64)
			bc = v
			if err != nil {
				errs <- err
				continue
			}
			s.mb.RecordSplunkBucketsSearchableStatusDataPoint(now, bc, host, searchable, i.Build, i.Version)
		}
	}
}

func (s *splunkScraper) scrapeIndexesBucketCountAdHoc(_ context.Context, now pcommon.Timestamp, info infoDict, errs chan error) {
	// Because we have to utilize network resources for each KPI we should check that each metrics
	// is enabled before proceeding
	if !s.conf.Metrics.SplunkIndexesSize.Enabled {
		return
	}
	i := info[typeCm].Entries[0].Content

	sr := searchResponse{
		search: searchDict[`SplunkIndexesData`],
	}

	var (
		req *http.Request
		res *http.Response
		err error
	)

	start := time.Now()

	for {
		req, err = s.splunkClient.createRequest(typeCm, &sr)
		if err != nil {
			errs <- err
			return
		}

		res, err = s.splunkClient.makeRequest(req)
		if err != nil {
			errs <- err
			return
		}

		// if its a 204 the body will be empty because we are still waiting on search results
		err = unmarshallSearchReq(res, &sr)
		if err != nil {
			errs <- err
		}
		res.Body.Close()

		// if no errors and 200 returned scrape was successful, return. Note we must make sure that
		// the 200 is coming after the first request which provides a jobId to retrieve results

		if sr.Return == 200 && sr.Jobid != nil {
			break
		}

		if sr.Return == 200 {
			break
		}

		if sr.Return == 204 {
			time.Sleep(2 * time.Second)
		}

		if sr.Return == 400 {
			break
		}

		if time.Since(start) > s.conf.Timeout {
			errs <- errMaxSearchWaitTimeExceeded
			return
		}
	}
	// Record the results
	var indexer string
	var bc int64
	for _, f := range sr.Fields {
		switch fieldName := f.FieldName; fieldName {
		case "title":
			indexer = f.Value
			continue
		case "total_size_gb":
			v, err := strconv.ParseFloat(f.Value, 64)
			if err != nil {
				errs <- err
				continue
			}
			s.mb.RecordSplunkIndexesSizeDataPoint(now, v, indexer, i.Build, i.Version)
		case "average_size_gb":
			v, err := strconv.ParseFloat(f.Value, 64)
			if err != nil {
				errs <- err
				continue
			}
			s.mb.RecordSplunkIndexesAvgSizeDataPoint(now, v, indexer, i.Build, i.Version)
		case "average_usage_perc":
			v, err := strconv.ParseFloat(f.Value, 64)
			if err != nil {
				errs <- err
				continue
			}
			s.mb.RecordSplunkIndexesAvgUsageDataPoint(now, v, indexer, i.Build, i.Version)
		case "median_data_age":
			v, err := strconv.ParseInt(f.Value, 10, 64)
			bc = v
			if err != nil {
				errs <- err
				continue
			}
			s.mb.RecordSplunkIndexesMedianDataAgeDataPoint(now, bc, indexer, i.Build, i.Version)
		case "bucket_count":
			v, err := strconv.ParseInt(f.Value, 10, 64)
			bc = v
			if err != nil {
				errs <- err
				continue
			}
			s.mb.RecordSplunkIndexesBucketCountDataPoint(now, bc, indexer, i.Build, i.Version)
		}
	}
}

func (s *splunkScraper) scrapeSchedulerCompletionRatioByHost(_ context.Context, now pcommon.Timestamp, info infoDict, errs chan error) {
	// Because we have to utilize network resources for each KPI we should check that each metrics
	// is enabled before proceeding
	if !s.conf.Metrics.SplunkSchedulerCompletionRatio.Enabled {
		return
	}
	i := info[typeCm].Entries[0].Content

	sr := searchResponse{
		search: searchDict[`SplunkSchedulerCompletionRatio`],
	}

	var (
		req *http.Request
		res *http.Response
		err error
	)

	start := time.Now()

	for {
		req, err = s.splunkClient.createRequest(typeCm, &sr)
		if err != nil {
			errs <- err
			return
		}

		res, err = s.splunkClient.makeRequest(req)
		if err != nil {
			errs <- err
			return
		}

		// if its a 204 the body will be empty because we are still waiting on search results
		err = unmarshallSearchReq(res, &sr)
		if err != nil {
			errs <- err
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

		if sr.Return == 400 {
			break
		}

		if time.Since(start) > s.conf.Timeout {
			errs <- errMaxSearchWaitTimeExceeded
			return
		}
	}

	// Record the results
	var host string
	for _, f := range sr.Fields {
		switch fieldName := f.FieldName; fieldName {
		case "host":
			host = f.Value
			continue
		case "completion_ratio":
			v, err := strconv.ParseFloat(f.Value, 64)
			if err != nil {
				errs <- err
				continue
			}
			s.mb.RecordSplunkSchedulerCompletionRatioDataPoint(now, v, host, i.Build, i.Version)
		}
	}
}

func (s *splunkScraper) scrapeIndexerRawWriteSecondsByHost(_ context.Context, now pcommon.Timestamp, info infoDict, errs chan error) {
	// Because we have to utilize network resources for each KPI we should check that each metrics
	// is enabled before proceeding
	if !s.conf.Metrics.SplunkIndexerRawWriteTime.Enabled {
		return
	}
	i := info[typeCm].Entries[0].Content

	sr := searchResponse{
		search: searchDict[`SplunkIndexerRawWriteSeconds`],
	}

	var (
		req *http.Request
		res *http.Response
		err error
	)

	start := time.Now()

	for {
		req, err = s.splunkClient.createRequest(typeCm, &sr)
		if err != nil {
			errs <- err
			return
		}

		res, err = s.splunkClient.makeRequest(req)
		if err != nil {
			errs <- err
			return
		}

		// if its a 204 the body will be empty because we are still waiting on search results
		err = unmarshallSearchReq(res, &sr)
		if err != nil {
			errs <- err
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

		if sr.Return == 400 {
			break
		}

		if time.Since(start) > s.conf.Timeout {
			errs <- errMaxSearchWaitTimeExceeded
			return
		}
	}

	// Record the results
	var host string
	for _, f := range sr.Fields {
		switch fieldName := f.FieldName; fieldName {
		case "host":
			host = f.Value
			continue
		case "raw_data_write_seconds":
			v, err := strconv.ParseFloat(f.Value, 64)
			if err != nil {
				errs <- err
				continue
			}
			s.mb.RecordSplunkIndexerRawWriteTimeDataPoint(now, v, host, i.Build, i.Version)
		}
	}
}

func (s *splunkScraper) scrapeIndexerCPUSecondsByHost(_ context.Context, now pcommon.Timestamp, info infoDict, errs chan error) {
	// Because we have to utilize network resources for each KPI we should check that each metrics
	// is enabled before proceeding
	if !s.conf.Metrics.SplunkIndexerCPUTime.Enabled {
		return
	}
	i := info[typeCm].Entries[0].Content

	sr := searchResponse{
		search: searchDict[`SplunkIndexerCpuSeconds`],
	}

	var (
		req *http.Request
		res *http.Response
		err error
	)

	start := time.Now()

	for {
		req, err = s.splunkClient.createRequest(typeCm, &sr)
		if err != nil {
			errs <- err
			return
		}

		res, err = s.splunkClient.makeRequest(req)
		if err != nil {
			errs <- err
			return
		}

		// if its a 204 the body will be empty because we are still waiting on search results
		err = unmarshallSearchReq(res, &sr)
		if err != nil {
			errs <- err
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

		if sr.Return == 400 {
			break
		}

		if time.Since(start) > s.conf.Timeout {
			errs <- errMaxSearchWaitTimeExceeded
			return
		}
	}

	// Record the results
	var host string
	for _, f := range sr.Fields {
		switch fieldName := f.FieldName; fieldName {
		case "host":
			host = f.Value
			continue
		case "service_cpu_seconds":
			v, err := strconv.ParseFloat(f.Value, 64)
			if err != nil {
				errs <- err
				continue
			}
			s.mb.RecordSplunkIndexerCPUTimeDataPoint(now, v, host, i.Build, i.Version)
		}
	}
}

func (s *splunkScraper) scrapeAvgIopsByHost(_ context.Context, now pcommon.Timestamp, info infoDict, errs chan error) {
	// Because we have to utilize network resources for each KPI we should check that each metrics
	// is enabled before proceeding
	if !s.conf.Metrics.SplunkIoAvgIops.Enabled {
		return
	}
	i := info[typeCm].Entries[0].Content

	sr := searchResponse{
		search: searchDict[`SplunkIoAvgIops`],
	}

	var (
		req *http.Request
		res *http.Response
		err error
	)

	start := time.Now()

	for {
		req, err = s.splunkClient.createRequest(typeCm, &sr)
		if err != nil {
			errs <- err
			return
		}

		res, err = s.splunkClient.makeRequest(req)
		if err != nil {
			errs <- err
			return
		}

		// if its a 204 the body will be empty because we are still waiting on search results
		err = unmarshallSearchReq(res, &sr)
		if err != nil {
			errs <- err
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

		if sr.Return == 400 {
			break
		}

		if time.Since(start) > s.conf.Timeout {
			errs <- errMaxSearchWaitTimeExceeded
			return
		}
	}

	// Record the results
	var host string
	for _, f := range sr.Fields {
		switch fieldName := f.FieldName; fieldName {
		case "host":
			host = f.Value
			continue
		case "iops":
			v, err := strconv.ParseInt(f.Value, 10, 64)
			if err != nil {
				errs <- err
				continue
			}
			s.mb.RecordSplunkIoAvgIopsDataPoint(now, v, host, i.Build, i.Version)
		}
	}
}

func (s *splunkScraper) scrapeSchedulerRunTimeByHost(_ context.Context, now pcommon.Timestamp, info infoDict, errs chan error) {
	// Because we have to utilize network resources for each KPI we should check that each metrics
	// is enabled before proceeding
	if !s.conf.Metrics.SplunkSchedulerAvgRunTime.Enabled {
		return
	}
	i := info[typeCm].Entries[0].Content

	sr := searchResponse{
		search: searchDict[`SplunkSchedulerAvgRunTime`],
	}

	var (
		req *http.Request
		res *http.Response
		err error
	)

	start := time.Now()

	for {
		req, err = s.splunkClient.createRequest(typeCm, &sr)
		if err != nil {
			errs <- err
			return
		}

		res, err = s.splunkClient.makeRequest(req)
		if err != nil {
			errs <- err
			return
		}

		// if its a 204 the body will be empty because we are still waiting on search results
		err = unmarshallSearchReq(res, &sr)
		if err != nil {
			errs <- err
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

		if sr.Return == 400 {
			break
		}

		if time.Since(start) > s.conf.Timeout {
			errs <- errMaxSearchWaitTimeExceeded
			return
		}
	}

	// Record the results
	var host string
	for _, f := range sr.Fields {
		switch fieldName := f.FieldName; fieldName {
		case "host":
			host = f.Value
			continue
		case "run_time_avg":
			v, err := strconv.ParseFloat(f.Value, 64)
			if err != nil {
				errs <- err
				continue
			}
			s.mb.RecordSplunkSchedulerAvgRunTimeDataPoint(now, v, host, i.Build, i.Version)
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
		return fmt.Errorf("failed to read response: %w", err)
	}

	err = xml.Unmarshal(body, &sr)
	if err != nil {
		return fmt.Errorf("failed to unmarshall response: %w", err)
	}

	return nil
}

// Scrape index throughput introspection endpoint
func (s *splunkScraper) scrapeIndexThroughput(_ context.Context, now pcommon.Timestamp, info infoDict, errs chan error) {
	if !s.conf.Metrics.SplunkIndexerThroughput.Enabled || !s.splunkClient.isConfigured(typeIdx) {
		return
	}
	i := info[typeIdx].Entries[0].Content

	var it indexThroughput

	ept := apiDict[`SplunkIndexerThroughput`]

	req, err := s.splunkClient.createAPIRequest(typeIdx, ept)
	if err != nil {
		errs <- err
		return
	}

	res, err := s.splunkClient.makeRequest(req)
	if err != nil {
		errs <- err
		return
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		errs <- err
		return
	}

	err = json.Unmarshal(body, &it)
	if err != nil {
		errs <- err
		return
	}

	for _, entry := range it.Entries {
		s.mb.RecordSplunkIndexerThroughputDataPoint(now, 1000*entry.Content.AvgKb, entry.Content.Status, i.Build, i.Version)
	}
}

// Scrape indexes extended total size
func (s *splunkScraper) scrapeIndexesTotalSize(_ context.Context, now pcommon.Timestamp, info infoDict, errs chan error) {
	if !s.conf.Metrics.SplunkDataIndexesExtendedTotalSize.Enabled || !s.splunkClient.isConfigured(typeIdx) {
		return
	}
	i := info[typeIdx].Entries[0].Content

	var it indexesExtended
	ept := apiDict[`SplunkDataIndexesExtended`]

	req, err := s.splunkClient.createAPIRequest(typeIdx, ept)
	if err != nil {
		errs <- err
		return
	}

	res, err := s.splunkClient.makeRequest(req)
	if err != nil {
		errs <- err
		return
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		errs <- err
		return
	}

	err = json.Unmarshal(body, &it)
	if err != nil {
		errs <- err
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
				errs <- err
			}
		}

		s.mb.RecordSplunkDataIndexesExtendedTotalSizeDataPoint(now, totalSize, name, i.Build, i.Version)
	}
}

// Scrape indexes extended total event count
func (s *splunkScraper) scrapeIndexesEventCount(_ context.Context, now pcommon.Timestamp, info infoDict, errs chan error) {
	if !s.conf.Metrics.SplunkDataIndexesExtendedEventCount.Enabled || !s.splunkClient.isConfigured(typeIdx) {
		return
	}
	i := info[typeIdx].Entries[0].Content

	var it indexesExtended

	ept := apiDict[`SplunkDataIndexesExtended`]

	req, err := s.splunkClient.createAPIRequest(typeIdx, ept)
	if err != nil {
		errs <- err
		return
	}

	res, err := s.splunkClient.makeRequest(req)
	if err != nil {
		errs <- err
		return
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		errs <- err
		return
	}

	err = json.Unmarshal(body, &it)
	if err != nil {
		errs <- err
		return
	}

	var name string
	for _, f := range it.Entries {
		if f.Name != "" {
			name = f.Name
		}
		totalEventCount := int64(f.Content.TotalEventCount)

		s.mb.RecordSplunkDataIndexesExtendedEventCountDataPoint(now, totalEventCount, name, i.Build, i.Version)
	}
}

// Scrape indexes extended total bucket count
func (s *splunkScraper) scrapeIndexesBucketCount(_ context.Context, now pcommon.Timestamp, info infoDict, errs chan error) {
	if !s.conf.Metrics.SplunkDataIndexesExtendedBucketCount.Enabled || !s.splunkClient.isConfigured(typeIdx) {
		return
	}
	i := info[typeIdx].Entries[0].Content

	var it indexesExtended

	ept := apiDict[`SplunkDataIndexesExtended`]

	req, err := s.splunkClient.createAPIRequest(typeIdx, ept)
	if err != nil {
		errs <- err
		return
	}

	res, err := s.splunkClient.makeRequest(req)
	if err != nil {
		errs <- err
		return
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		errs <- err
		return
	}

	err = json.Unmarshal(body, &it)
	if err != nil {
		errs <- err
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
				errs <- err
			}
		}

		s.mb.RecordSplunkDataIndexesExtendedBucketCountDataPoint(now, totalBucketCount, name, i.Build, i.Version)
	}
}

// Scrape indexes extended raw size
func (s *splunkScraper) scrapeIndexesRawSize(_ context.Context, now pcommon.Timestamp, info infoDict, errs chan error) {
	if !s.conf.Metrics.SplunkDataIndexesExtendedRawSize.Enabled || !s.splunkClient.isConfigured(typeIdx) {
		return
	}
	i := info[typeIdx].Entries[0].Content

	var it indexesExtended

	ept := apiDict[`SplunkDataIndexesExtended`]

	req, err := s.splunkClient.createAPIRequest(typeIdx, ept)
	if err != nil {
		errs <- err
		return
	}

	res, err := s.splunkClient.makeRequest(req)
	if err != nil {
		errs <- err
		return
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		errs <- err
		return
	}

	err = json.Unmarshal(body, &it)
	if err != nil {
		errs <- err
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
				errs <- err
			}
		}
		s.mb.RecordSplunkDataIndexesExtendedRawSizeDataPoint(now, totalRawSize, name, i.Build, i.Version)
	}
}

// Scrape indexes extended bucket event count
func (s *splunkScraper) scrapeIndexesBucketEventCount(_ context.Context, now pcommon.Timestamp, info infoDict, errs chan error) {
	if !s.conf.Metrics.SplunkDataIndexesExtendedBucketEventCount.Enabled || !s.splunkClient.isConfigured(typeIdx) {
		return
	}
	i := info[typeIdx].Entries[0].Content

	var it indexesExtended

	ept := apiDict[`SplunkDataIndexesExtended`]

	req, err := s.splunkClient.createAPIRequest(typeIdx, ept)
	if err != nil {
		errs <- err
		return
	}

	res, err := s.splunkClient.makeRequest(req)
	if err != nil {
		errs <- err
		return
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		errs <- err
		return
	}

	err = json.Unmarshal(body, &it)
	if err != nil {
		errs <- err
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
				errs <- err
			}
			s.mb.RecordSplunkDataIndexesExtendedBucketEventCountDataPoint(now, bucketEventCount, name, bucketDir, i.Build, i.Version)
		}
		if f.Content.BucketDirs.Home.EventCount != "" {
			bucketDir = "home"
			bucketEventCount, err = strconv.ParseInt(f.Content.BucketDirs.Home.EventCount, 10, 64)
			if err != nil {
				errs <- err
			}
			s.mb.RecordSplunkDataIndexesExtendedBucketEventCountDataPoint(now, bucketEventCount, name, bucketDir, i.Build, i.Version)
		}
		if f.Content.BucketDirs.Thawed.EventCount != "" {
			bucketDir = "thawed"
			bucketEventCount, err = strconv.ParseInt(f.Content.BucketDirs.Thawed.EventCount, 10, 64)
			if err != nil {
				errs <- err
			}
			s.mb.RecordSplunkDataIndexesExtendedBucketEventCountDataPoint(now, bucketEventCount, name, bucketDir, i.Build, i.Version)
		}
	}
}

// Scrape indexes extended bucket hot/warm count
func (s *splunkScraper) scrapeIndexesBucketHotWarmCount(_ context.Context, now pcommon.Timestamp, info infoDict, errs chan error) {
	if !s.conf.Metrics.SplunkDataIndexesExtendedBucketHotCount.Enabled || !s.splunkClient.isConfigured(typeIdx) {
		return
	}
	i := info[typeIdx].Entries[0].Content

	var it indexesExtended

	ept := apiDict[`SplunkDataIndexesExtended`]

	req, err := s.splunkClient.createAPIRequest(typeIdx, ept)
	if err != nil {
		errs <- err
		return
	}

	res, err := s.splunkClient.makeRequest(req)
	if err != nil {
		errs <- err
		return
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		errs <- err
		return
	}

	err = json.Unmarshal(body, &it)
	if err != nil {
		errs <- err
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
				errs <- err
			}
			s.mb.RecordSplunkDataIndexesExtendedBucketHotCountDataPoint(now, bucketHotCount, name, bucketDir, i.Build, i.Version)
		}
		if f.Content.BucketDirs.Home.WarmBucketCount != "" {
			bucketWarmCount, err = strconv.ParseInt(f.Content.BucketDirs.Home.WarmBucketCount, 10, 64)
			bucketDir = "warm"
			if err != nil {
				errs <- err
			}
			s.mb.RecordSplunkDataIndexesExtendedBucketWarmCountDataPoint(now, bucketWarmCount, name, bucketDir, i.Build, i.Version)
		}
	}
}

// Scrape introspection queues
func (s *splunkScraper) scrapeIntrospectionQueues(_ context.Context, now pcommon.Timestamp, info infoDict, errs chan error) {
	if !s.conf.Metrics.SplunkServerIntrospectionQueuesCurrent.Enabled || !s.splunkClient.isConfigured(typeIdx) {
		return
	}
	i := info[typeIdx].Entries[0].Content

	var it introspectionQueues

	ept := apiDict[`SplunkIntrospectionQueues`]

	req, err := s.splunkClient.createAPIRequest(typeIdx, ept)
	if err != nil {
		errs <- err
		return
	}

	res, err := s.splunkClient.makeRequest(req)
	if err != nil {
		errs <- err
		return
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		errs <- err
		return
	}

	err = json.Unmarshal(body, &it)
	if err != nil {
		errs <- err
		return
	}

	var name string
	for _, f := range it.Entries {
		if f.Name != "" {
			name = f.Name
		}

		currentQueuesSize := int64(f.Content.CurrentSize)

		s.mb.RecordSplunkServerIntrospectionQueuesCurrentDataPoint(now, currentQueuesSize, name, i.Build, i.Version)
	}
}

// Scrape introspection queues bytes
func (s *splunkScraper) scrapeIntrospectionQueuesBytes(_ context.Context, now pcommon.Timestamp, info infoDict, errs chan error) {
	if !s.conf.Metrics.SplunkServerIntrospectionQueuesCurrentBytes.Enabled || !s.splunkClient.isConfigured(typeIdx) {
		return
	}
	i := info[typeIdx].Entries[0].Content

	var it introspectionQueues

	ept := apiDict[`SplunkIntrospectionQueues`]

	req, err := s.splunkClient.createAPIRequest(typeIdx, ept)
	if err != nil {
		errs <- err
		return
	}

	res, err := s.splunkClient.makeRequest(req)
	if err != nil {
		errs <- err
		return
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		errs <- err
		return
	}

	err = json.Unmarshal(body, &it)
	if err != nil {
		errs <- err
		return
	}
	var name string
	for _, f := range it.Entries {
		if f.Name != "" {
			name = f.Name
		}

		currentQueueSizeBytes := int64(f.Content.CurrentSizeBytes)

		s.mb.RecordSplunkServerIntrospectionQueuesCurrentBytesDataPoint(now, currentQueueSizeBytes, name, i.Build, i.Version)
	}
}

// Scrape introspection kv store status
func (s *splunkScraper) scrapeKVStoreStatus(_ context.Context, now pcommon.Timestamp, info infoDict, errs chan error) {
	if !s.conf.Metrics.SplunkKvstoreStatus.Enabled ||
		!s.conf.Metrics.SplunkKvstoreReplicationStatus.Enabled ||
		!s.conf.Metrics.SplunkKvstoreBackupStatus.Enabled ||
		!s.splunkClient.isConfigured(typeCm) {
		return
	}
	i := info[typeCm].Entries[0].Content

	var kvs kvStoreStatus

	ept := apiDict[`SplunkKVStoreStatus`]

	req, err := s.splunkClient.createAPIRequest(typeCm, ept)
	if err != nil {
		errs <- err
		return
	}

	res, err := s.splunkClient.makeRequest(req)
	if err != nil {
		errs <- err
		return
	}
	defer res.Body.Close()

	if err := json.NewDecoder(res.Body).Decode(&kvs); err != nil {
		errs <- err
		return
	}

	var st, brs, rs, se, ext string
	for _, kv := range kvs.Entries {
		st = kv.Content.Current.Status // overall status
		brs = kv.Content.Current.BackupRestoreStatus
		rs = kv.Content.Current.ReplicationStatus
		se = kv.Content.Current.StorageEngine
		ext = kv.Content.KVService.Status

		// a 0 gauge value means that the metric was not reported in the api call
		// to the introspection endpoint.
		if st == "" {
			st = kvStatusUnknown
			// set to 0 to indicate no status being reported
			s.mb.RecordSplunkKvstoreStatusDataPoint(now, 0, se, ext, st, i.Build, i.Version)
		} else {
			s.mb.RecordSplunkKvstoreStatusDataPoint(now, 1, se, ext, st, i.Build, i.Version)
		}

		if rs == "" {
			rs = kvRestoreStatusUnknown
			s.mb.RecordSplunkKvstoreReplicationStatusDataPoint(now, 0, rs, i.Build, i.Version)
		} else {
			s.mb.RecordSplunkKvstoreReplicationStatusDataPoint(now, 1, rs, i.Build, i.Version)
		}

		if brs == "" {
			brs = kvBackupStatusFailed
			s.mb.RecordSplunkKvstoreBackupStatusDataPoint(now, 0, brs, i.Build, i.Version)
		} else {
			s.mb.RecordSplunkKvstoreBackupStatusDataPoint(now, 1, brs, i.Build, i.Version)
		}
	}
}

// Scrape dispatch artifacts
func (s *splunkScraper) scrapeSearchArtifacts(_ context.Context, now pcommon.Timestamp, info infoDict, errs chan error) {
	// if NONE of the metrics set in this scrape are set we return early
	if !s.conf.Metrics.SplunkServerSearchartifactsAdhoc.Enabled && !s.conf.Metrics.SplunkServerSearchartifactsScheduled.Enabled && !s.conf.Metrics.SplunkServerSearchartifactsCompleted.Enabled && !s.conf.Metrics.SplunkServerSearchartifactsIncomplete.Enabled && !s.conf.Metrics.SplunkServerSearchartifactsInvalid.Enabled && !s.conf.Metrics.SplunkServerSearchartifactsSavedsearches.Enabled && !s.conf.Metrics.SplunkServerSearchartifactsJobCacheSize.Enabled && !s.conf.Metrics.SplunkServerSearchartifactsJobCacheCount.Enabled {
		return
	}

	if !s.splunkClient.isConfigured(typeSh) {
		return
	}
	i := info[typeSh].Entries[0].Content

	var da dispatchArtifacts

	ept := apiDict[`SplunkDispatchArtifacts`]

	req, err := s.splunkClient.createAPIRequest(typeSh, ept)
	if err != nil {
		errs <- err
		return
	}

	res, err := s.splunkClient.makeRequest(req)
	if err != nil {
		errs <- err
		return
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		errs <- err
		return
	}
	err = json.Unmarshal(body, &da)
	if err != nil {
		errs <- err
		return
	}

	for _, f := range da.Entries {
		if s.conf.Metrics.SplunkServerSearchartifactsAdhoc.Enabled {
			adhocCount, err := strconv.ParseInt(f.Content.AdhocCount, 10, 64)
			if err != nil {
				errs <- err
			}
			s.mb.RecordSplunkServerSearchartifactsAdhocDataPoint(now, adhocCount, s.conf.SHEndpoint.Endpoint, i.Build, i.Version)
		}

		if s.conf.Metrics.SplunkServerSearchartifactsScheduled.Enabled {
			scheduledCount, err := strconv.ParseInt(f.Content.ScheduledCount, 10, 64)
			if err != nil {
				errs <- err
			}
			s.mb.RecordSplunkServerSearchartifactsScheduledDataPoint(now, scheduledCount, s.conf.SHEndpoint.Endpoint, i.Build, i.Version)
		}

		if s.conf.Metrics.SplunkServerSearchartifactsCompleted.Enabled {
			completedCount, err := strconv.ParseInt(f.Content.CompletedCount, 10, 64)
			if err != nil {
				errs <- err
			}
			s.mb.RecordSplunkServerSearchartifactsCompletedDataPoint(now, completedCount, s.conf.SHEndpoint.Endpoint, i.Build, i.Version)
		}

		if s.conf.Metrics.SplunkServerSearchartifactsIncomplete.Enabled {
			incompleteCount, err := strconv.ParseInt(f.Content.IncompleteCount, 10, 64)
			if err != nil {
				errs <- err
			}
			s.mb.RecordSplunkServerSearchartifactsIncompleteDataPoint(now, incompleteCount, s.conf.SHEndpoint.Endpoint, i.Build, i.Version)
		}

		if s.conf.Metrics.SplunkServerSearchartifactsInvalid.Enabled {
			invalidCount, err := strconv.ParseInt(f.Content.InvalidCount, 10, 64)
			if err != nil {
				errs <- err
			}
			s.mb.RecordSplunkServerSearchartifactsInvalidDataPoint(now, invalidCount, s.conf.SHEndpoint.Endpoint, i.Build, i.Version)
		}

		if s.conf.Metrics.SplunkServerSearchartifactsSavedsearches.Enabled {
			savedSearchesCount, err := strconv.ParseInt(f.Content.SavedSearchesCount, 10, 64)
			if err != nil {
				errs <- err
			}
			s.mb.RecordSplunkServerSearchartifactsSavedsearchesDataPoint(now, savedSearchesCount, s.conf.SHEndpoint.Endpoint, i.Build, i.Version)
		}

		if s.conf.Metrics.SplunkServerSearchartifactsJobCacheSize.Enabled {
			infoCacheSize, err := strconv.ParseInt(f.Content.InfoCacheSize, 10, 64)
			if err != nil {
				errs <- err
			}
			statusCacheSize, err := strconv.ParseInt(f.Content.StatusCacheSize, 10, 64)
			if err != nil {
				errs <- err
			}
			s.mb.RecordSplunkServerSearchartifactsJobCacheSizeDataPoint(now, infoCacheSize, s.conf.SHEndpoint.Endpoint, "info", i.Build, i.Version)
			s.mb.RecordSplunkServerSearchartifactsJobCacheSizeDataPoint(now, statusCacheSize, s.conf.SHEndpoint.Endpoint, "status", i.Build, i.Version)
		}

		if s.conf.Metrics.SplunkServerSearchartifactsJobCacheCount.Enabled {
			cacheTotalEntries, err := strconv.ParseInt(f.Content.CacheTotalEntries, 10, 64)
			if err != nil {
				errs <- err
			}
			s.mb.RecordSplunkServerSearchartifactsJobCacheCountDataPoint(now, cacheTotalEntries, s.conf.SHEndpoint.Endpoint, i.Build, i.Version)
		}
	}
}

// Scrape Health Introspection Endpoint
func (s *splunkScraper) scrapeHealth(_ context.Context, now pcommon.Timestamp, info infoDict, errs chan error) {
	if !s.conf.Metrics.SplunkHealth.Enabled {
		return
	}
	i := info[typeCm].Entries[0].Content

	ept := apiDict[`SplunkHealth`]
	var ha healthArtifacts

	req, err := s.splunkClient.createAPIRequest(typeCm, ept)
	if err != nil {
		errs <- err
		return
	}

	res, err := s.splunkClient.makeRequest(req)
	if err != nil {
		errs <- err
		return
	}
	defer res.Body.Close()

	if err := json.NewDecoder(res.Body).Decode(&ha); err != nil {
		errs <- err
		return
	}

	s.settings.Logger.Debug(fmt.Sprintf("Features: %s", ha.Entries))
	for _, details := range ha.Entries {
		s.traverseHealthDetailFeatures(details.Content, now, i)
	}
}

func (s *splunkScraper) traverseHealthDetailFeatures(details healthDetails, now pcommon.Timestamp, i InfoContent) {
	if details.Features == nil {
		return
	}

	for k, feature := range details.Features {
		if feature.Health != "red" {
			s.settings.Logger.Debug(feature.Health)
			s.mb.RecordSplunkHealthDataPoint(now, 1, k, feature.Health, i.Build, i.Version)
		} else {
			s.settings.Logger.Debug(feature.Health)
			s.mb.RecordSplunkHealthDataPoint(now, 0, k, feature.Health, i.Build, i.Version)
		}
		s.traverseHealthDetailFeatures(feature, now, i)
	}
}

// somewhat unique scrape function for gathering the info attribute
func (s *splunkScraper) scrapeInfo(_ context.Context, _ pcommon.Timestamp, errs chan error) map[any]Info {
	// there could be an endpoint configured for each type (never more than 3)

	info := make(infoDict)
	nullInfo := Info{Host: "", Entries: make([]InfoEntry, 1)}
	info[typeCm] = nullInfo
	info[typeSh] = nullInfo
	info[typeIdx] = nullInfo

	for cliType := range s.splunkClient.clients {
		var i Info

		ept := apiDict[`SplunkInfo`]

		req, err := s.splunkClient.createAPIRequest(cliType, ept)
		if err != nil {
			errs <- err
			return info
		}

		res, err := s.splunkClient.makeRequest(req)
		if err != nil {
			errs <- err
			return info
		}
		defer res.Body.Close()

		body, err := io.ReadAll(res.Body)
		if err != nil {
			errs <- err
			return info
		}

		err = json.Unmarshal(body, &i)
		if err != nil {
			errs <- err
			return info
		}

		info[cliType] = i
	}

	return info
}
