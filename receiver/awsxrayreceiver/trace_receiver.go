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

package awsxrayreceiver

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenterror"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/obsreport"
	"go.uber.org/zap"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"sync"
)

// AwsXrayReceiver type is used to handle spans received in the AWS data format.
type AwsXrayReceiver struct {
	mu        sync.Mutex
	startOnce sync.Once
	stopOnce  sync.Once
	logger    *zap.Logger

	config *Config
	server *http.Server

	nextConsumer consumer.TestBedTraceConsumer
}

// New creates a new awsxrayreceiver.AwsXrayReceiver reference.
func New(
	nextConsumer consumer.TestBedTraceConsumer,
	params component.ReceiverCreateParams,
	config *Config, ) (*AwsXrayReceiver, error) {
	if nextConsumer == nil {
		return nil, componenterror.ErrNilNextConsumer
	}

	ar := &AwsXrayReceiver{
		logger:       params.Logger,
		config:       config,
		nextConsumer: nextConsumer,
	}
	return ar, nil
}

// Start spins up the receiver's HTTP server and makes the receiver start its processing.
func (ar *AwsXrayReceiver) Start(_ context.Context, host component.Host) error {
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
func (ar *AwsXrayReceiver) handleRequest(ctx context.Context, req *http.Request) error {
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

	json.Unmarshal([]byte(body), &result)

	records, ok := result["TraceSegmentDocuments"].([]interface{})
	if !ok {
		panic("Not a slice")
	}

	count := len(records)

	ar.nextConsumer.ConsumeTestbedTraces(count)

	return nil
}

// HTTPHandlerFunction returns an http.HandlerFunc that handles awsXray requests
func (ar *AwsXrayReceiver) HTTPHandlerFunc(rw http.ResponseWriter, req *http.Request) {
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
func (ar *AwsXrayReceiver) Shutdown(context.Context) error {
	var err = componenterror.ErrAlreadyStopped
	ar.stopOnce.Do(func() {
		err = ar.server.Close()
	})
	return err
}

//todo : This methods can be used when we want to convert the data from aws traces to span, right now it is not required
/*
type TraceSegmentDocuments struct {
TraceDocuments []Documents  `json:"TraceSegmentDocuments,omitempty"`
}
type Documents struct {
	TraceID   string  `json:"trace_id,omitempty"`
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	StartTime float64 `json:"start_time"`
	EndTime   float64 `json:"end_time,omitempty"`
	Origin      string     `json:"origin,omitempty"`
	Type         string   `json:"type,omitempty"`
	Namespace    string   `json:"namespace,omitempty"`
	AWS         AWSData  `json:"aws,omitempty"`
	Annotations map[string]interface{}            `json:"annotations,omitempty"`
}

type AWSData struct {
	AccountID         string             `json:"account_id,omitempty"`
	Operation         string             `json:"operation,omitempty"`
	RemoteRegion      string             `json:"region,omitempty"`
	RequestID         string             `json:"request_id,omitempty"`
	QueueURL          string             `json:"queue_url,omitempty"`
	TableName         string             `json:"table_name,omitempty"`
}


//add this code at line 120
	var ts TraceSegmentDocuments
	if err := json.Unmarshal([]byte(body), &ts); err != nil {
		panic(err)
	}
	fmt.Println(ts)

func (bd *TraceSegmentDocuments) UnmarshalJSON(b []byte) error {
	d := map[string][]string{}
	if err := json.Unmarshal(b, &d); err != nil {
		return err
	}
	dd := []Documents{}
	td := []pdata.Traces{}

	for _, bd := range d["TraceSegmentDocuments"] {
		d := Documents{}
		if err := json.Unmarshal([]byte(bd), &d); err != nil {
			panic(err)
		}
		//td = append(td,awsSpanToTraceSpan(d))
		dd = append(dd, d)
	}

	fmt.Println(dd)

	fmt.Println(dd[0].Name)

	return nil
}

func awsSpanToTraceSpan(document Documents) (*tracepb.Span) {


	pbs := &tracepb.Span{
		TraceId:      []byte(document.TraceID),
		Name:         &tracepb.TruncatableString{Value: document.Name},
		StartTime:    TimeToTimestamp(TimestampFromFloat64(document.StartTime)),
		EndTime:      TimeToTimestamp(TimestampFromFloat64(document.EndTime)),
	}


	return pbs
}

func TimestampFromFloat64(ts float64) time.Time {
	secs := int64(ts)
	nsecs := int64((ts - float64(secs)) * 1e9)
	return time.Unix(secs, nsecs)
}

func TimeToTimestamp(t time.Time) *timestamp.Timestamp {
	if t.IsZero() {
		return nil
	}
	nanoTime := t.UnixNano()
	return &timestamp.Timestamp{
		Seconds: nanoTime / 1e9,
		Nanos:   int32(nanoTime % 1e9),
	}
}*/
