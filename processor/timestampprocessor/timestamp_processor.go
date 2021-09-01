package timestampprocessor

import (
	"context"
	"fmt"

	"go.uber.org/zap"

	"go.opentelemetry.io/collector/model/pdata"
)

type filterMetricProcessor struct {
	cfg    *Config
	logger *zap.Logger
}

func newTimestampMetricProcessor(logger *zap.Logger, cfg *Config) (*filterMetricProcessor, error) {
	logger.Info("Metric timestamp processor configured")
	return &filterMetricProcessor{cfg: cfg, logger: logger}, nil
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
