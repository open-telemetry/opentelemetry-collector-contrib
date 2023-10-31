// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package skywalkingreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/skywalkingreceiver"

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"

	"github.com/gorilla/mux"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/multierr"
	"google.golang.org/grpc"
	cds "skywalking.apache.org/repo/goapi/collect/agent/configuration/v3"
	event "skywalking.apache.org/repo/goapi/collect/event/v3"
	v3 "skywalking.apache.org/repo/goapi/collect/language/agent/v3"
	profile "skywalking.apache.org/repo/goapi/collect/language/profile/v3"
	management "skywalking.apache.org/repo/goapi/collect/management/v3"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/skywalkingreceiver/internal/metrics"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/skywalkingreceiver/internal/trace"
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
	config *configuration

	grpc            *grpc.Server
	collectorServer *http.Server

	goroutines sync.WaitGroup

	settings receiver.CreateSettings

	traceReceiver *trace.Receiver

	metricsReceiver *metrics.Receiver

	dummyReportService *dummyReportService
}

// newSkywalkingReceiver creates a TracesReceiver that receives traffic as a Skywalking collector
func newSkywalkingReceiver(
	config *configuration,
	set receiver.CreateSettings,
) *swReceiver {
	return &swReceiver{
		config:   config,
		settings: set,
	}
}

// registerTraceConsumer register a TracesReceiver that receives trace
func (sr *swReceiver) registerTraceConsumer(tc consumer.Traces) error {
	if tc == nil {
		return component.ErrNilNextConsumer
	}
	var err error
	sr.traceReceiver, err = trace.NewReceiver(tc, sr.settings)
	if err != nil {
		return err
	}
	return nil
}

// registerTraceConsumer register a TracesReceiver that receives trace
func (sr *swReceiver) registerMetricsConsumer(mc consumer.Metrics) error {
	if mc == nil {
		return component.ErrNilNextConsumer
	}
	var err error
	sr.metricsReceiver, err = metrics.NewReceiver(mc, sr.settings)
	if err != nil {
		return err
	}
	return nil
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
		nr.HandleFunc("/v3/segments", sr.traceReceiver.HTTPHandler).Methods(http.MethodPost)
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
		if sr.traceReceiver != nil {
			v3.RegisterTraceSegmentReportServiceServer(sr.grpc, sr.traceReceiver)
		}
		if sr.metricsReceiver != nil {
			v3.RegisterJVMMetricReportServiceServer(sr.grpc, sr.metricsReceiver)
		}
		sr.dummyReportService = &dummyReportService{}
		management.RegisterManagementServiceServer(sr.grpc, sr.dummyReportService)
		cds.RegisterConfigurationDiscoveryServiceServer(sr.grpc, sr.dummyReportService)
		event.RegisterEventServiceServer(sr.grpc, &eventService{})
		profile.RegisterProfileTaskServer(sr.grpc, sr.dummyReportService)
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
