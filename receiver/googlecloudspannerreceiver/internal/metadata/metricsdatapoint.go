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
	"time"

	"go.opentelemetry.io/collector/model/pdata"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver/internal/datasource"
)

type MetricsDataPointKey struct {
	MetricName     string
	MetricUnit     string
	MetricDataType MetricDataType
}

type MetricsDataPoint struct {
	metricName  string
	timestamp   time.Time
	databaseID  *datasource.DatabaseID
	labelValues []LabelValue
	metricValue MetricValue
}

func (mdp *MetricsDataPoint) CopyTo(dataPoint pdata.NumberDataPoint) {
	dataPoint.SetTimestamp(pdata.NewTimestampFromTime(mdp.timestamp))

	mdp.metricValue.SetValueTo(dataPoint)

	attributes := dataPoint.Attributes()

	for _, labelValue := range mdp.labelValues {
		labelValue.SetValueTo(attributes)
	}

	dataPoint.Attributes().InsertString(projectIDLabelName, mdp.databaseID.ProjectID())
	dataPoint.Attributes().InsertString(instanceIDLabelName, mdp.databaseID.InstanceID())
	dataPoint.Attributes().InsertString(databaseLabelName, mdp.databaseID.DatabaseName())
}

func (mdp *MetricsDataPoint) GroupingKey() MetricsDataPointKey {
	return MetricsDataPointKey{
		MetricName:     mdp.metricName,
		MetricUnit:     mdp.metricValue.Unit(),
		MetricDataType: mdp.metricValue.DataType(),
	}
}
