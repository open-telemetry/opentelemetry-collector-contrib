// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package lokireceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/lokireceiver"

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"

	"github.com/grafana/loki/pkg/push"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/loki"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/lokireceiver/internal"
)

const (
	pbContentType   = "application/x-protobuf"
	jsonContentType = "application/json"
)

const ErrAtLeastOneEntryFailedToProcess = "at least one entry in the push request failed to process"

type lokiReceiver struct {
	conf         *Config
	nextConsumer consumer.Logs
	settings     receiver.CreateSettings
	httpMux      *http.ServeMux
	serverHTTP   *http.Server
	serverGRPC   *grpc.Server
	shutdownWG   sync.WaitGroup

	obsrepGRPC *receiverhelper.ObsReport
	obsrepHTTP *receiverhelper.ObsReport
}

func newLokiReceiver(conf *Config, nextConsumer consumer.Logs, settings receiver.CreateSettings) (*lokiReceiver, error) {
	r := &lokiReceiver{
		conf:         conf,
		nextConsumer: nextConsumer,
		settings:     settings,
	}

	var err error
	r.obsrepGRPC, err = receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{
		ReceiverID:             settings.ID,
		Transport:              "grpc",
		ReceiverCreateSettings: settings,
	})
	if err != nil {
		return nil, err
	}
	r.obsrepHTTP, err = receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{
		ReceiverID:             settings.ID,
		Transport:              "http",
		ReceiverCreateSettings: settings,
	})
	if err != nil {
		return nil, err
	}

	if nextConsumer == nil {
		return nil, component.ErrNilNextConsumer
	}

	if conf.HTTP != nil {
		r.httpMux = http.NewServeMux()
		r.httpMux.HandleFunc("/loki/api/v1/push", func(resp http.ResponseWriter, req *http.Request) {
			if req.Method != http.MethodPost {
				handleUnmatchedMethod(resp)
				return
			}
			switch req.Header.Get("Content-Type") {
			case jsonContentType, pbContentType:
				handleLogs(resp, req, r)
			default:
				handleUnmatchedContentType(resp)
			}
		})
	}

	return r, nil
}

func (r *lokiReceiver) startProtocolsServers(host component.Host) error {
	var err error
	if r.conf.HTTP != nil {
		r.serverHTTP, err = r.conf.HTTP.ToServer(host, r.settings.TelemetrySettings, r.httpMux, confighttp.WithDecoder("snappy", func(body io.ReadCloser) (io.ReadCloser, error) { return body, nil }))
		if err != nil {
			return fmt.Errorf("failed create http server error: %w", err)
		}
		err = r.startHTTPServer(host)
		if err != nil {
			return fmt.Errorf("failed to start http server error: %w", err)
		}
	}

	if r.conf.GRPC != nil {
		r.serverGRPC, err = r.conf.GRPC.ToServer(host, r.settings.TelemetrySettings)
		if err != nil {
			return fmt.Errorf("failed create grpc server error: %w", err)
		}

		push.RegisterPusherServer(r.serverGRPC, r)

		err = r.startGRPCServer(host)
		if err != nil {
			return fmt.Errorf("failed to start grpc server error: %w", err)
		}
	}

	return err
}

func (r *lokiReceiver) startHTTPServer(host component.Host) error {
	r.settings.Logger.Info("Starting HTTP server", zap.String("endpoint", r.conf.HTTP.Endpoint))
	listener, err := r.conf.HTTP.ToListener()
	if err != nil {
		return err
	}
	r.shutdownWG.Add(1)

	go func() {
		defer r.shutdownWG.Done()
		if errHTTP := r.serverHTTP.Serve(listener); !errors.Is(errHTTP, http.ErrServerClosed) && errHTTP != nil {
			host.ReportFatalError(errHTTP)
		}
	}()
	return nil
}

func (r *lokiReceiver) startGRPCServer(host component.Host) error {
	r.settings.Logger.Info("Starting GRPC server", zap.String("endpoint", r.conf.GRPC.NetAddr.Endpoint))
	listener, err := r.conf.GRPC.ToListener()
	if err != nil {
		return err
	}
	r.shutdownWG.Add(1)

	go func() {
		defer r.shutdownWG.Done()
		if errGRPC := r.serverGRPC.Serve(listener); !errors.Is(errGRPC, grpc.ErrServerStopped) && errGRPC != nil {
			host.ReportFatalError(errGRPC)
		}
	}()
	return nil
}

func (r *lokiReceiver) Push(ctx context.Context, pushRequest *push.PushRequest) (*push.PushResponse, error) {
	logs, err := loki.PushRequestToLogs(pushRequest, r.conf.KeepTimestamp)
	if err != nil {
		r.settings.Logger.Warn(ErrAtLeastOneEntryFailedToProcess, zap.Error(err))
		return &push.PushResponse{}, err
	}
	ctx = r.obsrepGRPC.StartLogsOp(ctx)
	logRecordCount := logs.LogRecordCount()
	err = r.nextConsumer.ConsumeLogs(ctx, logs)
	r.obsrepGRPC.EndLogsOp(ctx, "protobuf", logRecordCount, err)
	return &push.PushResponse{}, nil
}

func (r *lokiReceiver) Start(_ context.Context, host component.Host) error {
	return r.startProtocolsServers(host)
}

func (r *lokiReceiver) Shutdown(ctx context.Context) error {
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

func handleUnmatchedMethod(resp http.ResponseWriter) {
	status := http.StatusMethodNotAllowed
	writeResponse(resp, "text/plain", status, []byte(fmt.Sprintf("%v method not allowed, supported: [POST]", status)))
}

func handleUnmatchedContentType(resp http.ResponseWriter) {
	status := http.StatusUnsupportedMediaType
	writeResponse(resp, "text/plain", status, []byte(fmt.Sprintf("%v unsupported media type, supported: [%s, %s]", status, jsonContentType, pbContentType)))
}

func writeResponse(w http.ResponseWriter, contentType string, statusCode int, msg []byte) {
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(statusCode)
	// Nothing we can do with the error if we cannot write to the response.
	_, _ = w.Write(msg)
}

func handleLogs(resp http.ResponseWriter, req *http.Request, r *lokiReceiver) {
	pushRequest, err := internal.ParseRequest(req)
	if err != nil {
		http.Error(resp, err.Error(), http.StatusBadRequest)
		return
	}

	logs, err := loki.PushRequestToLogs(pushRequest, r.conf.KeepTimestamp)
	if err != nil {
		r.settings.Logger.Warn(ErrAtLeastOneEntryFailedToProcess, zap.Error(err))
		http.Error(resp, err.Error(), http.StatusBadRequest)
		return
	}
	ctx := r.obsrepHTTP.StartLogsOp(req.Context())
	logRecordCount := logs.LogRecordCount()
	err = r.nextConsumer.ConsumeLogs(ctx, logs)
	r.obsrepHTTP.EndLogsOp(ctx, "json", logRecordCount, err)

	resp.WriteHeader(http.StatusNoContent)
}
