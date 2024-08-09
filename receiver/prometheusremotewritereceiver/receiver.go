// // Copyright The OpenTelemetry Authors
// // SPDX-License-Identifier: Apache-2.0

package prometheusremotewritereceiver

import (
	"context"
	"errors"
	"net"
	"net/http"
	"sync"

	"go.uber.org/zap"
	"google.golang.org/grpc"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/consumer"

	"go.opentelemetry.io/collector/receiver"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusremotewritereceiver/internal/metrics"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
)

// remoteWriteReceiver is the type that exposes Trace and Metrics reception.
type remoteWriteReceiver struct {
	cfg        *Config
	serverGRPC *grpc.Server
	httpMux    *http.ServeMux
	serverHTTP *http.Server
	consumer   consumer.Metrics

	metricsReceiver *metrics.Receiver
	shutdownWG sync.WaitGroup

	obsrepGRPC *receiverhelper.ObsReport
	obsrepHTTP *receiverhelper.ObsReport

	settings *receiver.CreateSettings
}

// newRemoteWriteReceiver just creates the OpenTelemetry receiver services. It is the caller's
// responsibility to invoke the respective Start*Reception methods as well
// as the various Stop*Reception methods to end it.
func newRemoteWriteReceiver(set *receiver.CreateSettings, cfg *Config, next consumer.Metrics) (*remoteWriteReceiver, error) {
	r := &remoteWriteReceiver{
		cfg:      cfg,
		settings: set,
		consumer: next,
	}
	if cfg.HTTP != nil {
		r.httpMux = http.NewServeMux()
	}

	var err error
	r.obsrepGRPC, err = receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{
		ReceiverID:             set.ID,
		Transport:              "grpc",
		ReceiverCreateSettings: *set,
	})
	if err != nil {
		return nil, err
	}
	r.obsrepHTTP, err = receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{
		ReceiverID:             set.ID,
		Transport:              "http",
		ReceiverCreateSettings: *set,
	})
	if err != nil {
		return nil, err
	}

	return r, nil
}

func (r *remoteWriteReceiver) startGRPCServer(host component.Host) error {

	// If GRPC is not enabled, nothing to start.
	if r.cfg.GRPC == nil {
		return nil
	}
	var err error
	if r.serverGRPC, err = r.cfg.GRPC.ToServer(context.Background(), host, r.settings.TelemetrySettings); err != nil {
		return err
	}

	r.settings.Logger.Info("Starting GRPC server", zap.String("endpoint", r.cfg.GRPC.NetAddr.Endpoint))
	var gln net.Listener
	if gln, err = r.cfg.GRPC.NetAddr.Listen(context.Background()); err != nil {
		return err
	}

	r.shutdownWG.Add(1)
	go func() {
		defer r.shutdownWG.Done()

		if errGrpc := r.serverGRPC.Serve(gln); errGrpc != nil && !errors.Is(errGrpc, grpc.ErrServerStopped) {
			r.settings.ReportStatus(component.NewFatalErrorEvent(errGrpc))
		}
	}()
	return nil
}

func (r *remoteWriteReceiver) startHTTPServer(ctx context.Context, host component.Host) error {
	if r.cfg.HTTP == nil {
		return nil
	}

	httpMux := http.NewServeMux()

	if r.consumer != nil {
		httpMetricsReceiver := metrics.New(r.consumer, r.obsrepHTTP, r.cfg.MetricTypes)
		httpMux.HandleFunc(r.cfg.HTTP.MetricsURLPath, func(resp http.ResponseWriter, req *http.Request) {
			handleMetrics(resp, req, httpMetricsReceiver)
		})
	}

	var err error
	if r.serverHTTP, err = r.cfg.HTTP.ToServer(ctx, host, r.settings.TelemetrySettings, httpMux, confighttp.WithErrorHandler(errorHandler)); err != nil {
		return err
	}

	r.settings.Logger.Info("Starting HTTP server", zap.String("endpoint", r.cfg.HTTP.ServerConfig.Endpoint))
	var hln net.Listener
	if hln, err = r.cfg.HTTP.ServerConfig.ToListener(ctx); err != nil {
		return err
	}
	r.shutdownWG.Add(1)

	go func() {
		defer r.shutdownWG.Done()

		if errHTTP := r.serverHTTP.Serve(hln); errHTTP != nil && !errors.Is(errHTTP, http.ErrServerClosed) {
			r.settings.ReportStatus(component.NewFatalErrorEvent(errHTTP))
		}
	}()
	return nil
}

// Start runs the trace receiver on the gRPC server. Currently
// it also enables the metrics receiver too.
func (r *remoteWriteReceiver) Start(ctx context.Context, host component.Host) error {
	if err := r.startGRPCServer(host); err != nil {
		return err
	}
	if err := r.startHTTPServer(ctx, host); err != nil {
		// It's possible that a valid GRPC server configuration was specified,
		// but an invalid HTTP configuration. If that's the case, the successfully
		// started GRPC server must be shutdown to ensure no goroutines are leaked.
		return errors.Join(err, r.Shutdown(ctx))
	}

	return nil
}

// // Shutdown is a method to turn off receiving.
func (r *remoteWriteReceiver) Shutdown(ctx context.Context) error {
	var err error

	if r.serverHTTP != nil {
		err = r.serverHTTP.Shutdown(ctx)
	}

	if r.serverGRPC != nil {
		r.serverGRPC.GracefulStop()
	}

	r.shutdownWG.Wait()
	return err
}

func (r *remoteWriteReceiver) registerMetricsConsumer(mc consumer.Metrics) {
	r.consumer = mc
}
