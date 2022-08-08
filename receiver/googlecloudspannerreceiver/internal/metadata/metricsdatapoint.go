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

package metadata // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver/internal/metadata"

import (
	"fmt"
	"time"

	"github.com/mitchellh/hashstructure"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver/internal/datasource"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver/internal/filter"
)

const (
	projectIDLabelName  = "project_id"
	instanceIDLabelName = "instance_id"
	databaseLabelName   = "database"
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

// Fields must be exported for hashing purposes
type dataForHashing struct {
	MetricName string
	Labels     []label
}

// Fields must be exported for hashing purposes
type label struct {
	Name  string
	Value interface{}
}

func (mdp *MetricsDataPoint) CopyTo(dataPoint pmetric.NumberDataPoint) {
	dataPoint.SetTimestamp(pcommon.NewTimestampFromTime(mdp.timestamp))

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
		MetricUnit:     mdp.metricValue.Metadata().Unit(),
		MetricDataType: mdp.metricValue.Metadata().DataType(),
	}
}

func (mdp *MetricsDataPoint) ToItem() (*filter.Item, error) {
	seriesKey, err := mdp.hash()
	if err != nil {
		return nil, err
	}

	return &filter.Item{
		SeriesKey: seriesKey,
		Timestamp: mdp.timestamp,
	}, nil
}

func (mdp *MetricsDataPoint) toDataForHashing() dataForHashing {
	// Do not use map here because it has unpredicted order
	// Taking into account 3 default labels: project_id, instance_id, database
	labels := make([]label, len(mdp.labelValues)+3)

	labels[0] = label{Name: projectIDLabelName, Value: mdp.databaseID.ProjectID()}
	labels[1] = label{Name: instanceIDLabelName, Value: mdp.databaseID.InstanceID()}
	labels[2] = label{Name: databaseLabelName, Value: mdp.databaseID.DatabaseName()}

	labelsIndex := 3
	for _, labelValue := range mdp.labelValues {
		labels[labelsIndex] = label{Name: labelValue.Metadata().Name(), Value: labelValue.Value()}
		labelsIndex++
	}

	return dataForHashing{
		MetricName: mdp.metricName,
		Labels:     labels,
	}
}

func (mdp *MetricsDataPoint) hash() (string, error) {
	hashedData, err := hashstructure.Hash(mdp.toDataForHashing(), nil)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", hashedData), nil
}
