// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlecloudmonitoringreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudmonitoringreceiver"

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	monitoring "cloud.google.com/go/monitoring/apiv3/v2"
	"cloud.google.com/go/monitoring/apiv3/v2/monitoringpb"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
	"google.golang.org/genproto/googleapis/api/metric"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudmonitoringreceiver/internal"
)

type monitoringReceiver struct {
	config            *Config
	logger            *zap.Logger
	client            *monitoring.MetricClient
	metricsBuilder    *internal.MetricsBuilder
	mutex             sync.RWMutex
	metricDescriptors map[string]*metric.MetricDescriptor // key is the Type of MetricDescriptor
}

func newGoogleCloudMonitoringReceiver(cfg *Config, logger *zap.Logger) *monitoringReceiver {
	return &monitoringReceiver{
		config:            cfg,
		logger:            logger,
		metricsBuilder:    internal.NewMetricsBuilder(logger),
		metricDescriptors: make(map[string]*metric.MetricDescriptor),
	}
}

func (mr *monitoringReceiver) Start(ctx context.Context, _ component.Host) error {
	// Lock to ensure thread-safe access to mr.client
	mr.mutex.Lock()
	defer mr.mutex.Unlock()

	// Skip client initialization if already initialized
	if mr.client == nil {
		if err := mr.initializeClient(ctx); err != nil {
			return err
		}
		mr.logger.Info("Monitoring client successfully created.")
	}

	// Initialize metric descriptors, even if the client was previously initialized
	if len(mr.metricDescriptors) == 0 {
		if err := mr.initializeMetricDescriptors(ctx); err != nil {
			return err
		}
	}

	return nil
}

func (mr *monitoringReceiver) Shutdown(context.Context) error {
	mr.mutex.Lock()
	defer mr.mutex.Unlock()

	var err error
	if mr.client != nil {
		err = mr.client.Close()
	}
	return err
}

func (mr *monitoringReceiver) Scrape(ctx context.Context) (pmetric.Metrics, error) {
	var (
		gInterval    time.Duration
		gDelay       time.Duration
		calStartTime time.Time
		calEndTime   time.Time
		filterQuery  string
		gErr         error
	)

	metrics := pmetric.NewMetrics()

	// Iterate over each metric in the configuration to calculate start/end times and construct the filter query.
	mr.mutex.RLock()
	defer mr.mutex.RUnlock()
	for metricType, metricDesc := range mr.metricDescriptors {
		// Set interval and delay times, using defaults if not provided
		gInterval = mr.config.CollectionInterval
		if gInterval <= 0 {
			gInterval = defaultCollectionInterval
		}

		gDelay = metricDesc.GetMetadata().GetIngestDelay().AsDuration()
		if gDelay <= 0 {
			gDelay = defaultFetchDelay
		}

		// Calculate the start and end times
		calStartTime, calEndTime = calculateStartEndTime(gInterval, gDelay)

		// Get the filter query for the metric
		filterQuery = fmt.Sprintf(`metric.type = "%s"`, metricType)

		// Define the request to list time series data
		tsReq := &monitoringpb.ListTimeSeriesRequest{
			Name:   "projects/" + mr.config.ProjectID,
			Filter: filterQuery,
			Interval: &monitoringpb.TimeInterval{
				EndTime:   &timestamppb.Timestamp{Seconds: calEndTime.Unix()},
				StartTime: &timestamppb.Timestamp{Seconds: calStartTime.Unix()},
			},
			View: monitoringpb.ListTimeSeriesRequest_FULL,
		}

		// Create an iterator for the time series data
		tsIter := mr.client.ListTimeSeries(ctx, tsReq)
		mr.logger.Debug("Retrieving time series data for metric", zap.String("name", metricDesc.Type))

		// Iterate over the time series data
		for {
			timeSeries, err := tsIter.Next()
			if errors.Is(err, iterator.Done) {
				break
			}

			// Handle errors and break conditions for the iterator
			if err != nil {
				gErr = fmt.Errorf("failed to retrieve time series data: %w", err)
				return metrics, gErr
			}

			// Convert and append the metric directly within the loop
			mr.convertGCPTimeSeriesToMetrics(metrics, metricDesc, timeSeries)
		}
	}

	return metrics, gErr
}

