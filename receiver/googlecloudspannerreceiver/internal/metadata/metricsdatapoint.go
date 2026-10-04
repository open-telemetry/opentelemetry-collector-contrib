// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metadata // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver/internal/metadata"

import (
	"crypto/sha256"
	"encoding/binary"
	"hash/fnv"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/mitchellh/hashstructure/v2"
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
	Value any
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
			builderHashedKey.WriteString(strconv.FormatUint(uint64(hashFunction.Sum32()), 10) + ",")
		} else {
			builderHashedKey.WriteString(strconv.FormatUint(uint64(hashFunction.Sum32()), 10))
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
		if labelValue.Metadata().Name() != "row_range_start_key" {
			continue
		}
		key := labelValue.Value().(string)
		hashedKey := parseAndHashRowrangestartkey(key)
		v := mdp.labelValues[index].(byteSliceLabelValue)
		p := &v
		p.ModifyValue(hashedKey)
		mdp.labelValues[index] = v
	}
}

func (mdp *MetricsDataPoint) HideSplitStatsKeysPII() {
	for index, labelValue := range mdp.labelValues {
		if labelValue.Metadata().Name() != "split_start" && labelValue.Metadata().Name() != "split_limit" {
			continue
		}

		switch v := labelValue.(type) {
		case byteSliceLabelValue:
			p := &v
			p.ModifyValue(parseAndHashSplitStatsKey(v.Value().(string)))
			mdp.labelValues[index] = v
		case stringSliceLabelValue:
			p := &v
			p.ModifyValue(parseAndHashSplitStatsKey(v.Value().(string)))
			mdp.labelValues[index] = v
		case stringLabelValue:
			p := &v
			p.ModifyValue(parseAndHashSplitStatsKey(v.Value().(string)))
			mdp.labelValues[index] = v
		}
	}
}

func parseAndHashSplitStatsKey(val string) string {
	if val == "<begin>" || val == "<end>" || val == "<infinity>" {
		return val
	}

	var result strings.Builder
	var current strings.Builder
	inQuotes := false
	escapeNext := false
	bracketLevel := 0

	flushToken := func(hashToken bool) {
		if current.Len() == 0 {
			return
		}
		token := current.String()
		current.Reset()

		if !hashToken || token == "NULL" || token == "<begin>" || token == "<end>" || token == "<infinity>" {
			result.WriteString(token)
			return
		}

		hash := sha256.Sum256([]byte(token))
		hashUint := binary.BigEndian.Uint32(hash[:4])
		hashedStr := strconv.FormatUint(uint64(hashUint), 10)

		result.WriteString(hashedStr)
	}

	for i := 0; i < len(val); i++ {
		c := val[i]

		if escapeNext {
			current.WriteByte(c)
			escapeNext = false
			continue
		}

		if c == '\\' {
			current.WriteByte(c)
			escapeNext = true
			continue
		}

		if c == '"' {
			inQuotes = !inQuotes
			current.WriteByte(c)
			continue
		}

		if !inQuotes {
			if c == '(' {
				flushToken(false)
				result.WriteByte(c)
				bracketLevel++
				continue
			}
			if c == ')' {
				flushToken(bracketLevel > 0)
				result.WriteByte(c)
				if bracketLevel > 0 {
					bracketLevel--
				}
				continue
			}
			if c == ',' {
				flushToken(bracketLevel > 0)
				result.WriteByte(c)
				continue
			}
		}

		current.WriteByte(c)
	}

	flushToken(bracketLevel > 0)

	return result.String()
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
		if labelValue.Metadata().Name() != "query_text" {
			continue
		}
		queryText := labelValue.Value().(string)
		truncateQueryText := TruncateString(queryText, length)
		v := mdp.labelValues[index].(stringLabelValue)
		p := &v
		p.ModifyValue(truncateQueryText)
		mdp.labelValues[index] = v
	}
}

func (mdp *MetricsDataPoint) hash() (string, error) {
	hashedData, err := hashstructure.Hash(mdp.toDataForHashing(), hashstructure.FormatV1, nil)
	if err != nil {
		return "", err
	}

	return strconv.FormatUint(hashedData, 16), nil
}
