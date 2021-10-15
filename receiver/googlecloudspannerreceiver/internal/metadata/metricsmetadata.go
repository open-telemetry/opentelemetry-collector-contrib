// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package metadata

import (
	"fmt"
	"time"

	"cloud.google.com/go/spanner"
	"go.opentelemetry.io/collector/model/pdata"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver/internal/datasource"
)

const (
	projectIDLabelName  = "project_id"
	instanceIDLabelName = "instance_id"
	databaseLabelName   = "database"
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

	var value LabelValue

	switch labelValueMetadataCasted := labelValueMetadata.(type) {
	case StringLabelValueMetadata:
		value = newStringLabelValue(labelValueMetadataCasted, valueHolder)
	case Int64LabelValueMetadata:
		value = newInt64LabelValue(labelValueMetadataCasted, valueHolder)
	case BoolLabelValueMetadata:
		value = newBoolLabelValue(labelValueMetadataCasted, valueHolder)
	case StringSliceLabelValueMetadata:
		value = newStringSliceLabelValue(labelValueMetadataCasted, valueHolder)
	case ByteSliceLabelValueMetadata:
		value = newByteSliceLabelValue(labelValueMetadataCasted, valueHolder)
	}

	return value, nil
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

	var value MetricValue

	switch metricValueMetadataCasted := metricValueMetadata.(type) {
	case Int64MetricValueMetadata:
		value = newInt64MetricValue(metricValueMetadataCasted, valueHolder)
	case Float64MetricValueMetadata:
		value = newFloat64MetricValue(metricValueMetadataCasted, valueHolder)
	}

	return value, nil
}

func (metadata *MetricsMetadata) RowToMetrics(databaseID *datasource.DatabaseID, row *spanner.Row) ([]pdata.Metrics, error) {
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

	return metadata.toMetrics(databaseID, timestamp, labelValues, metricValues), nil
}

func (metadata *MetricsMetadata) toMetrics(databaseID *datasource.DatabaseID, timestamp time.Time,
	labelValues []LabelValue, metricValues []MetricValue) []pdata.Metrics {

	var metrics []pdata.Metrics

	for _, metricValue := range metricValues {
		md := pdata.NewMetrics()
		rms := md.ResourceMetrics()
		rm := rms.AppendEmpty()

		ilms := rm.InstrumentationLibraryMetrics()
		ilm := ilms.AppendEmpty()
		metric := ilm.Metrics().AppendEmpty()
		metric.SetName(metadata.MetricNamePrefix + metricValue.Name())
		metric.SetUnit(metricValue.Unit())
		metric.SetDataType(metricValue.DataType().MetricDataType())

		var dataPoints pdata.NumberDataPointSlice

		switch metricValue.DataType().MetricDataType() {
		case pdata.MetricDataTypeGauge:
			dataPoints = metric.Gauge().DataPoints()
		case pdata.MetricDataTypeSum:
			metric.Sum().SetAggregationTemporality(metricValue.DataType().AggregationTemporality())
			metric.Sum().SetIsMonotonic(metricValue.DataType().IsMonotonic())
			dataPoints = metric.Sum().DataPoints()
		}

		dataPoint := dataPoints.AppendEmpty()

		switch valueCasted := metricValue.(type) {
		case float64MetricValue:
			dataPoint.SetDoubleVal(valueCasted.value)
		case int64MetricValue:
			dataPoint.SetIntVal(valueCasted.value)
		}

		dataPoint.SetTimestamp(pdata.NewTimestampFromTime(timestamp))

		for _, labelValue := range labelValues {
			switch valueCasted := labelValue.(type) {
			case stringLabelValue:
				dataPoint.Attributes().InsertString(valueCasted.name, valueCasted.value)
			case boolLabelValue:
				dataPoint.Attributes().InsertBool(valueCasted.name, valueCasted.value)
			case int64LabelValue:
				dataPoint.Attributes().InsertInt(valueCasted.name, valueCasted.value)
			case stringSliceLabelValue:
				dataPoint.Attributes().InsertString(valueCasted.name, valueCasted.value)
			case byteSliceLabelValue:
				dataPoint.Attributes().InsertString(valueCasted.name, valueCasted.value)
			}
		}

		dataPoint.Attributes().InsertString(projectIDLabelName, databaseID.ProjectID())
		dataPoint.Attributes().InsertString(instanceIDLabelName, databaseID.InstanceID())
		dataPoint.Attributes().InsertString(databaseLabelName, databaseID.DatabaseName())

		metrics = append(metrics, md)
	}

	return metrics
}

func (metadata *MetricsMetadata) MetadataType() MetricsMetadataType {
	if metadata.TimestampColumnName == "" {
		return MetricsMetadataTypeCurrentStats
	}
	return MetricsMetadataTypeIntervalStats
}
