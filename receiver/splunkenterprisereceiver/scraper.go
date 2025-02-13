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
	errOut := make(chan *scrapererror.ScrapeErrors)
	var errs *scrapererror.ScrapeErrors
	now := pcommon.NewTimestampFromTime(time.Now())
	metricScrapes := []func(context.Context, pcommon.Timestamp, chan error){
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

	for _, fn := range metricScrapes {
		wg.Add(1)
		go func(
			fn func(ctx context.Context, now pcommon.Timestamp, errs chan error),
			ctx context.Context,
			now pcommon.Timestamp,
			errs chan error,
		) {
			// actual function body
			defer wg.Done()
			fn(ctx, now, errs)
		}(fn, ctx, now, errChan)
	}

	wg.Wait()
	close(errChan)
	errs = <-errOut
	return s.mb.Emit(), errs.Combine()
}

// Each metric has its own scrape function associated with it
func (s *splunkScraper) scrapeLicenseUsageByIndex(ctx context.Context, now pcommon.Timestamp, errs chan error) {
	// Because we have to utilize network resources for each KPI we should check that each metrics
	// is enabled before proceeding
	if !s.conf.MetricsBuilderConfig.Metrics.SplunkLicenseIndexUsage.Enabled || !s.splunkClient.isConfigured(typeCm) {
		return
	}
	ctx = context.WithValue(ctx, endpointType("type"), typeCm)

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
		req, err = s.splunkClient.createRequest(ctx, &sr)
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

		if time.Since(start) > s.conf.ControllerConfig.Timeout {
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
			s.mb.RecordSplunkLicenseIndexUsageDataPoint(now, int64(v), indexName)
		}
	}
}

func (s *splunkScraper) scrapeAvgExecLatencyByHost(ctx context.Context, now pcommon.Timestamp, errs chan error) {
	// Because we have to utilize network resources for each KPI we should check that each metrics
	// is enabled before proceeding
	if !s.conf.MetricsBuilderConfig.Metrics.SplunkSchedulerAvgExecutionLatency.Enabled {
		return
	}

	sr := searchResponse{
		search: searchDict[`SplunkSchedulerAvgExecLatencySearch`],
	}
	ctx = context.WithValue(ctx, endpointType("type"), typeCm)

	var (
		req *http.Request
		res *http.Response
		err error
	)

	start := time.Now()

	for {
		req, err = s.splunkClient.createRequest(ctx, &sr)
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

		if time.Since(start) > s.conf.ControllerConfig.Timeout {
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
			s.mb.RecordSplunkSchedulerAvgExecutionLatencyDataPoint(now, v, host)
		}
	}
}

func (s *splunkScraper) scrapeIndexerAvgRate(ctx context.Context, now pcommon.Timestamp, errs chan error) {
	// Because we have to utilize network resources for each KPI we should check that each metrics
	// is enabled before proceeding
	if !s.conf.MetricsBuilderConfig.Metrics.SplunkIndexerAvgRate.Enabled {
		return
	}

	sr := searchResponse{
		search: searchDict[`SplunkIndexerAvgRate`],
	}
	ctx = context.WithValue(ctx, endpointType("type"), typeCm)

	var (
		req *http.Request
		res *http.Response
		err error
	)

	start := time.Now()

	for {
		req, err = s.splunkClient.createRequest(ctx, &sr)
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

		if time.Since(start) > s.conf.ControllerConfig.Timeout {
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
			s.mb.RecordSplunkIndexerAvgRateDataPoint(now, v, host)
		}
	}
}

