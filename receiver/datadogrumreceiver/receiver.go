package datadogrumreceiver

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/rs/cors"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadogrumreceiver/internal/translator"
)

type datadogRUMReceiver struct {
	address string
	config  *Config
	params  receiver.Settings

	nextTracesConsumer consumer.Traces
	nextLogsConsumer   consumer.Logs

	server    *http.Server
	lReceiver *receiverhelper.ObsReport
}

func newDataDogRUMReceiver(config *Config, params receiver.Settings) (component.Component, error) {
	instance, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{LongLivedCtx: false, ReceiverID: params.ID, Transport: "http", ReceiverCreateSettings: params})
	if err != nil {
		return nil, err
	}

	return &datadogRUMReceiver{
		params: params,
		config: config,
		server: &http.Server{
			ReadTimeout: config.ReadTimeout,
		},
		lReceiver: instance,
	}, nil
}

func (ddr *datadogRUMReceiver) Start(ctx context.Context, host component.Host) error {
	ddmux := http.NewServeMux()

	ddmux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	if ddr.nextTracesConsumer != nil || ddr.nextLogsConsumer != nil {
		ddmux.HandleFunc("/api/v2/rum", ddr.handleEvent)
	}

	var err error

	// TODO: look over this
	corsHandler := cors.New(cors.Options{
		AllowedOrigins:   []string{"https://localhost:*", "http://localhost:*"}, // Specify allowed origins
		AllowedMethods:   []string{"GET", "POST", "OPTIONS"},                    // Specify allowed methods
		AllowedHeaders:   []string{"Content-Type", "Authorization"},             // Specify allowed headers
		AllowCredentials: true,                                                  // Allow credentials
	}).Handler(ddmux)

	ddr.server, err = ddr.config.ServerConfig.ToServer(
		ctx,
		host,
		ddr.params.TelemetrySettings,
		corsHandler,
	)
	if err != nil {
		return fmt.Errorf("failed to create server definition: %w", err)
	}
	hln, err := ddr.config.ServerConfig.ToListener(ctx)
	if err != nil {
		return fmt.Errorf("failed to create datadog listener: %w", err)
	}

	ddr.address = hln.Addr().String()

	go func() {
		if err := ddr.server.Serve(hln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			componentstatus.ReportStatus(host, componentstatus.NewFatalErrorEvent(fmt.Errorf("error starting datadog receiver: %w", err)))
		}
	}()
	return nil
}

func (ddr *datadogRUMReceiver) handleEvent(w http.ResponseWriter, req *http.Request) {
	obsCtx := ddr.lReceiver.StartTracesOp(req.Context())
	var err error
	var eventCount int
	defer func(eventCount *int) {
		ddr.lReceiver.EndTracesOp(obsCtx, "datadog", *eventCount, err)
	}(&eventCount)

	defer func() {
		_, errs := io.Copy(io.Discard, req.Body)
		err = errors.Join(err, errs, req.Body.Close())
	}()

	buf := translator.GetBuffer()
	defer translator.PutBuffer(buf)
	_, err = io.Copy(buf, req.Body)
	if err != nil {
		http.Error(w, "Unable to read request body", http.StatusBadRequest)
		ddr.params.Logger.Error("Unable to read request body", zap.Error(err))
		return
	}

	fmt.Printf("&&&&&&&&&& RECEIVED RUM REQUEST BODY: %v\n", buf.String())

	_, _ = w.Write([]byte("OK"))
}

func (ddr *datadogRUMReceiver) Shutdown(ctx context.Context) (err error) {
	return ddr.server.Shutdown(ctx)
}
