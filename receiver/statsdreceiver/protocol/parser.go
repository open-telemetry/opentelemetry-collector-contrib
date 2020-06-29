package protocol

import (
	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
)

type Parser interface {
	Parse(in string) (*metricspb.Metric, error)
}
