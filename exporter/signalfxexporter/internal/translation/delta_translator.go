// Copyright The OpenTelemetry Authors
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

package translation // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/internal/translation"

import (
	"github.com/gogo/protobuf/proto"
	sfxpb "github.com/signalfx/com_signalfx_metrics_protobuf/model"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/ttlmap"
)

type deltaTranslator struct {
	prevPts *ttlmap.TTLMap
}

func newDeltaTranslator(ttl int64) *deltaTranslator {
	sweepIntervalSeconds := ttl / 2
	if sweepIntervalSeconds == 0 {
		sweepIntervalSeconds = 1
	}
	m := ttlmap.New(sweepIntervalSeconds, ttl)
	m.Start()
	return &deltaTranslator{prevPts: m}
}

func (t *deltaTranslator) translate(pts []*sfxpb.DataPoint, tr Rule) []*sfxpb.DataPoint {
	for _, currPt := range pts {
		deltaMetricName, ok := tr.Mapping[currPt.Metric]
		if !ok {
			// only metrics defined in Rule.Mapping get translated
			continue
		}
		deltaPt := t.deltaPt(deltaMetricName, currPt)
		if deltaPt == nil {
			continue
		}
		pts = append(pts, deltaPt)
	}
	return pts
}

func (t *deltaTranslator) deltaPt(deltaMetricName string, currPt *sfxpb.DataPoint) *sfxpb.DataPoint {
	// check if we have a previous point for this metric + dimensions
	dimKey := stringifyDimensions(currPt.Dimensions, nil)
	fullKey := currPt.Metric + ":" + dimKey
	v := t.prevPts.Get(fullKey)
	// without proto.Clone here, points' DoubleValue are converted into IntValues, presumably by other translators
	t.prevPts.Put(fullKey, proto.Clone(currPt))
	if v == nil {
		// no previous point, so we can't calculate a delta
		return nil
	}
	prevPt := v.(*sfxpb.DataPoint)
	var deltaPt *sfxpb.DataPoint
	switch {
	case currPt.Value.DoubleValue != nil && prevPt.Value.DoubleValue != nil:
		deltaPt = doubleDeltaPt(currPt, prevPt, deltaMetricName)
	case currPt.Value.IntValue != nil && prevPt.Value.IntValue != nil:
		deltaPt = intDeltaPt(currPt, prevPt, deltaMetricName)
	default:
		return nil
	}
	return deltaPt
}

func doubleDeltaPt(currPt *sfxpb.DataPoint, prevPt *sfxpb.DataPoint, deltaMetricName string) *sfxpb.DataPoint {
	delta := *currPt.Value.DoubleValue - *prevPt.Value.DoubleValue
	if delta < 0 {
		// assume a reset, emit the current value
		delta = *currPt.Value.DoubleValue
	}
	deltaPt := basePt(currPt, deltaMetricName)
	*deltaPt.Value.DoubleValue = delta
	return deltaPt
}

func intDeltaPt(currPt *sfxpb.DataPoint, prevPt *sfxpb.DataPoint, deltaMetricName string) *sfxpb.DataPoint {
	delta := *currPt.Value.IntValue - *prevPt.Value.IntValue
	if delta < 0 {
		// assume a reset, emit the current value
		delta = *currPt.Value.IntValue
	}
	deltaPt := basePt(currPt, deltaMetricName)
	*deltaPt.Value.IntValue = delta
	return deltaPt
}

var metricTypeGauge = sfxpb.MetricType_GAUGE

func basePt(currPt *sfxpb.DataPoint, deltaMetricName string) *sfxpb.DataPoint {
	deltaPt := proto.Clone(currPt).(*sfxpb.DataPoint)
	deltaPt.Metric = deltaMetricName
	deltaPt.MetricType = &metricTypeGauge
	return deltaPt
}
