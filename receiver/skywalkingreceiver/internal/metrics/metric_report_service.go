// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package metrics

import (
	"context"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/obsreport"
	"go.opentelemetry.io/collector/receiver"
	common "skywalking.apache.org/repo/goapi/collect/common/v3"
	agent "skywalking.apache.org/repo/goapi/collect/language/agent/v3"
)

const (
	collectorHTTPTransport = "http"
	grpcTransport          = "grpc"
	failing                = "failing"
)

type Receiver struct {
	nextConsumer consumer.Metrics
	grpcObsrecv  *obsreport.Receiver
	httpObsrecv  *obsreport.Receiver
	agent.UnimplementedJVMMetricReportServiceServer
}

// NewReceiver creates a new Receiver reference.
func NewReceiver(nextConsumer consumer.Metrics, set receiver.CreateSettings) (*Receiver, error) {
	grpcObsrecv, err := obsreport.NewReceiver(obsreport.ReceiverSettings{
		ReceiverID:             set.ID,
		Transport:              grpcTransport,
		ReceiverCreateSettings: set,
	})
	if err != nil {
		return nil, err
	}
	httpObsrecv, err := obsreport.NewReceiver(obsreport.ReceiverSettings{
		ReceiverID:             set.ID,
		Transport:              collectorHTTPTransport,
		ReceiverCreateSettings: set,
	})
	if err != nil {
		return nil, err
	}
	return &Receiver{
		nextConsumer: nextConsumer,
		grpcObsrecv:  grpcObsrecv,
		httpObsrecv:  httpObsrecv,
	}, nil
}

// Collect implements the service Collect traces func.
func (r *Receiver) Collect(ctx context.Context, jvmMetricCollection *agent.JVMMetricCollection) (*common.Commands, error) {
	err := consumeMetrics(ctx, jvmMetricCollection, r.nextConsumer)
	if err != nil {
		return &common.Commands{}, err
	}
	return &common.Commands{}, nil
}

func consumeMetrics(ctx context.Context, collection *agent.JVMMetricCollection, nextConsumer consumer.Metrics) error {
	if collection == nil {
		return nil
	}
	pmd := SkywalingToMetrics(collection)
	return nextConsumer.ConsumeMetrics(ctx, pmd)

}
