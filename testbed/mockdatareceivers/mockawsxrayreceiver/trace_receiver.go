// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mockawsxrayreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/testbed/mockdatareceivers/mockawsxrayreceiver"

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/obsreport"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"
)

// MockAwsXrayReceiver type is used to handle spans received in the AWS data format.
type MockAwsXrayReceiver struct {
	mu     sync.Mutex
	logger *zap.Logger

	config *Config
	server *http.Server

	nextConsumer consumer.Traces
	obsrecv      *obsreport.Receiver
	httpsObsrecv *obsreport.Receiver
}

// New creates a new awsxrayreceiver.MockAwsXrayReceiver reference.
func New(
	nextConsumer consumer.Traces,
	params receiver.CreateSettings,
	config *Config) (*MockAwsXrayReceiver, error) {
	if nextConsumer == nil {
		return nil, component.ErrNilNextConsumer
	}

	obsrecv, err := obsreport.NewReceiver(obsreport.ReceiverSettings{ReceiverID: params.ID, Transport: "http", ReceiverCreateSettings: params})
	if err != nil {
		return nil, err
	}
	httpsObsrecv, err := obsreport.NewReceiver(obsreport.ReceiverSettings{ReceiverID: params.ID, Transport: "https", ReceiverCreateSettings: params})
	if err != nil {
		return nil, err
	}

	ar := &MockAwsXrayReceiver{
		logger:       params.Logger,
		config:       config,
		nextConsumer: nextConsumer,
		obsrecv:      obsrecv,
		httpsObsrecv: httpsObsrecv,
	}
	return ar, nil
}

// Start spins up the receiver's HTTP server and makes the receiver start its processing.
func (ar *MockAwsXrayReceiver) Start(_ context.Context, host component.Host) error {
	ar.mu.Lock()
	defer ar.mu.Unlock()

	// set up the listener
	ln, err := net.Listen("tcp", ar.config.Endpoint)
	if err != nil {
		return fmt.Errorf("failed to bind to address %s: %w", ar.config.Endpoint, err)
	}
	ar.logger.Info(fmt.Sprintf("listen to address %s", ar.config.Endpoint))

	// use gorilla mux to create a router/handler
	nr := mux.NewRouter()
	nr.HandleFunc("/TraceSegments", ar.HTTPHandlerFunc)

	// create a server with the handler
	ar.server = &http.Server{Handler: nr, ReadHeaderTimeout: 20 * time.Second}

	// run the server on a routine
	go func() {
		if ar.config.TLSCredentials != nil {
			host.ReportFatalError(ar.server.ServeTLS(ln, ar.config.TLSCredentials.CertFile, ar.config.TLSCredentials.KeyFile))
		} else {
			host.ReportFatalError(ar.server.Serve(ln))
		}
	}()
	return nil
}

// handleRequest parses an http request containing aws json request and passes the count of the traces to next consumer
func (ar *MockAwsXrayReceiver) handleRequest(req *http.Request) error {
	obsrecv := ar.obsrecv

	if ar.config.TLSCredentials != nil {
		obsrecv = ar.httpsObsrecv
	}

	ctx := obsrecv.StartTracesOp(req.Context())
	body, err := io.ReadAll(req.Body)
	if err != nil {
		log.Fatalln(err)
	}

	var result map[string]interface{}

	if err = json.Unmarshal(body, &result); err != nil {
		log.Fatalln(err)
	}

	traces, _ := ToTraces(body)
	sc := traces.SpanCount()

	err = ar.nextConsumer.ConsumeTraces(ctx, traces)
	obsrecv.EndTracesOp(ctx, typeStr, sc, err)
	return err
}

// HTTPHandlerFunc returns an http.HandlerFunc that handles awsXray requests
func (ar *MockAwsXrayReceiver) HTTPHandlerFunc(rw http.ResponseWriter, req *http.Request) {
	// handle the request payload
	err := ar.handleRequest(req)
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		return
	}
}

// Shutdown tells the receiver that should stop reception,
// giving it a chance to perform any necessary clean-up and shutting down
// its HTTP server.
func (ar *MockAwsXrayReceiver) Shutdown(context.Context) error {
	return ar.server.Close()
}

func ToTraces(rawSeg []byte) (ptrace.Traces, error) {
	var result map[string]interface{}
	err := json.Unmarshal(rawSeg, &result)
	if err != nil {
		return ptrace.Traces{}, err
	}

	records, ok := result["TraceSegmentDocuments"].([]interface{})
	if !ok {
		panic("Not a slice")
	}

	traceData := ptrace.NewTraces()
	rspan := traceData.ResourceSpans().AppendEmpty()
	ils := rspan.ScopeSpans().AppendEmpty()
	ils.Spans().EnsureCapacity(len(records))

	for i := 0; i < len(records); i++ {
		ils.Spans().AppendEmpty()
	}

	return traceData, nil
}