// initializeClient handles the creation of the monitoring client
func (mr *monitoringReceiver) initializeClient(ctx context.Context) error {
	// Use google.FindDefaultCredentials to find the credentials
	creds, err := google.FindDefaultCredentials(ctx, "https://www.googleapis.com/auth/monitoring.read")
	if err != nil {
		return fmt.Errorf("failed to find default credentials: %w", err)
	}

	// Attempt to create the monitoring client
	client, err := monitoring.NewMetricClient(ctx, option.WithCredentials(creds))
	if err != nil {
		return fmt.Errorf("failed to create a monitoring client: %w", err)
	}

	mr.client = client
	return nil
}

// initializeMetricDescriptors handles the retrieval and processing of metric descriptors
func (mr *monitoringReceiver) initializeMetricDescriptors(ctx context.Context) error {
	// Call the metricDescriptorAPI method to start processing metric descriptors.
	if err := mr.metricDescriptorAPI(ctx); err != nil {
		return err
	}

	return nil
}

// metricDescriptorAPI fetches and processes metric descriptors from the monitoring API.
func (mr *monitoringReceiver) metricDescriptorAPI(ctx context.Context) error {
	// Iterate over each metric in the configuration to calculate start/end times and construct the filter query.
	for _, metric := range mr.config.MetricsList {
		// Get the filter query for the metric
		filterQuery := getFilterQuery(metric)

		// Define the request to list metric descriptors
		metricReq := &monitoringpb.ListMetricDescriptorsRequest{
			Name:   "projects/" + mr.config.ProjectID,
			Filter: filterQuery,
		}

		// Create an iterator for the metric descriptors
		metricIter := mr.client.ListMetricDescriptors(ctx, metricReq)

		// Iterate over the time series data
		for {
			metricDesc, err := metricIter.Next()
			if errors.Is(err, iterator.Done) {
				break
			}

			// Handle errors and break conditions for the iterator
			if err != nil {
				return fmt.Errorf("failed to retrieve metric descriptors data: %w", err)
			}
			mr.metricDescriptors[metricDesc.Type] = metricDesc
		}
	}

	mr.logger.Info("Successfully retrieved all metric descriptors.")
	return nil
}

// calculateStartEndTime calculates the start and end times based on the current time, interval, and delay.
// It enforces a maximum interval of 23 hours to avoid querying data older than 24 hours.
func calculateStartEndTime(interval, delay time.Duration) (time.Time, time.Time) {
	const maxInterval = 23 * time.Hour // Maximum allowed interval is 23 hours

	// Get the current time
	now := time.Now()

	// Cap the interval at 23 hours if it exceeds that
	if interval > maxInterval {
		interval = maxInterval
	}

	// Calculate end time by subtracting delay
	endTime := now.Add(-delay)

	// Calculate start time by subtracting the interval from the end time
	startTime := endTime.Add(-interval)

	// Return start and end times
	return startTime, endTime
}

// getFilterQuery constructs a filter query string based on the provided metric.
func getFilterQuery(metric MetricConfig) string {
	var filterQuery string

	// see https://cloud.google.com/monitoring/api/v3/filters
	if metric.MetricName != "" {
		filterQuery = fmt.Sprintf(`metric.type = "%s"`, metric.MetricName)
	} else {
		filterQuery = metric.MetricDescriptorFilter
	}

	return filterQuery
}

