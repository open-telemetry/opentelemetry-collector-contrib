// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"context"

	p "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusremotewritereceiver/proto"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
)

const dataFormatProtobuf = "protobuf"

// Receiver is the type used to handle metrics from OpenTelemetry exporters.
type Receiver struct {
	p.UnimplementedRemoteWriteServiceServer
	nextConsumer consumer.Metrics
	metricMap    map[string]string
	obsreport    *receiverhelper.ObsReport
}

// New creates a new Receiver reference.
func New(nextConsumer consumer.Metrics, obsreport *receiverhelper.ObsReport, metricTypes map[string][]string) *Receiver {
	metricMap := make(map[string]string)
	for k, a := range metricTypes {
		for _, v := range a {
			metricMap[v] = k
		}
	}
	return &Receiver{
		nextConsumer: nextConsumer,
		metricMap:    metricMap,
		obsreport:    obsreport,
	}
}

func (r *Receiver) Write(ctx context.Context, req *p.WriteRequest) (*p.WriteResponse, error) {
	md := req.Timeseries
	dataPointCount := len(md)
	if dataPointCount == 0 {
		return &p.WriteResponse{}, nil
	}

	//TODO: Finish conversion implementation. Histogram and Summary unimplemented. Can likely consolidate Gauge and Counter into one function.
	result := ConvertMetrics(req, r.metricMap)

	ctx = r.obsreport.StartMetricsOp(ctx)
	err := r.nextConsumer.ConsumeMetrics(ctx, *result)
	r.obsreport.EndMetricsOp(ctx, dataFormatProtobuf, dataPointCount, err)

	return &p.WriteResponse{}, err
}
