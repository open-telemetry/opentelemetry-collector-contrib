// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package mockawsxrayreceiver

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"sync"

	"github.com/gorilla/mux"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenterror"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/obsreport"
	"go.uber.org/zap"
)

// MockAwsXrayReceiver type is used to handle spans received in the AWS data format.
type MockAwsXrayReceiver struct {
	mu        sync.Mutex
	startOnce sync.Once
	stopOnce  sync.Once
	logger    *zap.Logger

	config *Config
	server *http.Server

	nextConsumer consumer.TracesConsumer
}

// New creates a new awsxrayreceiver.MockAwsXrayReceiver reference.
func New(
	nextConsumer consumer.TracesConsumer,
	params component.ReceiverCreateParams,
	config *Config) (*MockAwsXrayReceiver, error) {
	if nextConsumer == nil {
		return nil, componenterror.ErrNilNextConsumer
	}

	ar := &MockAwsXrayReceiver{
		logger:       params.Logger,
		config:       config,
		nextConsumer: nextConsumer,
	}
	return ar, nil
}

// Start spins up the receiver's HTTP server and makes the receiver start its processing.
func (ar *MockAwsXrayReceiver) Start(_ context.Context, host component.Host) error {
	ar.mu.Lock()
	defer ar.mu.Unlock()

	var err = componenterror.ErrAlreadyStarted
	ar.startOnce.Do(func() {
		var ln net.Listener

		// set up the listener
		ln, err = net.Listen("tcp", ar.config.Endpoint)
		if err != nil {
			err = fmt.Errorf("failed to bind to address %s: %v", ar.config.Endpoint, err)
			return
		}
		ar.logger.Info(fmt.Sprintf("listen to address %s", ar.config.Endpoint))

		// use gorilla mux to create a router/handler
		nr := mux.NewRouter()
		nr.HandleFunc("/TraceSegments", ar.HTTPHandlerFunc)

		// create a server with the handler
		ar.server = &http.Server{Handler: nr}

		// run the server on a routine
		go func() {
			if ar.config.TLSCredentials != nil {
				host.ReportFatalError(ar.server.ServeTLS(ln, ar.config.TLSCredentials.CertFile, ar.config.TLSCredentials.KeyFile))
			} else {
				host.ReportFatalError(ar.server.Serve(ln))
			}
		}()
	})
	return err
}

// handleRequest parses an http request containing aws json request and passes the count of the traces to next consumer
func (ar *MockAwsXrayReceiver) handleRequest(ctx context.Context, req *http.Request) error {
	transport := "http"
	if ar.config.TLSCredentials != nil {
		transport = "https"
	}

	ctx = obsreport.ReceiverContext(ctx, ar.config.Name(), transport, "")
	ctx = obsreport.StartTraceDataReceiveOp(ctx, ar.config.Name(), transport)
	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		log.Fatalln(err)
	}

	var result map[string]interface{}

	json.Unmarshal(body, &result)

	traces, _ := ToTraces(body)

	ar.nextConsumer.ConsumeTraces(ctx, *traces)

	return nil
}

// HTTPHandlerFunction returns an http.HandlerFunc that handles awsXray requests
func (ar *MockAwsXrayReceiver) HTTPHandlerFunc(rw http.ResponseWriter, req *http.Request) {
	// create context with the receiver name from the request context
	ctx := obsreport.ReceiverContext(req.Context(), ar.config.Name(), "http", "")

	// handle the request payload
	err := ar.handleRequest(ctx, req)
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		return
	}

}

// Shutdown tells the receiver that should stop reception,
// giving it a chance to perform any necessary clean-up and shutting down
// its HTTP server.
func (ar *MockAwsXrayReceiver) Shutdown(context.Context) error {
	var err = componenterror.ErrAlreadyStopped
	ar.stopOnce.Do(func() {
		err = ar.server.Close()
	})
	return err
}

func ToTraces(rawSeg []byte) (*pdata.Traces, error) {
	var result map[string]interface{}
	err := json.Unmarshal(rawSeg, &result)
	if err != nil {
		return nil, err
	}

	records, ok := result["TraceSegmentDocuments"].([]interface{})
	if !ok {
		panic("Not a slice")
	}

	traceData := pdata.NewTraces()
	rspanSlice := traceData.ResourceSpans()
	rspanSlice.Resize(1)      // initialize a new empty pdata.ResourceSpans
	rspan := rspanSlice.At(0) // retrieve the empty pdata.ResourceSpans we just created

	rspan.InstrumentationLibrarySpans().Resize(1)
	ils := rspan.InstrumentationLibrarySpans().At(0)
	ils.Spans().Resize(len(records))

	return &traceData, nil
}
