// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otelarrowreceiver // import "github.com/open-telemetry/otel-arrow/collector/receiver/otelarrowreceiver"

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"

	arrowpb "github.com/open-telemetry/otel-arrow/api/experimental/arrow/v1"
	arrowRecord "github.com/open-telemetry/otel-arrow/pkg/otel/arrow_record"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	"github.com/open-telemetry/otel-arrow/collector/netstats"
	"github.com/open-telemetry/otel-arrow/collector/receiver/otelarrowreceiver/internal/arrow"
	"github.com/open-telemetry/otel-arrow/collector/receiver/otelarrowreceiver/internal/logs"
	"github.com/open-telemetry/otel-arrow/collector/receiver/otelarrowreceiver/internal/metrics"
	"github.com/open-telemetry/otel-arrow/collector/receiver/otelarrowreceiver/internal/trace"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/extension/auth"
	"go.opentelemetry.io/collector/obsreport"
	"go.opentelemetry.io/collector/pdata/plog/plogotlp"
	"go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"
	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"
	"go.opentelemetry.io/collector/receiver"
)

// otlpReceiver is the type that exposes Trace and Metrics reception.
type otlpReceiver struct {
	cfg        *Config
	serverGRPC *grpc.Server
	httpMux    *http.ServeMux
	serverHTTP *http.Server

	tracesReceiver  *trace.Receiver
	metricsReceiver *metrics.Receiver
	logsReceiver    *logs.Receiver
	arrowReceiver   *arrow.Receiver
	shutdownWG      sync.WaitGroup

	obsrepGRPC *obsreport.Receiver
	obsrepHTTP *obsreport.Receiver
	netStats   *netstats.NetworkReporter

	settings receiver.CreateSettings
}

// newOtlpReceiver just creates the OpenTelemetry receiver services. It is the caller's
// responsibility to invoke the respective Start*Reception methods as well
// as the various Stop*Reception methods to end it.
func newOtlpReceiver(cfg *Config, set receiver.CreateSettings) (*otlpReceiver, error) {
	netStats, err := netstats.NewReceiverNetworkReporter(set)
	if err != nil {
		return nil, err
	}
	r := &otlpReceiver{
		cfg:      cfg,
		settings: set,
		netStats: netStats,
	}
	if cfg.HTTP != nil {
		r.httpMux = http.NewServeMux()
	}

	r.obsrepGRPC, err = obsreport.NewReceiver(obsreport.ReceiverSettings{
		ReceiverID:             set.ID,
		Transport:              "grpc",
		ReceiverCreateSettings: set,
	})
	if err != nil {
		return nil, err
	}
	r.obsrepHTTP, err = obsreport.NewReceiver(obsreport.ReceiverSettings{
		ReceiverID:             set.ID,
		Transport:              "http",
		ReceiverCreateSettings: set,
	})
	if err != nil {
		return nil, err
	}

	return r, nil
}

func (r *otlpReceiver) startGRPCServer(cfg *configgrpc.GRPCServerSettings, host component.Host) error {
	r.settings.Logger.Info("Starting GRPC server", zap.String("endpoint", cfg.NetAddr.Endpoint))

	gln, err := cfg.ToListener()
	if err != nil {
		return err
	}
	r.shutdownWG.Add(1)
	go func() {
		defer r.shutdownWG.Done()

		if errGrpc := r.serverGRPC.Serve(gln); errGrpc != nil && !errors.Is(errGrpc, grpc.ErrServerStopped) {
			host.ReportFatalError(errGrpc)
		}
	}()
	return nil
}

func (r *otlpReceiver) startHTTPServer(cfg *confighttp.HTTPServerSettings, host component.Host) error {
	r.settings.Logger.Info("Starting HTTP server", zap.String("endpoint", cfg.Endpoint))
	var hln net.Listener
	hln, err := cfg.ToListener()
	if err != nil {
		return err
	}
	r.shutdownWG.Add(1)
	go func() {
		defer r.shutdownWG.Done()

		if errHTTP := r.serverHTTP.Serve(hln); errHTTP != nil && !errors.Is(errHTTP, http.ErrServerClosed) {
			host.ReportFatalError(errHTTP)
		}
	}()
	return nil
}

func (r *otlpReceiver) startProtocolServers(host component.Host) error {
	var err error
	if r.cfg.GRPC != nil {
		var serverOpts []grpc.ServerOption

		if r.netStats != nil {
			serverOpts = append(serverOpts, grpc.StatsHandler(r.netStats))
		}
		r.serverGRPC, err = r.cfg.GRPC.ToServer(host, r.settings.TelemetrySettings, serverOpts...)
		if err != nil {
			return err
		}

		if r.cfg.Arrow != nil {
			var authServer auth.Server
			if r.cfg.GRPC.Auth != nil {
				authServer, err = r.cfg.GRPC.Auth.GetServerAuthenticator(host.GetExtensions())
				if err != nil {
					return err
				}
			}

			r.arrowReceiver = arrow.New(arrow.Consumers(r), r.settings, r.obsrepGRPC, r.cfg.GRPC, authServer, func() arrowRecord.ConsumerAPI {
				var opts []arrowRecord.Option
				if r.cfg.Arrow.MemoryLimit != 0 {
					// in which case the default is selected in the arrowRecord package.
					opts = append(opts, arrowRecord.WithMemoryLimit(r.cfg.Arrow.MemoryLimit))
				}
				return arrowRecord.NewConsumer(opts...)
			})

			arrowpb.RegisterArrowStreamServiceServer(r.serverGRPC, r.arrowReceiver)
		}

		if r.tracesReceiver != nil {
			ptraceotlp.RegisterGRPCServer(r.serverGRPC, r.tracesReceiver)

			if r.cfg.Arrow != nil {
				arrowpb.RegisterArrowTracesServiceServer(r.serverGRPC, r.arrowReceiver)
			}
		}

		if r.metricsReceiver != nil {
			pmetricotlp.RegisterGRPCServer(r.serverGRPC, r.metricsReceiver)

			if r.cfg.Arrow != nil {
				arrowpb.RegisterArrowMetricsServiceServer(r.serverGRPC, r.arrowReceiver)
			}
		}

		if r.logsReceiver != nil {
			plogotlp.RegisterGRPCServer(r.serverGRPC, r.logsReceiver)

			if r.cfg.Arrow != nil {
				arrowpb.RegisterArrowLogsServiceServer(r.serverGRPC, r.arrowReceiver)
			}
		}

		err = r.startGRPCServer(r.cfg.GRPC, host)
		if err != nil {
			return err
		}
	}
	if r.cfg.HTTP != nil {
		r.serverHTTP, err = r.cfg.HTTP.ToServer(
			host,
			r.settings.TelemetrySettings,
			r.httpMux,
			confighttp.WithErrorHandler(errorHandler),
		)
		if err != nil {
			return err
		}

		err = r.startHTTPServer(r.cfg.HTTP.HTTPServerSettings, host)
		if err != nil {
			return err
		}
	}

	return err
}

