package metrics

import (
	"fmt"

	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

type TemporalityType metricdata.Temporality

func (t *TemporalityType) Set(v string) error {
	switch v {
	case "delta":
		*t = TemporalityType(metricdata.DeltaTemporality)
		return nil
	case "cumulative":
		*t = TemporalityType(metricdata.CumulativeTemporality)
		return nil
	default:
		return fmt.Errorf(`temporality must be one of "delta" or "cumulative"`)
	}
}

func (t *TemporalityType) String() string {
	return string(metricdata.Temporality(*t))
}

func (t *TemporalityType) Type() string {
	return "temporality"
}

// AsTemporality converts the TemporalityType to metricdata.Temporality
func (t TemporalityType) AsTemporality() metricdata.Temporality {
	return metricdata.Temporality(t)
}