// ConvertGCPTimeSeriesToMetrics converts GCP Monitoring TimeSeries to pmetric.Metrics
func (mr *monitoringReceiver) convertGCPTimeSeriesToMetrics(metrics pmetric.Metrics, metricDesc *metric.MetricDescriptor, timeSeries *monitoringpb.TimeSeries) {
	// Map to track existing ResourceMetrics by resource attributes
	resourceMetricsMap := make(map[string]pmetric.ResourceMetrics)

	// Generate a unique key based on resource attributes
	resourceKey := generateResourceKey(timeSeries.Resource.Type, timeSeries.Resource.Labels, timeSeries)

	// Check if ResourceMetrics for this resource already exists
	rm, exists := resourceMetricsMap[resourceKey]

	if !exists {
		// Create a new ResourceMetrics if not already present
		rm = metrics.ResourceMetrics().AppendEmpty()

		// Set resource labels
		resource := rm.Resource()
		resource.Attributes().PutStr("gcp.resource_type", timeSeries.Resource.Type)
		for k, v := range timeSeries.Resource.Labels {
			resource.Attributes().PutStr(k, v)
		}

		// Set metadata (user and system labels)
		if timeSeries.GetMetadata() != nil {
			for k, v := range timeSeries.GetMetadata().GetUserLabels() {
				resource.Attributes().PutStr(k, v)
			}
			if timeSeries.GetMetadata().GetSystemLabels() != nil {
				for k, v := range timeSeries.GetMetadata().GetSystemLabels().GetFields() {
					resource.Attributes().PutStr(k, fmt.Sprintf("%v", v))
				}
			}
		}

		// Add metric-specific labels if they are present
		if len(timeSeries.GetMetric().Labels) > 0 {
			for k, v := range timeSeries.GetMetric().GetLabels() {
				resource.Attributes().PutStr(k, fmt.Sprintf("%v", v))
			}
		}

		// Store the newly created ResourceMetrics in the map
		resourceMetricsMap[resourceKey] = rm
	}

	// Ensure we have a ScopeMetrics to append the metric to
	var sm pmetric.ScopeMetrics
	if rm.ScopeMetrics().Len() == 0 {
		sm = rm.ScopeMetrics().AppendEmpty()
	} else {
		// For simplicity, let's assume all metrics will share the same ScopeMetrics
		sm = rm.ScopeMetrics().At(0)
	}

	// Create a new Metric
	m := sm.Metrics().AppendEmpty()

	// Set metric name, description, and unit
	m.SetName(metricDesc.GetName())
	m.SetDescription(metricDesc.GetDescription())
	m.SetUnit(metricDesc.Unit)

	switch timeSeries.GetValueType() {
	case metric.MetricDescriptor_DISTRIBUTION:
		switch timeSeries.GetMetricKind() {
		case metric.MetricDescriptor_DELTA:
			mr.metricsBuilder.ConvertDistributionToMetrics(timeSeries, m)
		default:
			metricError := fmt.Sprintf("\n Unsupported distribution metric kind: %v\n", timeSeries.GetMetricKind())
			mr.logger.Warn(metricError)
		}
	default:
		// Convert the TimeSeries to the appropriate metric type
		switch timeSeries.GetMetricKind() {
		case metric.MetricDescriptor_GAUGE:
			mr.metricsBuilder.ConvertGaugeToMetrics(timeSeries, m)
		case metric.MetricDescriptor_CUMULATIVE:
			mr.metricsBuilder.ConvertSumToMetrics(timeSeries, m)
		case metric.MetricDescriptor_DELTA:
			mr.metricsBuilder.ConvertDeltaToMetrics(timeSeries, m)
		default:
			metricError := fmt.Sprintf("\n Unsupported metric kind: %v\n", timeSeries.GetMetricKind())
			mr.logger.Warn(metricError)
		}
	}
}

// Helper function to generate a unique key for a resource based on its attributes
func generateResourceKey(resourceType string, labels map[string]string, timeSeries *monitoringpb.TimeSeries) string {
	key := resourceType
	for k, v := range labels {
		key += k + v
	}
	if timeSeries != nil {
		for k, v := range timeSeries.Metric.Labels {
			key += k + v
		}
		if timeSeries.Resource.Labels != nil {
			for k, v := range timeSeries.Resource.Labels {
				key += k + v
			}
		}
	}
	return key
}
