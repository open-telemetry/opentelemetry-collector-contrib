// Copyright  OpenTelemetry Authors
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

package timestampprocessor

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/model/pdata"

	"go.uber.org/zap"
)

type filterMetricProcessor struct {
	cfg    *Config
	logger *zap.Logger
}

func newTimestampMetricProcessor(logger *zap.Logger, cfg config.Processor) (*filterMetricProcessor, error) {
	logger.Info("Building metric timestamp processor")

	oCfg := cfg.(*Config)
	if oCfg.RoundToNearest == nil {
		return nil, fmt.Errorf("error creating \"timestamp\" processor due to missing required field \"round_to_nearest\" of processor %v", oCfg.ID())
	}

	return &filterMetricProcessor{cfg: oCfg, logger: logger}, nil
}

// processMetrics takes incoming metrics and adjusts their timestamps to
// the nearest time unit (specified by duration in the config)
func (fmp *filterMetricProcessor) processMetrics(_ context.Context, src pdata.Metrics) (pdata.Metrics, error) {
	// set the timestamps to the nearest time unit
	for i := 0; i < src.ResourceMetrics().Len(); i++ {
		rm := src.ResourceMetrics().At(i)
		for j := 0; j < rm.InstrumentationLibraryMetrics().Len(); j++ {
			ms := rm.InstrumentationLibraryMetrics().At(j)
			for k := 0; k < ms.Metrics().Len(); k++ {
				m := ms.Metrics().At(k)

				switch m.DataType() {
				case pdata.MetricDataTypeGauge:
					dataPoints := m.Gauge().DataPoints()
					for l := 0; l < dataPoints.Len(); l++ {
						gotDataPoint := dataPoints.At(l)
						snappedTimestamp := gotDataPoint.Timestamp().AsTime().Truncate(*fmp.cfg.RoundToNearest)
						gotDataPoint.SetTimestamp(pdata.TimestampFromTime(snappedTimestamp))
					}
				case pdata.MetricDataTypeSum:
					dataPoints := m.Sum().DataPoints()
					for l := 0; l < dataPoints.Len(); l++ {
						gotDataPoint := dataPoints.At(l)
						snappedTimestamp := gotDataPoint.Timestamp().AsTime().Truncate(*fmp.cfg.RoundToNearest)
						gotDataPoint.SetTimestamp(pdata.TimestampFromTime(snappedTimestamp))
					}
				case pdata.MetricDataTypeHistogram:
					dataPoints := m.Histogram().DataPoints()
					for l := 0; l < dataPoints.Len(); l++ {
						gotDataPoint := dataPoints.At(l)
						snappedTimestamp := gotDataPoint.Timestamp().AsTime().Truncate(*fmp.cfg.RoundToNearest)
						gotDataPoint.SetTimestamp(pdata.TimestampFromTime(snappedTimestamp))
					}
				case pdata.MetricDataTypeSummary:
					dataPoints := m.Summary().DataPoints()
					for l := 0; l < dataPoints.Len(); l++ {
						gotDataPoint := dataPoints.At(l)
						snappedTimestamp := gotDataPoint.Timestamp().AsTime().Truncate(*fmp.cfg.RoundToNearest)
						gotDataPoint.SetTimestamp(pdata.TimestampFromTime(snappedTimestamp))
					}
				default:
					fmt.Printf("Unknown type")
					return src, fmt.Errorf("unknown type: %s", m.DataType().String())
				}
			}
		}
	}
	return src, nil
}
