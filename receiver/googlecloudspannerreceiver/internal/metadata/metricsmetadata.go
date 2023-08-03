// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metadata // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver/internal/metadata"

import (
	"fmt"
	"time"

	"cloud.google.com/go/spanner"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver/internal/datasource"
)

type MetricsMetadataType int32

const (
	MetricsMetadataTypeCurrentStats MetricsMetadataType = iota
	MetricsMetadataTypeIntervalStats
)

type MetricsMetadata struct {
	Name                string
	Query               string
	MetricNamePrefix    string
	TimestampColumnName string
	HighCardinality     bool
	// In addition to common metric labels
	QueryLabelValuesMetadata  []LabelValueMetadata
	QueryMetricValuesMetadata []MetricValueMetadata
}

func (metadata *MetricsMetadata) timestamp(row *spanner.Row) (time.Time, error) {
	if metadata.MetadataType() == MetricsMetadataTypeCurrentStats {
		return time.Now().UTC(), nil
	}
	var timestamp time.Time
	err := row.ColumnByName(metadata.TimestampColumnName, &timestamp)
	return timestamp, err
}

func (metadata *MetricsMetadata) toLabelValues(row *spanner.Row) ([]LabelValue, error) {
	values := make([]LabelValue, len(metadata.QueryLabelValuesMetadata))

	for i, metadataItems := range metadata.QueryLabelValuesMetadata {
		var err error

		if values[i], err = toLabelValue(metadataItems, row); err != nil {
			return nil, err
		}
	}

	return values, nil
}

func toLabelValue(labelValueMetadata LabelValueMetadata, row *spanner.Row) (LabelValue, error) {
	valueHolder := labelValueMetadata.ValueHolder()

	err := row.ColumnByName(labelValueMetadata.ColumnName(), valueHolder)
	if err != nil {
		return nil, err
	}

	return labelValueMetadata.NewLabelValue(valueHolder), nil
}

func (metadata *MetricsMetadata) toMetricValues(row *spanner.Row) ([]MetricValue, error) {
	values := make([]MetricValue, len(metadata.QueryMetricValuesMetadata))

	for i, metadataItems := range metadata.QueryMetricValuesMetadata {
		var err error

		if values[i], err = toMetricValue(metadataItems, row); err != nil {
			return nil, err
		}
	}

	return values, nil
}

func toMetricValue(metricValueMetadata MetricValueMetadata, row *spanner.Row) (MetricValue, error) {
	valueHolder := metricValueMetadata.ValueHolder()

	err := row.ColumnByName(metricValueMetadata.ColumnName(), valueHolder)
	if err != nil {
		return nil, err
	}

	return metricValueMetadata.NewMetricValue(valueHolder), nil
}

func (metadata *MetricsMetadata) RowToMetricsDataPoints(databaseID *datasource.DatabaseID, row *spanner.Row) ([]*MetricsDataPoint, error) {
	timestamp, err := metadata.timestamp(row)
	if err != nil {
		return nil, fmt.Errorf("error occurred during extracting timestamp %w", err)
	}

	// Reading labels
	labelValues, err := metadata.toLabelValues(row)
	if err != nil {
		return nil, fmt.Errorf("error occurred during extracting label values for row: %w", err)
	}

	// Reading metrics
	metricValues, err := metadata.toMetricValues(row)
	if err != nil {
		return nil, fmt.Errorf("error occurred during extracting metric values row: %w", err)
	}

	return metadata.toMetricsDataPoints(databaseID, timestamp, labelValues, metricValues), nil
}

func (metadata *MetricsMetadata) toMetricsDataPoints(databaseID *datasource.DatabaseID, timestamp time.Time,
	labelValues []LabelValue, metricValues []MetricValue) []*MetricsDataPoint {

	var dataPoints []*MetricsDataPoint

	for _, metricValue := range metricValues {
		dataPoint := &MetricsDataPoint{
			metricName:  metadata.MetricNamePrefix + metricValue.Metadata().Name(),
			timestamp:   timestamp,
			databaseID:  databaseID,
			labelValues: labelValues,
			metricValue: metricValue,
		}
		dataPoints = append(dataPoints, dataPoint)
	}

	return dataPoints
}

func (metadata *MetricsMetadata) MetadataType() MetricsMetadataType {
	if metadata.TimestampColumnName == "" {
		return MetricsMetadataTypeCurrentStats
	}
	return MetricsMetadataTypeIntervalStats
}