func (s *splunkScraper) scrapeIndexerPipelineQueues(ctx context.Context, now pcommon.Timestamp, errs chan error) {
	// Because we have to utilize network resources for each KPI we should check that each metrics
	// is enabled before proceeding
	if !s.conf.MetricsBuilderConfig.Metrics.SplunkAggregationQueueRatio.Enabled {
		return
	}

	sr := searchResponse{
		search: searchDict[`SplunkPipelineQueues`],
	}
	ctx = context.WithValue(ctx, endpointType("type"), typeCm)

	var (
		req *http.Request
		res *http.Response
		err error
	)

	start := time.Now()

	for {
		req, err = s.splunkClient.createRequest(ctx, &sr)
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

		if time.Since(start) > s.conf.ControllerConfig.Timeout {
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
			s.mb.RecordSplunkAggregationQueueRatioDataPoint(now, v, host)
		case "index_queue_ratio":
			v, err := strconv.ParseFloat(f.Value, 64)
			if err != nil {
				errs <- err
				continue
			}
			s.mb.RecordSplunkIndexerQueueRatioDataPoint(now, v, host)
		case "parse_queue_ratio":
			v, err := strconv.ParseFloat(f.Value, 64)
			if err != nil {
				errs <- err
				continue
			}
			s.mb.RecordSplunkParseQueueRatioDataPoint(now, v, host)
		case "pipeline_sets":
			v, err := strconv.ParseInt(f.Value, 10, 64)
			ps = v
			if err != nil {
				errs <- err
				continue
			}
			s.mb.RecordSplunkPipelineSetCountDataPoint(now, ps, host)
		case "typing_queue_ratio":
			v, err := strconv.ParseFloat(f.Value, 64)
			if err != nil {
				errs <- err
				continue
			}
			s.mb.RecordSplunkTypingQueueRatioDataPoint(now, v, host)
		}
	}
}

func (s *splunkScraper) scrapeBucketsSearchableStatus(ctx context.Context, now pcommon.Timestamp, errs chan error) {
	// Because we have to utilize network resources for each KPI we should check that each metrics
	// is enabled before proceeding
	if !s.conf.MetricsBuilderConfig.Metrics.SplunkBucketsSearchableStatus.Enabled {
		return
	}

	sr := searchResponse{
		search: searchDict[`SplunkBucketsSearchableStatus`],
	}
	ctx = context.WithValue(ctx, endpointType("type"), typeCm)

	var (
		req *http.Request
		res *http.Response
		err error
	)

	start := time.Now()

	for {
		req, err = s.splunkClient.createRequest(ctx, &sr)
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

		if time.Since(start) > s.conf.ControllerConfig.Timeout {
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
			s.mb.RecordSplunkBucketsSearchableStatusDataPoint(now, bc, host, searchable)
		}
	}
}

func (s *splunkScraper) scrapeIndexesBucketCountAdHoc(ctx context.Context, now pcommon.Timestamp, errs chan error) {
	// Because we have to utilize network resources for each KPI we should check that each metrics
	// is enabled before proceeding
	if !s.conf.MetricsBuilderConfig.Metrics.SplunkIndexesSize.Enabled {
		return
	}

	sr := searchResponse{
		search: searchDict[`SplunkIndexesData`],
	}
	ctx = context.WithValue(ctx, endpointType("type"), typeCm)

	var (
		req *http.Request
		res *http.Response
		err error
	)

	start := time.Now()

	for {
		req, err = s.splunkClient.createRequest(ctx, &sr)
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

		if time.Since(start) > s.conf.ControllerConfig.Timeout {
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
			s.mb.RecordSplunkIndexesSizeDataPoint(now, v, indexer)
		case "average_size_gb":
			v, err := strconv.ParseFloat(f.Value, 64)
			if err != nil {
				errs <- err
				continue
			}
			s.mb.RecordSplunkIndexesAvgSizeDataPoint(now, v, indexer)
		case "average_usage_perc":
			v, err := strconv.ParseFloat(f.Value, 64)
			if err != nil {
				errs <- err
				continue
			}
			s.mb.RecordSplunkIndexesAvgUsageDataPoint(now, v, indexer)
		case "median_data_age":
			v, err := strconv.ParseInt(f.Value, 10, 64)
			bc = v
			if err != nil {
				errs <- err
				continue
			}
			s.mb.RecordSplunkIndexesMedianDataAgeDataPoint(now, bc, indexer)
		case "bucket_count":
			v, err := strconv.ParseInt(f.Value, 10, 64)
			bc = v
			if err != nil {
				errs <- err
				continue
			}
			s.mb.RecordSplunkIndexesBucketCountDataPoint(now, bc, indexer)
		}
	}
}

