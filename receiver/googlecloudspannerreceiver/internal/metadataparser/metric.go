// Copyright The OpenTelemetry Authors
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
