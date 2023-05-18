// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package trace // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/skywalkingreceiver/internal/trace"

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/obsreport"
	"go.opentelemetry.io/collector/receiver"
	"google.golang.org/protobuf/proto"
	common "skywalking.apache.org/repo/goapi/collect/common/v3"
	agent "skywalking.apache.org/repo/goapi/collect/language/agent/v3"
	v3 "skywalking.apache.org/repo/goapi/collect/language/agent/v3"
)

const (
	collectorHTTPTransport = "http"
	grpcTransport          = "grpc"
	failing                = "failing"
)

type Receiver struct {
	nextConsumer consumer.Traces
	grpcObsrecv  *obsreport.Receiver
	httpObsrecv  *obsreport.Receiver
	agent.UnimplementedTraceSegmentReportServiceServer
}

// NewReceiver creates a new Receiver reference.
func NewReceiver(nextConsumer consumer.Traces, set receiver.CreateSettings) (*Receiver, error) {
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
func (r *Receiver) Collect(stream agent.TraceSegmentReportService_CollectServer) error {
	for {
		segmentObject, err := stream.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return stream.SendAndClose(&common.Commands{})
			}
			return err
		}

		err = consumeTraces(stream.Context(), segmentObject, r.nextConsumer)
		if err != nil {
			return stream.SendAndClose(&common.Commands{})
		}
	}
}

// CollectInSync implements the service CollectInSync traces func.
func (r *Receiver) CollectInSync(ctx context.Context, segments *agent.SegmentCollection) (*common.Commands, error) {
	for _, segment := range segments.Segments {
		marshaledSegment, err := proto.Marshal(segment)
		if err != nil {
			fmt.Printf("cannot marshal segemnt from sync, %v", err)
		}
		err = consumeTraces(ctx, segment, r.nextConsumer)
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

func (r *Receiver) HTTPHandler(rsp http.ResponseWriter, req *http.Request) {
	rsp.Header().Set("Content-Type", "application/json")
	b, err := io.ReadAll(req.Body)
	if err != nil {
		response := &Response{Status: failing, Msg: err.Error()}
		ResponseWithJSON(rsp, response, http.StatusBadRequest)
		return
	}
	var data []*v3.SegmentObject
	if err = json.Unmarshal(b, &data); err != nil {
		fmt.Printf("cannot Unmarshal skywalking segment collection, %v", err)
	}

	for _, segment := range data {
		err = consumeTraces(req.Context(), segment, r.nextConsumer)
		if err != nil {
			fmt.Printf("cannot consume traces, %v", err)
		}
	}
}

type Response struct {
	Status string `json:"status"`
	Msg    string `json:"msg"`
}

func ResponseWithJSON(rsp http.ResponseWriter, response *Response, code int) {
	rsp.WriteHeader(code)
	_ = json.NewEncoder(rsp).Encode(response)
}