func (s *splunkScraper) scrapeSchedulerCompletionRatioByHost(ctx context.Context, now pcommon.Timestamp, errs chan error) {
	// Because we have to utilize network resources for each KPI we should check that each metrics
	// is enabled before proceeding
	if !s.conf.MetricsBuilderConfig.Metrics.SplunkSchedulerCompletionRatio.Enabled {
		return
	}

	sr := searchResponse{
		search: searchDict[`SplunkSchedulerCompletionRatio`],
	}
	ctx = context.WithValue(ctx, endpointType("type"), typeCm)

	var (
		req *http.Request
		res *http.Response
		err error
	)

	start := time.Now()

	for {
		req, err = s.splunkClient.createRequest(ctx, &sr)
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

		if time.Since(start) > s.conf.ControllerConfig.Timeout {
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
			s.mb.RecordSplunkSchedulerCompletionRatioDataPoint(now, v, host)
		}
	}
}

func (s *splunkScraper) scrapeIndexerRawWriteSecondsByHost(ctx context.Context, now pcommon.Timestamp, errs chan error) {
	// Because we have to utilize network resources for each KPI we should check that each metrics
	// is enabled before proceeding
	if !s.conf.MetricsBuilderConfig.Metrics.SplunkIndexerRawWriteTime.Enabled {
		return
	}

	sr := searchResponse{
		search: searchDict[`SplunkIndexerRawWriteSeconds`],
	}
	ctx = context.WithValue(ctx, endpointType("type"), typeCm)

	var (
		req *http.Request
		res *http.Response
		err error
	)

	start := time.Now()

	for {
		req, err = s.splunkClient.createRequest(ctx, &sr)
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

		if time.Since(start) > s.conf.ControllerConfig.Timeout {
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
			s.mb.RecordSplunkIndexerRawWriteTimeDataPoint(now, v, host)
		}
	}
}

func (s *splunkScraper) scrapeIndexerCPUSecondsByHost(ctx context.Context, now pcommon.Timestamp, errs chan error) {
	// Because we have to utilize network resources for each KPI we should check that each metrics
	// is enabled before proceeding
	if !s.conf.MetricsBuilderConfig.Metrics.SplunkIndexerCPUTime.Enabled {
		return
	}

	sr := searchResponse{
		search: searchDict[`SplunkIndexerCpuSeconds`],
	}
	ctx = context.WithValue(ctx, endpointType("type"), typeCm)

	var (
		req *http.Request
		res *http.Response
		err error
	)

	start := time.Now()

	for {
		req, err = s.splunkClient.createRequest(ctx, &sr)
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

		if time.Since(start) > s.conf.ControllerConfig.Timeout {
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
			s.mb.RecordSplunkIndexerCPUTimeDataPoint(now, v, host)
		}
	}
}

func (s *splunkScraper) scrapeAvgIopsByHost(ctx context.Context, now pcommon.Timestamp, errs chan error) {
	// Because we have to utilize network resources for each KPI we should check that each metrics
	// is enabled before proceeding
	if !s.conf.MetricsBuilderConfig.Metrics.SplunkIoAvgIops.Enabled {
		return
	}

	sr := searchResponse{
		search: searchDict[`SplunkIoAvgIops`],
	}
	ctx = context.WithValue(ctx, endpointType("type"), typeCm)

	var (
		req *http.Request
		res *http.Response
		err error
	)

	start := time.Now()

	for {
		req, err = s.splunkClient.createRequest(ctx, &sr)
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

		if time.Since(start) > s.conf.ControllerConfig.Timeout {
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
			s.mb.RecordSplunkIoAvgIopsDataPoint(now, v, host)
		}
	}
}

