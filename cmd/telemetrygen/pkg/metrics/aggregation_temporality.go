package metrics

import (
	"fmt"

	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

type AggregationTemporality metricdata.Temporality

func (t *AggregationTemporality) Set(v string) error {
	switch v {
	case "delta":
		*t = AggregationTemporality(metricdata.DeltaTemporality)
		return nil
	case "cumulative":
		*t = AggregationTemporality(metricdata.CumulativeTemporality)
		return nil
	default:
		return fmt.Errorf(`temporality must be one of "delta" or "cumulative"`)
	}
}

func (t *AggregationTemporality) String() string {
	return string(metricdata.Temporality(*t))
}

func (t *AggregationTemporality) Type() string {
	return "temporality"
}

// AsTemporality converts the AggregationTemporality to metricdata.Temporality
func (t AggregationTemporality) AsTemporality() metricdata.Temporality {
	return metricdata.Temporality(t)
}
