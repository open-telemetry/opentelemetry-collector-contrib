// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package skywalkingreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/skywalkingreceiver"

import (
	"context"
	"errors"
	"fmt"
	"io"

	"go.opentelemetry.io/collector/consumer"
	"google.golang.org/protobuf/proto"
	common "skywalking.apache.org/repo/goapi/collect/common/v3"
	agent "skywalking.apache.org/repo/goapi/collect/language/agent/v3"
)

type traceSegmentReportService struct {
	sr *swReceiver
	agent.UnimplementedTraceSegmentReportServiceServer
}

func (s *traceSegmentReportService) Collect(stream agent.TraceSegmentReportService_CollectServer) error {
	for {
		segmentObject, err := stream.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return stream.SendAndClose(&common.Commands{})
			}
			return err
		}

		err = consumeTraces(stream.Context(), segmentObject, s.sr.nextConsumer)
		if err != nil {
			return stream.SendAndClose(&common.Commands{})
		}
	}
}

func (s *traceSegmentReportService) CollectInSync(ctx context.Context, segments *agent.SegmentCollection) (*common.Commands, error) {
	for _, segment := range segments.Segments {
		marshaledSegment, err := proto.Marshal(segment)
		if err != nil {
			fmt.Printf("cannot marshal segemnt from sync, %v", err)
		}
		err = consumeTraces(ctx, segment, s.sr.nextConsumer)
		if err != nil {
			fmt.Printf("cannot consume traces, %v", err)
		}
		fmt.Printf("receivec data:%s", marshaledSegment)
	}
	return &common.Commands{}, nil
}

func consumeTraces(ctx context.Context, segment *agent.SegmentObject, consumer consumer.Traces) error {
	if segment == nil {
		return nil
	}
	ptd := SkywalkingToTraces(segment)
	return consumer.ConsumeTraces(ctx, ptd)
}