// Start runs the trace receiver on the gRPC server. Currently
// it also enables the metrics receiver too.
func (r *otlpReceiver) Start(_ context.Context, host component.Host) error {
	return r.startProtocolServers(host)
}

// Shutdown is a method to turn off receiving.
func (r *otlpReceiver) Shutdown(ctx context.Context) error {
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

func (r *otlpReceiver) registerTraceConsumer(tc consumer.Traces) error {
	if tc == nil {
		return component.ErrNilNextConsumer
	}
	r.tracesReceiver = trace.New(tc, r.obsrepGRPC)
	httpTracesReceiver := trace.New(tc, r.obsrepHTTP)
	if r.httpMux != nil {
		r.httpMux.HandleFunc(r.cfg.HTTP.TracesURLPath, func(resp http.ResponseWriter, req *http.Request) {
			if req.Method != http.MethodPost {
				handleUnmatchedMethod(resp)
				return
			}
			switch getMimeTypeFromContentType(req.Header.Get("Content-Type")) {
			case pbContentType:
				handleTraces(resp, req, httpTracesReceiver, pbEncoder)
			case jsonContentType:
				handleTraces(resp, req, httpTracesReceiver, jsEncoder)
			default:
				handleUnmatchedContentType(resp)
			}
		})
	}
	return nil
}

func (r *otlpReceiver) registerMetricsConsumer(mc consumer.Metrics) error {
	if mc == nil {
		return component.ErrNilNextConsumer
	}
	r.metricsReceiver = metrics.New(mc, r.obsrepGRPC)
	httpMetricsReceiver := metrics.New(mc, r.obsrepHTTP)
	if r.httpMux != nil {
		r.httpMux.HandleFunc(r.cfg.HTTP.MetricsURLPath, func(resp http.ResponseWriter, req *http.Request) {
			if req.Method != http.MethodPost {
				handleUnmatchedMethod(resp)
				return
			}
			switch getMimeTypeFromContentType(req.Header.Get("Content-Type")) {
			case pbContentType:
				handleMetrics(resp, req, httpMetricsReceiver, pbEncoder)
			case jsonContentType:
				handleMetrics(resp, req, httpMetricsReceiver, jsEncoder)
			default:
				handleUnmatchedContentType(resp)
			}
		})
	}
	return nil
}

func (r *otlpReceiver) registerLogsConsumer(lc consumer.Logs) error {
	if lc == nil {
		return component.ErrNilNextConsumer
	}
	r.logsReceiver = logs.New(lc, r.obsrepGRPC)
	httpLogsReceiver := logs.New(lc, r.obsrepHTTP)
	if r.httpMux != nil {
		r.httpMux.HandleFunc(r.cfg.HTTP.LogsURLPath, func(resp http.ResponseWriter, req *http.Request) {
			if req.Method != http.MethodPost {
				handleUnmatchedMethod(resp)
				return
			}
			switch getMimeTypeFromContentType(req.Header.Get("Content-Type")) {
			case pbContentType:
				handleLogs(resp, req, httpLogsReceiver, pbEncoder)
			case jsonContentType:
				handleLogs(resp, req, httpLogsReceiver, jsEncoder)
			default:
				handleUnmatchedContentType(resp)
			}
		})
	}
	return nil
}

func handleUnmatchedMethod(resp http.ResponseWriter) {
	status := http.StatusMethodNotAllowed
	writeResponse(resp, "text/plain", status, []byte(fmt.Sprintf("%v method not allowed, supported: [POST]", status)))
}

func handleUnmatchedContentType(resp http.ResponseWriter) {
	status := http.StatusUnsupportedMediaType
	writeResponse(resp, "text/plain", status, []byte(fmt.Sprintf("%v unsupported media type, supported: [%s, %s]", status, jsonContentType, pbContentType)))
}

var _ arrow.Consumers = &otlpReceiver{}

func (r *otlpReceiver) Traces() consumer.Traces {
	if r.tracesReceiver == nil {
		return nil
	}
	return r.tracesReceiver.Consumer()
}

func (r *otlpReceiver) Metrics() consumer.Metrics {
	if r.metricsReceiver == nil {
		return nil
	}
	return r.metricsReceiver.Consumer()
}

func (r *otlpReceiver) Logs() consumer.Logs {
	if r.logsReceiver == nil {
		return nil
	}
	return r.logsReceiver.Consumer()
}