func (s *splunkScraper) scrapeSchedulerRunTimeByHost(ctx context.Context, now pcommon.Timestamp, errs chan error) {
	// Because we have to utilize network resources for each KPI we should check that each metrics
	// is enabled before proceeding
	if !s.conf.MetricsBuilderConfig.Metrics.SplunkSchedulerAvgRunTime.Enabled {
		return
	}

	sr := searchResponse{
		search: searchDict[`SplunkSchedulerAvgRunTime`],
	}
	ctx = context.WithValue(ctx, endpointType("type"), typeCm)

	var (
		req *http.Request
		res *http.Response
		err error
	)

	start := time.Now()

	for {
		req, err = s.splunkClient.createRequest(ctx, &sr)
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

		if time.Since(start) > s.conf.ControllerConfig.Timeout {
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
			s.mb.RecordSplunkSchedulerAvgRunTimeDataPoint(now, v, host)
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
func (s *splunkScraper) scrapeIndexThroughput(ctx context.Context, now pcommon.Timestamp, errs chan error) {
	if !s.conf.MetricsBuilderConfig.Metrics.SplunkIndexerThroughput.Enabled || !s.splunkClient.isConfigured(typeIdx) {
		return
	}

	ctx = context.WithValue(ctx, endpointType("type"), typeIdx)
	var it indexThroughput

	ept := apiDict[`SplunkIndexerThroughput`]

	req, err := s.splunkClient.createAPIRequest(ctx, ept)
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
		s.mb.RecordSplunkIndexerThroughputDataPoint(now, 1000*entry.Content.AvgKb, entry.Content.Status)
	}
}

// Scrape indexes extended total size
func (s *splunkScraper) scrapeIndexesTotalSize(ctx context.Context, now pcommon.Timestamp, errs chan error) {
	if !s.conf.MetricsBuilderConfig.Metrics.SplunkDataIndexesExtendedTotalSize.Enabled || !s.splunkClient.isConfigured(typeIdx) {
		return
	}

	ctx = context.WithValue(ctx, endpointType("type"), typeIdx)
	var it IndexesExtended
	ept := apiDict[`SplunkDataIndexesExtended`]

	req, err := s.splunkClient.createAPIRequest(ctx, ept)
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

		s.mb.RecordSplunkDataIndexesExtendedTotalSizeDataPoint(now, totalSize, name)
	}
}

// Scrape indexes extended total event count
func (s *splunkScraper) scrapeIndexesEventCount(ctx context.Context, now pcommon.Timestamp, errs chan error) {
	if !s.conf.MetricsBuilderConfig.Metrics.SplunkDataIndexesExtendedEventCount.Enabled || !s.splunkClient.isConfigured(typeIdx) {
		return
	}

	ctx = context.WithValue(ctx, endpointType("type"), typeIdx)
	var it IndexesExtended

	ept := apiDict[`SplunkDataIndexesExtended`]

	req, err := s.splunkClient.createAPIRequest(ctx, ept)
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

		s.mb.RecordSplunkDataIndexesExtendedEventCountDataPoint(now, totalEventCount, name)
	}
}

// Scrape indexes extended total bucket count
func (s *splunkScraper) scrapeIndexesBucketCount(ctx context.Context, now pcommon.Timestamp, errs chan error) {
	if !s.conf.MetricsBuilderConfig.Metrics.SplunkDataIndexesExtendedBucketCount.Enabled || !s.splunkClient.isConfigured(typeIdx) {
		return
	}

	ctx = context.WithValue(ctx, endpointType("type"), typeIdx)
	var it IndexesExtended

	ept := apiDict[`SplunkDataIndexesExtended`]

	req, err := s.splunkClient.createAPIRequest(ctx, ept)
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

		s.mb.RecordSplunkDataIndexesExtendedBucketCountDataPoint(now, totalBucketCount, name)
	}
}

