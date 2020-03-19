package redisreceiver

import (
	"strconv"

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
)

func strToInt64Point(s string) (*metricspb.Point, error) {
	i, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return nil, err
	}
	return &metricspb.Point{Value: &metricspb.Point_Int64Value{Int64Value: i}}, nil
}

func strToDoublePoint(s string) (*metricspb.Point, error) {
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil, err
	}
	return &metricspb.Point{Value: &metricspb.Point_DoubleValue{DoubleValue: f}}, nil
}
