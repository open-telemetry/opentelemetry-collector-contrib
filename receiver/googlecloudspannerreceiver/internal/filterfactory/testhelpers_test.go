// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filterfactory

import (
	"github.com/stretchr/testify/mock"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver/internal/filter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver/internal/metadata"
)

const (
	metricFullName = "metricFullName"
	prefix1        = "prefix1-"
	prefix2        = "prefix2-"
)

type mockFilter struct {
	mock.Mock
}

func (f *mockFilter) Filter(source []*filter.Item) ([]*filter.Item, error) {
	return source, nil
}

func (f *mockFilter) Shutdown() error {
	args := f.Called()
	return args.Error(0)
}

func (f *mockFilter) TotalLimit() int {
	return 0
}

func (f *mockFilter) LimitByTimestamp() int {
	return 0
}

func generateMetadataItems(prefixes []string, prefixHighCardinality []bool) []*metadata.MetricsMetadata {
	metricDataType := metadata.NewMetricType(pmetric.MetricTypeGauge, pmetric.AggregationTemporalityUnspecified, false)
	metadataItems := make([]*metadata.MetricsMetadata, len(prefixes))
	int64MetricValueMetadata, _ := metadata.NewMetricValueMetadata("int64", "int64Column", metricDataType, "int64Unit", metadata.IntValueType)
	float64MetricValueMetadata, _ := metadata.NewMetricValueMetadata("float64", "float64Column", metricDataType, "float64Unit", metadata.FloatValueType)

	for i, prefix := range prefixes {
		metadataItems[i] = &metadata.MetricsMetadata{
			MetricNamePrefix: prefix,
			HighCardinality:  prefixHighCardinality[i],
			QueryMetricValuesMetadata: []metadata.MetricValueMetadata{
				int64MetricValueMetadata,
				float64MetricValueMetadata,
			},
		}
	}

	return metadataItems
}