// Scrape indexes extended raw size
func (s *splunkScraper) scrapeIndexesRawSize(ctx context.Context, now pcommon.Timestamp, errs chan error) {
	if !s.conf.MetricsBuilderConfig.Metrics.SplunkDataIndexesExtendedRawSize.Enabled || !s.splunkClient.isConfigured(typeIdx) {
		return
	}

	ctx = context.WithValue(ctx, endpointType("type"), typeIdx)
	var it IndexesExtended

	ept := apiDict[`SplunkDataIndexesExtended`]

	req, err := s.splunkClient.createAPIRequest(ctx, ept)
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
		s.mb.RecordSplunkDataIndexesExtendedRawSizeDataPoint(now, totalRawSize, name)
	}
}

// Scrape indexes extended bucket event count
func (s *splunkScraper) scrapeIndexesBucketEventCount(ctx context.Context, now pcommon.Timestamp, errs chan error) {
	if !s.conf.MetricsBuilderConfig.Metrics.SplunkDataIndexesExtendedBucketEventCount.Enabled || !s.splunkClient.isConfigured(typeIdx) {
		return
	}

	ctx = context.WithValue(ctx, endpointType("type"), typeIdx)
	var it IndexesExtended

	ept := apiDict[`SplunkDataIndexesExtended`]

	req, err := s.splunkClient.createAPIRequest(ctx, ept)
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
			s.mb.RecordSplunkDataIndexesExtendedBucketEventCountDataPoint(now, bucketEventCount, name, bucketDir)
		}
		if f.Content.BucketDirs.Home.EventCount != "" {
			bucketDir = "home"
			bucketEventCount, err = strconv.ParseInt(f.Content.BucketDirs.Home.EventCount, 10, 64)
			if err != nil {
				errs <- err
			}
			s.mb.RecordSplunkDataIndexesExtendedBucketEventCountDataPoint(now, bucketEventCount, name, bucketDir)
		}
		if f.Content.BucketDirs.Thawed.EventCount != "" {
			bucketDir = "thawed"
			bucketEventCount, err = strconv.ParseInt(f.Content.BucketDirs.Thawed.EventCount, 10, 64)
			if err != nil {
				errs <- err
			}
			s.mb.RecordSplunkDataIndexesExtendedBucketEventCountDataPoint(now, bucketEventCount, name, bucketDir)
		}
	}
}

// Scrape indexes extended bucket hot/warm count
func (s *splunkScraper) scrapeIndexesBucketHotWarmCount(ctx context.Context, now pcommon.Timestamp, errs chan error) {
	if !s.conf.MetricsBuilderConfig.Metrics.SplunkDataIndexesExtendedBucketHotCount.Enabled || !s.splunkClient.isConfigured(typeIdx) {
		return
	}

	ctx = context.WithValue(ctx, endpointType("type"), typeIdx)
	var it IndexesExtended

	ept := apiDict[`SplunkDataIndexesExtended`]

	req, err := s.splunkClient.createAPIRequest(ctx, ept)
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
			s.mb.RecordSplunkDataIndexesExtendedBucketHotCountDataPoint(now, bucketHotCount, name, bucketDir)
		}
		if f.Content.BucketDirs.Home.WarmBucketCount != "" {
			bucketWarmCount, err = strconv.ParseInt(f.Content.BucketDirs.Home.WarmBucketCount, 10, 64)
			bucketDir = "warm"
			if err != nil {
				errs <- err
			}
			s.mb.RecordSplunkDataIndexesExtendedBucketWarmCountDataPoint(now, bucketWarmCount, name, bucketDir)
		}
	}
}

// Scrape introspection queues
func (s *splunkScraper) scrapeIntrospectionQueues(ctx context.Context, now pcommon.Timestamp, errs chan error) {
	if !s.conf.MetricsBuilderConfig.Metrics.SplunkServerIntrospectionQueuesCurrent.Enabled || !s.splunkClient.isConfigured(typeIdx) {
		return
	}

	ctx = context.WithValue(ctx, endpointType("type"), typeIdx)
	var it IntrospectionQueues

	ept := apiDict[`SplunkIntrospectionQueues`]

	req, err := s.splunkClient.createAPIRequest(ctx, ept)
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

		s.mb.RecordSplunkServerIntrospectionQueuesCurrentDataPoint(now, currentQueuesSize, name)
	}
}

