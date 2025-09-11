// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metadataparser // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver/internal/metadataparser"

import (
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver/internal/metadata"
)

type Metric struct {
	Label    `yaml:",inline"`
	DataType MetricType `yaml:"data"`
	Unit     string     `yaml:"unit"`
}

func (metric Metric) toMetricValueMetadata() (metadata.MetricValueMetadata, error) {
	dataType, err := metric.DataType.toMetricType()
	if err != nil {
		return nil, fmt.Errorf("invalid value data type received for metric %q", metric.Name)
	}

	return metadata.NewMetricValueMetadata(metric.Name, metric.ColumnName, dataType, metric.Unit, metric.ValueType)
}
