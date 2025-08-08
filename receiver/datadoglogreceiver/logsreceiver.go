package datadoglogreceiver

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"sync"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadoglogreceiver/internal"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.uber.org/zap"
)

const (
	jsonContentType = "application/json"
)

type datadoglogReceiver struct {
	host         component.Host
	settings     receiver.Settings
	cancel       context.CancelFunc
	logger       *zap.Logger
	nextConsumer consumer.Logs
	config       *Config
	shutdownWG   sync.WaitGroup
	httpMux      *http.ServeMux
	serverHTTP   *http.Server

	obsrepHTTP *receiverhelper.ObsReport
}

func newDatadogLogReceiver(
	config *Config,
	nextConsumer consumer.Logs,
	settings receiver.Settings,
) (*datadoglogReceiver, error) {
	r := &datadoglogReceiver{
		logger:       settings.Logger,
		nextConsumer: nextConsumer,
		config:       config,
		settings:     settings,
		httpMux:      http.NewServeMux(),
	}

	var err error

	r.obsrepHTTP, err = receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{
		ReceiverID:             settings.ID,
		Transport:              "http",
		ReceiverCreateSettings: settings,
	})
	if err != nil {
		return nil, err
	}

	r.httpMux.HandleFunc("/api/v2/logs", func(resp http.ResponseWriter, req *http.Request) {
		r.settings.Logger.Debug("Datadog Logs receiver /api/v2/logs handler called")
		if req.Method != http.MethodPost {
			handleUnmatchedMethod(resp)
			return
		}
		switch req.Header.Get("Content-Type") {
		case jsonContentType:
			handleLogs(resp, req, r)
		default:
			handleUnmatchedContentType(resp)
		}
	})

	return r, nil
}

func handleUnmatchedMethod(resp http.ResponseWriter) {
	status := http.StatusMethodNotAllowed
	writeResponse(
		resp,
		"text/plain",
		status,
		[]byte(fmt.Sprintf("%v method not allowed, supported: [POST]", status)),
	)
}

func handleUnmatchedContentType(resp http.ResponseWriter) {
	status := http.StatusUnsupportedMediaType
	writeResponse(
		resp,
		"text/plain",
		status,
		[]byte(fmt.Sprintf("%v unsupported media type, supported: [%s]", status, jsonContentType)),
	)
}

func writeResponse(w http.ResponseWriter, contentType string, statusCode int, msg []byte) {
	if contentType != "text/plain" && contentType != "application/json" {
		contentType = "text/plain"
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("X-Content-Type-Options", "nosniff") // Prevent content type sniffing
	w.WriteHeader(statusCode)

	tmpl := template.Must(template.New("response").Parse("{{.}}"))
	_ = tmpl.Execute(w, string(msg))
}

func handleLogs(resp http.ResponseWriter, req *http.Request, r *datadoglogReceiver) {
	logs, err := internal.ParseRequest(req, r.settings.Logger, r.config.EnableDdtagsAttribute)
	if err != nil {
		http.Error(resp, err.Error(), http.StatusBadRequest)
		return
	}
	ctx := r.obsrepHTTP.StartLogsOp(req.Context())
	logRecordCount := logs.LogRecordCount()
	err = r.nextConsumer.ConsumeLogs(ctx, *logs)
	r.obsrepHTTP.EndLogsOp(ctx, "json", logRecordCount, err)

	resp.WriteHeader(http.StatusNoContent)
}

func (receiver *datadoglogReceiver) Start(ctx context.Context, host component.Host) error {
	receiver.logger.Info(
		"Starting Datadog Logs receiver",
		zap.String("endpoint", receiver.config.HTTP.Endpoint),
	)
	var err error
	receiver.serverHTTP, err = receiver.config.HTTP.ToServer(
		ctx,
		host,
		receiver.settings.TelemetrySettings,
		receiver.httpMux,
	)
	if err != nil {
		return fmt.Errorf("failed create http server error: %w", err)
	}
	err = receiver.startHTTPServer(ctx)
	if err != nil {
		return fmt.Errorf("failed to start http server error: %w", err)
	}

	receiver.host = host

	return err
}

func (r *datadoglogReceiver) startHTTPServer(ctx context.Context) error {
	r.settings.Logger.Info("Starting HTTP server", zap.String("endpoint", r.config.HTTP.Endpoint))
	listener, err := r.config.HTTP.ToListener(ctx)
	if err != nil {
		r.settings.Logger.Error("Failed to bind to address", zap.Error(err))
		return err
	}
	r.shutdownWG.Add(1)

	go func() {
		defer r.shutdownWG.Done()
		if errHTTP := r.serverHTTP.Serve(listener); !errors.Is(errHTTP, http.ErrServerClosed) &&
			errHTTP != nil {
			r.settings.Logger.Error("HTTP server stopped unexpectedly", zap.Error(errHTTP))
			componentstatus.ReportStatus(r.host, componentstatus.NewFatalErrorEvent(errHTTP))
		}
	}()
	return nil
}

func (receiver *datadoglogReceiver) Shutdown(ctx context.Context) error {
	if receiver.cancel != nil {
		receiver.cancel()
	}

	var err error

	if receiver.serverHTTP != nil {
		err = receiver.serverHTTP.Shutdown(ctx)
	}

	receiver.shutdownWG.Wait()
	return err
}