// Scrape introspection queues bytes
func (s *splunkScraper) scrapeIntrospectionQueuesBytes(ctx context.Context, now pcommon.Timestamp, errs chan error) {
	if !s.conf.MetricsBuilderConfig.Metrics.SplunkServerIntrospectionQueuesCurrentBytes.Enabled || !s.splunkClient.isConfigured(typeIdx) {
		return
	}

	ctx = context.WithValue(ctx, endpointType("type"), typeIdx)
	var it IntrospectionQueues

	ept := apiDict[`SplunkIntrospectionQueues`]

	req, err := s.splunkClient.createAPIRequest(ctx, ept)
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

		s.mb.RecordSplunkServerIntrospectionQueuesCurrentBytesDataPoint(now, currentQueueSizeBytes, name)
	}
}

// Scrape introspection kv store status
func (s *splunkScraper) scrapeKVStoreStatus(ctx context.Context, now pcommon.Timestamp, errs chan error) {
	if !s.conf.MetricsBuilderConfig.Metrics.SplunkKvstoreStatus.Enabled ||
		!s.conf.MetricsBuilderConfig.Metrics.SplunkKvstoreReplicationStatus.Enabled ||
		!s.conf.MetricsBuilderConfig.Metrics.SplunkKvstoreBackupStatus.Enabled ||
		!s.splunkClient.isConfigured(typeCm) {
		return
	}

	ctx = context.WithValue(ctx, endpointType("type"), typeCm)
	var kvs KVStoreStatus

	ept := apiDict[`SplunkKVStoreStatus`]

	req, err := s.splunkClient.createAPIRequest(ctx, ept)
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
			st = KVStatusUnknown
			// set to 0 to indicate no status being reported
			s.mb.RecordSplunkKvstoreStatusDataPoint(now, 0, se, ext, st)
		} else {
			s.mb.RecordSplunkKvstoreStatusDataPoint(now, 1, se, ext, st)
		}

		if rs == "" {
			rs = KVRestoreStatusUnknown
			s.mb.RecordSplunkKvstoreReplicationStatusDataPoint(now, 0, rs)
		} else {
			s.mb.RecordSplunkKvstoreReplicationStatusDataPoint(now, 1, rs)
		}

		if brs == "" {
			brs = KVBackupStatusFailed
			s.mb.RecordSplunkKvstoreBackupStatusDataPoint(now, 0, brs)
		} else {
			s.mb.RecordSplunkKvstoreBackupStatusDataPoint(now, 1, brs)
		}
	}
}

