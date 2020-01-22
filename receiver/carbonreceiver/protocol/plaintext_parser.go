// Copyright 2019, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package protocol

import (
	"fmt"
	"strconv"
	"strings"

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
)

// PlaintextParser converts a line of https://graphite.readthedocs.io/en/latest/feeding-carbon.html#the-plaintext-protocol,
// treating tags per spec at https://graphite.readthedocs.io/en/latest/tags.html#carbon.
type PlaintextParser struct{}

var _ (Parser) = (*PlaintextParser)(nil)
var _ (ParserConfig) = (*PlaintextParser)(nil)

// BuildParser creates a new Parser instance that receives plaintext
// Carbon data.
func (p *PlaintextParser) BuildParser() (Parser, error) {
	if p == nil {
		return &PlaintextParser{}, nil
	}
	return p, nil
}

// Parse receives the string with plaintext data, aka line, in the Carbon
// format and transforms it to the collector metric format. See
// https://graphite.readthedocs.io/en/latest/feeding-carbon.html#the-plaintext-protocol.
func (p PlaintextParser) Parse(line string) (*metricspb.Metric, error) {
	parts := strings.SplitN(line, " ", 3)
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid carbon metric [%s]", line)
	}

	path := parts[0]
	valueStr := parts[1]
	timestampStr := parts[2]

	metricName, labelKeys, labelValues, err := p.parsePath(path)
	if err != nil {
		return nil, fmt.Errorf("invalid carbon metric [%s]: %v", line, err)
	}

	unixTime, err := strconv.ParseInt(timestampStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid carbon metric time [%s]: %v", line, err)
	}

	var metricType metricspb.MetricDescriptor_Type
	point := metricspb.Point{
		Timestamp: convertUnixSec(unixTime),
	}
	intVal, err := strconv.ParseInt(valueStr, 10, 64)
	if err == nil {
		metricType = metricspb.MetricDescriptor_GAUGE_INT64
		point.Value = &metricspb.Point_Int64Value{Int64Value: intVal}
	} else {
		dblVal, err := strconv.ParseFloat(valueStr, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid carbon metric value [%s]: %v", line, err)
		}
		metricType = metricspb.MetricDescriptor_GAUGE_DOUBLE
		point.Value = &metricspb.Point_DoubleValue{DoubleValue: dblVal}
	}

	metric := buildMetricForSinglePoint(
		metricName,
		metricType,
		labelKeys,
		labelValues,
		&point)
	return metric, nil
}

func (p *PlaintextParser) parsePath(path string) (name string, keys []*metricspb.LabelKey, values []*metricspb.LabelValue, err error) {
	parts := strings.SplitN(path, ";", 2)
	if len(parts) < 1 || parts[0] == "" {
		err = fmt.Errorf("empty metric name extracted from path [%s]", path)
		return
	}
	name = parts[0]
	if len(parts) == 1 {
		// No tags, no more work here.
		return
	}

	if parts[1] == "" {
		// Empty tags, nothing to do.
		return
	}

	tags := strings.Split(parts[1], ";")
	keys = make([]*metricspb.LabelKey, 0, len(tags))
	values = make([]*metricspb.LabelValue, 0, len(tags))
	for _, tag := range tags {
		idx := strings.IndexByte(tag, '=')
		if idx < 1 {
			err = fmt.Errorf("cannot parse metric path [%s]: incorrect key value separator for [%s]", path, tag)
			return
		}

		key := tag[:idx]
		keys = append(keys, &metricspb.LabelKey{Key: key})

		value := tag[idx+1:] // If value is empty, ie.: tag == "k=", this will return "".
		values = append(values, &metricspb.LabelValue{
			Value:    value,
			HasValue: true,
		})
	}

	return
}
