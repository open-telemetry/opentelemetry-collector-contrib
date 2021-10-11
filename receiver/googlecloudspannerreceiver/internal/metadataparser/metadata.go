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

package metadataparser

import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver/internal/metadata"

type Metadata struct {
	Name                string   `yaml:"name"`
	Query               string   `yaml:"query"`
	MetricNamePrefix    string   `yaml:"metric_name_prefix"`
	TimestampColumnName string   `yaml:"timestamp_column_name"`
	Labels              []Label  `yaml:"labels"`
	Metrics             []Metric `yaml:"metrics"`
}

func (m Metadata) MetricsMetadata() (*metadata.MetricsMetadata, error) {
	queryLabelValuesMetadata, err := m.toLabelValuesMetadata()
	if err != nil {
		return nil, err
	}

	queryMetricValuesMetadata, err := m.toMetricValuesMetadata()
	if err != nil {
		return nil, err
	}

	return &metadata.MetricsMetadata{
		Name:                      m.Name,
		Query:                     m.Query,
		MetricNamePrefix:          m.MetricNamePrefix,
		TimestampColumnName:       m.TimestampColumnName,
		QueryLabelValuesMetadata:  queryLabelValuesMetadata,
		QueryMetricValuesMetadata: queryMetricValuesMetadata,
	}, nil
}

func (m Metadata) toLabelValuesMetadata() ([]metadata.LabelValueMetadata, error) {
	valuesMetadata := make([]metadata.LabelValueMetadata, len(m.Labels))

	for i, label := range m.Labels {
		value, err := label.toLabelValueMetadata()
		if err != nil {
			return nil, err
		}

		valuesMetadata[i] = value
	}

	return valuesMetadata, nil
}

func (m Metadata) toMetricValuesMetadata() ([]metadata.MetricValueMetadata, error) {
	valuesMetadata := make([]metadata.MetricValueMetadata, len(m.Metrics))
	for i, metric := range m.Metrics {
		value, err := metric.toMetricValueMetadata()
		if err != nil {
			return nil, err
		}

		valuesMetadata[i] = value
	}

	return valuesMetadata, nil
}