// Scrape dispatch artifacts
func (s *splunkScraper) scrapeSearchArtifacts(ctx context.Context, now pcommon.Timestamp, errs chan error) {
	if !s.splunkClient.isConfigured(typeSh) {
		return
	}

	ctx = context.WithValue(ctx, endpointType("type"), typeSh)
	var da DispatchArtifacts

	ept := apiDict[`SplunkDispatchArtifacts`]

	req, err := s.splunkClient.createAPIRequest(ctx, ept)
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
		if s.conf.MetricsBuilderConfig.Metrics.SplunkServerSearchartifactsAdhoc.Enabled {
			adhocCount, err := strconv.ParseInt(f.Content.AdhocCount, 10, 64)
			if err != nil {
				errs <- err
			}
			s.mb.RecordSplunkServerSearchartifactsAdhocDataPoint(now, adhocCount, s.conf.SHEndpoint.Endpoint)
		}

		if s.conf.MetricsBuilderConfig.Metrics.SplunkServerSearchartifactsScheduled.Enabled {
			scheduledCount, err := strconv.ParseInt(f.Content.ScheduledCount, 10, 64)
			if err != nil {
				errs <- err
			}
			s.mb.RecordSplunkServerSearchartifactsScheduledDataPoint(now, scheduledCount, s.conf.SHEndpoint.Endpoint)
		}

		if s.conf.MetricsBuilderConfig.Metrics.SplunkServerSearchartifactsCompleted.Enabled {
			completedCount, err := strconv.ParseInt(f.Content.CompletedCount, 10, 64)
			if err != nil {
				errs <- err
			}
			s.mb.RecordSplunkServerSearchartifactsCompletedDataPoint(now, completedCount, s.conf.SHEndpoint.Endpoint)
		}

		if s.conf.MetricsBuilderConfig.Metrics.SplunkServerSearchartifactsIncomplete.Enabled {
			incompleteCount, err := strconv.ParseInt(f.Content.IncompleteCount, 10, 64)
			if err != nil {
				errs <- err
			}
			s.mb.RecordSplunkServerSearchartifactsIncompleteDataPoint(now, incompleteCount, s.conf.SHEndpoint.Endpoint)
		}

		if s.conf.MetricsBuilderConfig.Metrics.SplunkServerSearchartifactsInvalid.Enabled {
			invalidCount, err := strconv.ParseInt(f.Content.InvalidCount, 10, 64)
			if err != nil {
				errs <- err
			}
			s.mb.RecordSplunkServerSearchartifactsInvalidDataPoint(now, invalidCount, s.conf.SHEndpoint.Endpoint)
		}

		if s.conf.MetricsBuilderConfig.Metrics.SplunkServerSearchartifactsSavedsearches.Enabled {
			savedSearchesCount, err := strconv.ParseInt(f.Content.SavedSearchesCount, 10, 64)
			if err != nil {
				errs <- err
			}
			s.mb.RecordSplunkServerSearchartifactsSavedsearchesDataPoint(now, savedSearchesCount, s.conf.SHEndpoint.Endpoint)
		}

		if s.conf.MetricsBuilderConfig.Metrics.SplunkServerSearchartifactsJobCacheSize.Enabled {
			infoCacheSize, err := strconv.ParseInt(f.Content.InfoCacheSize, 10, 64)
			if err != nil {
				errs <- err
			}
			statusCacheSize, err := strconv.ParseInt(f.Content.StatusCacheSize, 10, 64)
			if err != nil {
				errs <- err
			}
			s.mb.RecordSplunkServerSearchartifactsJobCacheSizeDataPoint(now, infoCacheSize, s.conf.SHEndpoint.Endpoint, "info")
			s.mb.RecordSplunkServerSearchartifactsJobCacheSizeDataPoint(now, statusCacheSize, s.conf.SHEndpoint.Endpoint, "status")
		}

		if s.conf.MetricsBuilderConfig.Metrics.SplunkServerSearchartifactsJobCacheCount.Enabled {
			cacheTotalEntries, err := strconv.ParseInt(f.Content.CacheTotalEntries, 10, 64)
			if err != nil {
				errs <- err
			}
			s.mb.RecordSplunkServerSearchartifactsJobCacheCountDataPoint(now, cacheTotalEntries, s.conf.SHEndpoint.Endpoint)
		}
	}
}

// Scrape Health Introspection Endpoint
func (s *splunkScraper) scrapeHealth(ctx context.Context, now pcommon.Timestamp, errs chan error) {
	if !s.conf.MetricsBuilderConfig.Metrics.SplunkHealth.Enabled {
		return
	}

	ctx = context.WithValue(ctx, endpointType("type"), typeCm)

	ept := apiDict[`SplunkHealth`]
	var ha healthArtifacts

	req, err := s.splunkClient.createAPIRequest(ctx, ept)
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
		s.traverseHealthDetailFeatures(details.Content, now)
	}
}

func (s *splunkScraper) traverseHealthDetailFeatures(details healthDetails, now pcommon.Timestamp) {
	if details.Features == nil {
		return
	}

	for k, feature := range details.Features {
		if feature.Health != "red" {
			s.settings.Logger.Debug(feature.Health)
			s.mb.RecordSplunkHealthDataPoint(now, 1, k, feature.Health)
		} else {
			s.settings.Logger.Debug(feature.Health)
			s.mb.RecordSplunkHealthDataPoint(now, 0, k, feature.Health)
		}
		s.traverseHealthDetailFeatures(feature, now)
	}
}
