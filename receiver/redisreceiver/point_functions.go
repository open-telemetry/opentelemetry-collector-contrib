package redisreceiver

import (
	"strconv"

	metricsProto "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
)

type pointFcn func(s string) (*metricsProto.Point, error)

var _ pointFcn = strToInt64Point

func strToInt64Point(s string) (*metricsProto.Point, error) {
	i, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return nil, err
	}
	return &metricsProto.Point{Value: &metricsProto.Point_Int64Value{Int64Value: i}}, nil
}

func strToDoublePoint(s string) (*metricsProto.Point, error) {
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil, err
	}
	return &metricsProto.Point{Value: &metricsProto.Point_DoubleValue{DoubleValue: f}}, nil
}
