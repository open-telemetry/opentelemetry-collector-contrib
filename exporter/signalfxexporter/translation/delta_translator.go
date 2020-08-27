// Copyright 2020, OpenTelemetry Authors
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

package translation

import (
	"github.com/gogo/protobuf/proto"
	sfxpb "github.com/signalfx/com_signalfx_metrics_protobuf/model"
)

type deltaTranslator struct {
	prevPts map[string]*sfxpb.DataPoint
}

func newDeltaTranslator() *deltaTranslator {
	return &deltaTranslator{prevPts: map[string]*sfxpb.DataPoint{}}
}

func (t *deltaTranslator) translate(processedDataPoints []*sfxpb.DataPoint, tr Rule) []*sfxpb.DataPoint {
	for _, currPt := range processedDataPoints {
		deltaMetricName, ok := tr.Mapping[currPt.Metric]
		if !ok {
			// only metrics defined in Rule.Mapping get translated
			continue
		}
		deltaPt := t.deltaPt(deltaMetricName, currPt)
		if deltaPt == nil {
			continue
		}
		processedDataPoints = append(processedDataPoints, deltaPt)
	}
	return processedDataPoints
}

func (t *deltaTranslator) deltaPt(deltaMetricName string, currPt *sfxpb.DataPoint) *sfxpb.DataPoint {
	// check if we have a previous point for this metric + dimensions
	dimKey := stringifyDimensions(currPt.Dimensions, nil)
	fullKey := currPt.Metric + ":" + dimKey
	prevPt, ok := t.prevPts[fullKey]
	t.prevPts[fullKey] = currPt
	if !ok {
		// no previous point, so we can't calculate a delta
		return nil
	}
	var deltaPt *sfxpb.DataPoint
	if currPt.Value.DoubleValue != nil && prevPt.Value.DoubleValue != nil {
		deltaPt = doublePt(currPt, prevPt, deltaMetricName)
	} else if currPt.Value.IntValue != nil && prevPt.Value.IntValue != nil {
		deltaPt = intPt(currPt, prevPt, deltaMetricName)
	} else {
		return nil
	}
	return deltaPt
}

func doublePt(currPt *sfxpb.DataPoint, prevPt *sfxpb.DataPoint, deltaMetricName string) *sfxpb.DataPoint {
	deltaPt := basePt(currPt, deltaMetricName)
	*deltaPt.Value.DoubleValue = *currPt.Value.DoubleValue - *prevPt.Value.DoubleValue
	return deltaPt
}

func intPt(currPt *sfxpb.DataPoint, prevPt *sfxpb.DataPoint, deltaMetricName string) *sfxpb.DataPoint {
	deltaPt := basePt(currPt, deltaMetricName)
	*deltaPt.Value.IntValue = *currPt.Value.IntValue - *prevPt.Value.IntValue
	return deltaPt
}

var cumulativeCounterType = sfxpb.MetricType_CUMULATIVE_COUNTER

func basePt(currPt *sfxpb.DataPoint, deltaMetricName string) *sfxpb.DataPoint {
	deltaPt := proto.Clone(currPt).(*sfxpb.DataPoint)
	deltaPt.Metric = deltaMetricName
	deltaPt.MetricType = &cumulativeCounterType
	return deltaPt
}
