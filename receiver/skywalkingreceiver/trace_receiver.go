// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package skywalkingreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/skywalkingreceiver"

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"

	"github.com/gorilla/mux"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/obsreport"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/multierr"
	"google.golang.org/grpc"
	cds "skywalking.apache.org/repo/goapi/collect/agent/configuration/v3"
	event "skywalking.apache.org/repo/goapi/collect/event/v3"
	v3 "skywalking.apache.org/repo/goapi/collect/language/agent/v3"
	profile "skywalking.apache.org/repo/goapi/collect/language/profile/v3"
	management "skywalking.apache.org/repo/goapi/collect/management/v3"
)

// configuration defines the behavior and the ports that
// the Skywalking receiver will use.
type configuration struct {
	CollectorHTTPPort           int
	CollectorHTTPSettings       confighttp.HTTPServerSettings
	CollectorGRPCPort           int
	CollectorGRPCServerSettings configgrpc.GRPCServerSettings
}

// Receiver type is used to receive spans that were originally intended to be sent to Skywaking.
// This receiver is basically a Skywalking collector.
type swReceiver struct {
	nextConsumer consumer.Traces

	config *configuration

	grpc            *grpc.Server
	collectorServer *http.Server

	goroutines sync.WaitGroup

	settings receiver.CreateSettings

	grpcObsrecv          *obsreport.Receiver
	httpObsrecv          *obsreport.Receiver
	segmentReportService *traceSegmentReportService
	dummyReportService   *dummyReportService
}

const (
	collectorHTTPTransport = "http"
	grpcTransport          = "grpc"
	failing                = "failing"
)

// newSkywalkingReceiver creates a TracesReceiver that receives traffic as a Skywalking collector
func newSkywalkingReceiver(
	config *configuration,
	nextConsumer consumer.Traces,
	set receiver.CreateSettings,
) (*swReceiver, error) {

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

	return &swReceiver{
		config:       config,
		nextConsumer: nextConsumer,
		settings:     set,
		grpcObsrecv:  grpcObsrecv,
		httpObsrecv:  httpObsrecv,
	}, nil
}

func (sr *swReceiver) collectorGRPCAddr() string {
	var port int
	if sr.config != nil {
		port = sr.config.CollectorGRPCPort
	}
	return fmt.Sprintf(":%d", port)
}

func (sr *swReceiver) collectorGRPCEnabled() bool {
	return sr.config != nil && sr.config.CollectorGRPCPort > 0
}

func (sr *swReceiver) collectorHTTPEnabled() bool {
	return sr.config != nil && sr.config.CollectorHTTPPort > 0
}

func (sr *swReceiver) Start(_ context.Context, host component.Host) error {
	return sr.startCollector(host)
}

func (sr *swReceiver) Shutdown(ctx context.Context) error {
	var errs error

	if sr.collectorServer != nil {
		if cerr := sr.collectorServer.Shutdown(ctx); cerr != nil {
			errs = multierr.Append(errs, cerr)
		}
	}
	if sr.grpc != nil {
		sr.grpc.GracefulStop()
	}

	sr.goroutines.Wait()
	return errs
}

func (sr *swReceiver) startCollector(host component.Host) error {
	if !sr.collectorGRPCEnabled() && !sr.collectorHTTPEnabled() {
		return nil
	}

	if sr.collectorHTTPEnabled() {
		cln, cerr := sr.config.CollectorHTTPSettings.ToListener()
		if cerr != nil {
			return fmt.Errorf("failed to bind to Collector address %q: %w",
				sr.config.CollectorHTTPSettings.Endpoint, cerr)
		}

		nr := mux.NewRouter()
		nr.HandleFunc("/v3/segments", sr.httpHandler).Methods(http.MethodPost)
		sr.collectorServer, cerr = sr.config.CollectorHTTPSettings.ToServer(host, sr.settings.TelemetrySettings, nr)
		if cerr != nil {
			return cerr
		}

		sr.goroutines.Add(1)
		go func() {
			defer sr.goroutines.Done()
			if errHTTP := sr.collectorServer.Serve(cln); !errors.Is(errHTTP, http.ErrServerClosed) && errHTTP != nil {
				host.ReportFatalError(errHTTP)
			}
		}()
	}

	if sr.collectorGRPCEnabled() {
		var err error
		sr.grpc, err = sr.config.CollectorGRPCServerSettings.ToServer(host, sr.settings.TelemetrySettings)
		if err != nil {
			return fmt.Errorf("failed to build the options for the Skywalking gRPC Collector: %w", err)
		}

		gaddr := sr.collectorGRPCAddr()
		gln, gerr := net.Listen("tcp", gaddr)
		if gerr != nil {
			return fmt.Errorf("failed to bind to gRPC address %q: %w", gaddr, gerr)
		}

		sr.segmentReportService = &traceSegmentReportService{sr: sr}
		v3.RegisterTraceSegmentReportServiceServer(sr.grpc, sr.segmentReportService)
		sr.dummyReportService = &dummyReportService{}

		management.RegisterManagementServiceServer(sr.grpc, sr.dummyReportService)
		cds.RegisterConfigurationDiscoveryServiceServer(sr.grpc, sr.dummyReportService)
		event.RegisterEventServiceServer(sr.grpc, &eventService{})
		profile.RegisterProfileTaskServer(sr.grpc, sr.dummyReportService)
		v3.RegisterJVMMetricReportServiceServer(sr.grpc, sr.dummyReportService)
		v3.RegisterMeterReportServiceServer(sr.grpc, &meterService{})
		v3.RegisterCLRMetricReportServiceServer(sr.grpc, &clrService{})
		v3.RegisterBrowserPerfServiceServer(sr.grpc, sr.dummyReportService)

		sr.goroutines.Add(1)
		go func() {
			defer sr.goroutines.Done()
			if errGrpc := sr.grpc.Serve(gln); !errors.Is(errGrpc, grpc.ErrServerStopped) && errGrpc != nil {
				host.ReportFatalError(errGrpc)
			}
		}()
	}

	return nil
}

type Response struct {
	Status string `json:"status"`
	Msg    string `json:"msg"`
}

func (sr *swReceiver) httpHandler(rsp http.ResponseWriter, r *http.Request) {
	rsp.Header().Set("Content-Type", "application/json")
	b, err := io.ReadAll(r.Body)
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
		err = consumeTraces(r.Context(), segment, sr.nextConsumer)
		if err != nil {
			fmt.Printf("cannot consume traces, %v", err)
		}
	}
}

func ResponseWithJSON(rsp http.ResponseWriter, response *Response, code int) {
	rsp.WriteHeader(code)
	_ = json.NewEncoder(rsp).Encode(response)
}
