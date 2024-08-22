// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlecloudmonitoringreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudmonitoringreceiver"

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	monitoring "cloud.google.com/go/monitoring/apiv3/v2"
	"cloud.google.com/go/monitoring/apiv3/v2/monitoringpb"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
	"google.golang.org/genproto/googleapis/api/metric"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudmonitoringreceiver/internal"
)

type monitoringReceiver struct {
	config         *Config
	logger         *zap.Logger
	client         *monitoring.MetricClient
	metricsBuilder *internal.MetricsBuilder
	mutex          sync.Mutex
}

func newGoogleCloudMonitoringReceiver(cfg *Config, logger *zap.Logger) *monitoringReceiver {
	return &monitoringReceiver{
		config:         cfg,
		logger:         logger,
		metricsBuilder: internal.NewMetricsBuilder(logger),
	}
}

func (mr *monitoringReceiver) Start(ctx context.Context, _ component.Host) error {
	var client *monitoring.MetricClient
	var err error

	// If the client is already initialized, return nil
	if mr.client != nil {
		return nil
	}

	// Lock to ensure thread-safe access to mr.client
	mr.mutex.Lock()
	defer mr.mutex.Unlock()

	// Get service account key file path
	serAccKey := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")
	if serAccKey != "" {
		// Use provided credentials file
		credentialsFileClientOption := option.WithCredentialsFile(serAccKey)
		client, err = monitoring.NewMetricClient(ctx, credentialsFileClientOption)
	} else {
		// Set default credentials file path for testing
		os.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "testdata/serviceAccount.json")

		// Fallback to Application Default Credentials(https://google.aip.dev/auth/4110)
		client, err = monitoring.NewMetricClient(ctx)
	}

	// Attempt to create the monitoring client
	if err != nil {
		return fmt.Errorf("failed to create a monitoring client: %v", err)
	}
	mr.client = client
	mr.logger.Info("Monitoring client successfully created.")

	// Return nil after client creation
	return nil
}

func (mr *monitoringReceiver) Shutdown(context.Context) error {
	var err error
	if mr.client != nil {
		err = mr.client.Close()
	}
	return err
}

func (mr *monitoringReceiver) Scrape(ctx context.Context) (pmetric.Metrics, error) {
	var (
		gInternal            time.Duration
		gDelay               time.Duration
		calStartTime         time.Time
		calEndTime           time.Time
		filterQuery          string
		allTimeSeriesMetrics []*monitoringpb.TimeSeries
		gErr                 error
	)

	// Iterate over each metric in the configuration to calculate start/end times and construct the filter query.
	for _, metric := range mr.config.MetricsList {
		// Set interval and delay times, using defaults if not provided
		gInternal = mr.config.CollectionInterval
		if gInternal <= 0 {
			gInternal = defaultCollectionInterval
		}

		gDelay = metric.FetchDelay
		if gDelay <= 0 {
			gDelay = defaultFetchDelay
		}

		// Calculate the start and end times
		calStartTime, calEndTime = calculateStartEndTime(gInternal, gDelay)

		// Get the filter query for the metric
		filterQuery = getFilterQuery(metric)

		// Define the request to list time series data
		req := &monitoringpb.ListTimeSeriesRequest{
			Name:   "projects/" + mr.config.ProjectID,
			Filter: filterQuery,
			Interval: &monitoringpb.TimeInterval{
				EndTime:   &timestamppb.Timestamp{Seconds: calEndTime.Unix()},
				StartTime: &timestamppb.Timestamp{Seconds: calStartTime.Unix()},
			},
			View: monitoringpb.ListTimeSeriesRequest_FULL,
		}

		// Create an iterator for the time series data
		it := mr.client.ListTimeSeries(ctx, req)
		mr.logger.Debug("Retrieving time series data")

		var metrics pmetric.Metrics
		// Iterate over the time series data
		for {
			timeSeriesMetrics, err := it.Next()
			if errors.Is(err, iterator.Done) {
				break
			}

			// Handle errors and break conditions for the iterator
			if err != nil {
				gErr = fmt.Errorf("failed to retrieve time series data: %w", err)
				return metrics, gErr
			}

			allTimeSeriesMetrics = append(allTimeSeriesMetrics, timeSeriesMetrics)
		}
	}

	// Convert the GCP TimeSeries to pmetric.Metrics format of OpenTelemetry
	metrics := mr.convertGCPTimeSeriesToMetrics(allTimeSeriesMetrics)

	return metrics, gErr
}

// calculateStartEndTime calculates the start and end times based on the current time, interval, and delay.
func calculateStartEndTime(interval, delay time.Duration) (time.Time, time.Time) {
	// Get the current time
	now := time.Now()

	// Calculate end time by subtracting delay
	endTime := now.Add(-delay)

	// Calculate start time by subtracting interval from end time
	startTime := endTime.Add(-interval)

	// Return start and end times
	return startTime, endTime
}

// getFilterQuery constructs a filter query string based on the provided metric.
func getFilterQuery(metric MetricConfig) string {
	var filterQuery string
	const baseQuery = `metric.type =`

	// If a specific metric name is provided, use it in the filter query
	filterQuery = fmt.Sprintf(`%s "%s"`, baseQuery, metric.MetricName)
	return filterQuery
}

// ConvertGCPTimeSeriesToMetrics converts GCP Monitoring TimeSeries to pmetric.Metrics
func (mr *monitoringReceiver) convertGCPTimeSeriesToMetrics(timeSeriesMetrics []*monitoringpb.TimeSeries) pmetric.Metrics {
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()

	for _, resp := range timeSeriesMetrics {
		m := sm.Metrics().AppendEmpty()
		// Set metric name and description
		m.SetName(resp.Metric.Type)
		m.SetUnit(resp.Unit)

		// TODO: Retrieve and cache MetricDescriptor to set the correct description
		m.SetDescription("Converted from GCP Monitoring TimeSeries")

		// Set resource labels
		resource := rm.Resource()
		resource.Attributes().PutStr("resource_type", resp.Resource.Type)
		for k, v := range resp.Resource.Labels {
			resource.Attributes().PutStr(k, v)
		}

		// Set metadata (user and system labels)
		if resp.Metadata != nil {
			for k, v := range resp.Metadata.UserLabels {
				resource.Attributes().PutStr(k, v)
			}
			if resp.Metadata.SystemLabels != nil {
				for k, v := range resp.Metadata.SystemLabels.Fields {
					resource.Attributes().PutStr(k, fmt.Sprintf("%v", v))
				}
			}
		}

		switch resp.GetMetricKind() {
		case metric.MetricDescriptor_GAUGE:
			mr.metricsBuilder.ConvertGaugeToMetrics(resp, m)
		case metric.MetricDescriptor_CUMULATIVE:
			mr.metricsBuilder.ConvertSumToMetrics(resp, m)
		case metric.MetricDescriptor_DELTA:
			mr.metricsBuilder.ConvertDeltaToMetrics(resp, m)
		// TODO: Add support for HISTOGRAM
		// TODO: Add support for EXPONENTIAL_HISTOGRAM
		default:
			metricError := fmt.Sprintf("\n Unsupported metric kind: %v\n", resp.GetMetricKind())
			mr.logger.Info(metricError)
		}
	}

	return metrics
}
