// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlecloudmonitoringreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudmonitoringreceiver"

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"google.golang.org/api/iterator"
	"google.golang.org/genproto/googleapis/api/metric"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudmonitoringreceiver/internal"

	"cloud.google.com/go/monitoring/apiv3/v2/monitoringpb"
	
	monitoring "cloud.google.com/go/monitoring/apiv3/v2"
)

type monitoringReceiver struct {
	config         *Config
	logger         *zap.Logger
	client         *monitoring.MetricClient
	metricsBuilder *internal.MetricsBuilder
	startOnce      sync.Once
}

func newGoogleCloudMonitoringReceiver(cfg *Config, logger *zap.Logger) *monitoringReceiver {
	return &monitoringReceiver{
		config:         cfg,
		logger:         logger,
		metricsBuilder: internal.NewMetricsBuilder(logger),
	}
}

func (mr *monitoringReceiver) Start(ctx context.Context, _ component.Host) error {
	var startErr error
	mr.startOnce.Do(func() {
		client, err := monitoring.NewMetricClient(ctx)
		if err != nil {
			startErr = fmt.Errorf("failed to create a monitoring client: %v", err)
			return
		}

		mr.client = client
	})

	return startErr
}

func (m *monitoringReceiver) Shutdown(context.Context) error {
	m.logger.Debug("shutting down googlecloudmonitoringreceiver receiver")
	return nil
}

func (mr *monitoringReceiver) Scrape(ctx context.Context) (pmetric.Metrics, error) {
	var (
		calStartTime         time.Time
		calEndTime           time.Time
		filterQuery          string
		allTimeSeriesMetrics []*monitoringpb.TimeSeries
		gErr                 error
	)

	// Iterate over each metric in the configuration to calculate start/end times and construct the filter query.
	for _, metric := range mr.config.MetricsList {
		// Define the interval and delay times
		interval := mr.config.CollectionInterval
		delay := metric.Delay

		// Calculate the start and end times
		calStartTime, calEndTime = calculateStartEndTime(interval, delay)

		// Get the filter query for the metric
		filterQuery = getFilterQuery(metric)

		// Log an error if the filter query is empty
		if filterQuery == "" {
			mr.logger.Error("Internal Server Error")
		}

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
		mr.logger.Info("Time series data:")

		var metrics pmetric.Metrics
		// Iterate over the time series data
		for {
			timeSeriesMetrics, err := it.Next()
			if timeSeriesMetrics == nil && err != nil {
				if err == iterator.Done {
					mr.logger.Info(iterator.Done.Error())
					break
				}
			}

			// Handle errors and break conditions for the iterator
			if err != nil {
				err := fmt.Errorf("failed to retrieve time series data: %v", err)
				gErr = multierr.Append(gErr, err)
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

	// Calculate the start time (current time - delay)
	startTime := now.Add(-delay - interval)

	// Calculate the end time (start time + interval)
	endTime := startTime.Add(interval)

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

		// Assuming MetricDescriptor and description are set
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
		// Add cases for SUMMARY, HISTOGRAM, EXPONENTIAL_HISTOGRAM if needed
		default:
			metricError := fmt.Sprintf("\n Unsupported metric kind: %v\n", resp.GetMetricKind())
			mr.logger.Info(metricError)
		}
	}

	return metrics
}
