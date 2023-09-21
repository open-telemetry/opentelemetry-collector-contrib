// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metadata // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver/internal/metadata"

import (
	"fmt"
	"hash/fnv"
	"strings"
	"time"
	"unicode/utf8"

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
	MetricName string
	MetricUnit string
	MetricType MetricType
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
	attributes.EnsureCapacity(3 + len(mdp.labelValues))
	attributes.PutStr(projectIDLabelName, mdp.databaseID.ProjectID())
	attributes.PutStr(instanceIDLabelName, mdp.databaseID.InstanceID())
	attributes.PutStr(databaseLabelName, mdp.databaseID.DatabaseName())
	for i := range mdp.labelValues {
		mdp.labelValues[i].SetValueTo(attributes)
	}
}

func (mdp *MetricsDataPoint) GroupingKey() MetricsDataPointKey {
	return MetricsDataPointKey{
		MetricName: mdp.metricName,
		MetricUnit: mdp.metricValue.Metadata().Unit(),
		MetricType: mdp.metricValue.Metadata().DataType(),
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

// Convert row_range_start_key label of top-lock-stats metric from format "sample(key1, key2)" to "sample(hash1, hash2)"
func parseAndHashRowrangestartkey(key string) string {
	builderHashedKey := strings.Builder{}
	startIndexKeys := strings.Index(key, "(")
	if startIndexKeys == -1 || startIndexKeys == len(key)-1 { // if "(" does not exist or is the last character of the string, then label is of incorrect format
		return ""
	}
	substring := key[startIndexKeys+1 : len(key)-1]
	builderHashedKey.WriteString(key[:startIndexKeys+1])
	plusPresent := false
	if substring[len(substring)-1] == '+' {
		substring = substring[:len(substring)-1]
		plusPresent = true
	}
	keySlice := strings.Split(substring, ",")
	hashFunction := fnv.New32a()
	for cnt, subKey := range keySlice {
		hashFunction.Reset()
		hashFunction.Write([]byte(subKey))
		if cnt < len(keySlice)-1 {
			builderHashedKey.WriteString(fmt.Sprint(hashFunction.Sum32()) + ",")
		} else {
			builderHashedKey.WriteString(fmt.Sprint(hashFunction.Sum32()))
		}
	}
	if plusPresent {
		builderHashedKey.WriteString("+")
	}
	builderHashedKey.WriteString(")")
	return builderHashedKey.String()
}

func (mdp *MetricsDataPoint) HideLockStatsRowrangestartkeyPII() {
	for index, labelValue := range mdp.labelValues {
		if labelValue.Metadata().Name() == "row_range_start_key" {
			key := labelValue.Value().(string)
			hashedKey := parseAndHashRowrangestartkey(key)
			v := mdp.labelValues[index].(byteSliceLabelValue)
			p := &v
			p.ModifyValue(hashedKey)
			mdp.labelValues[index] = v
		}
	}
}

func TruncateString(str string, length int) string {
	if length <= 0 {
		return ""
	}

	if utf8.RuneCountInString(str) < length {
		return str
	}

	return string([]rune(str)[:length])
}

func (mdp *MetricsDataPoint) TruncateQueryText(length int) {
	for index, labelValue := range mdp.labelValues {
		if labelValue.Metadata().Name() == "query_text" {
			queryText := labelValue.Value().(string)
			truncateQueryText := TruncateString(queryText, length)
			v := mdp.labelValues[index].(stringLabelValue)
			p := &v
			p.ModifyValue(truncateQueryText)
			mdp.labelValues[index] = v
		}
	}
}

func (mdp *MetricsDataPoint) hash() (string, error) {
	hashedData, err := hashstructure.Hash(mdp.toDataForHashing(), nil)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", hashedData), nil
}
