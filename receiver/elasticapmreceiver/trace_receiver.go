package elasticapmreceiver

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"

	"go.opentelemetry.io/collector/component/componentstatus"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.uber.org/zap"
	"golang.org/x/sync/semaphore"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/elastic/apm-data/input/elasticapm"
	"github.com/elastic/apm-data/model/modelpb"
	"github.com/zgsolucoes/opentelemetry-collector-contrib/receiver/elasticapmreceiver/translator"
)

type elasticapmReceiver struct {
	cfg        *Config
	serverHTTP *http.Server
	httpMux    *http.ServeMux
	shutdownWG sync.WaitGroup

	nextConsumer  consumer.Traces
	traceReceiver *receiverhelper.ObsReport
	settings      receiver.Settings
}

func newElasticAPMReceiver(cfg *Config, settings receiver.Settings) (*elasticapmReceiver, error) {
	r := &elasticapmReceiver{
		cfg:      cfg,
		settings: settings,
		httpMux:  http.NewServeMux(),
	}

	var err error
	r.traceReceiver, err = receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{
		ReceiverID:             settings.ID,
		Transport:              "http",
		ReceiverCreateSettings: settings,
	})
	if err != nil {
		return nil, err
	}
	return r, nil
}

func (r *elasticapmReceiver) startHTTPServer(ctx context.Context, cfg *confighttp.ServerConfig, host component.Host) error {
	r.settings.Logger.Info("Starting HTTP server", zap.String("endpoint", cfg.Endpoint))
	listener, err := cfg.ToListener(ctx)
	if err != nil {
		return err
	}

	r.shutdownWG.Add(1)
	go func() {
		defer r.shutdownWG.Done()
		go func() {
			defer r.shutdownWG.Done()

			if errHTTP := r.serverHTTP.Serve(listener); !errors.Is(errHTTP, http.ErrServerClosed) && errHTTP != nil {
				componentstatus.ReportStatus(host, componentstatus.NewFatalErrorEvent(errHTTP))
			}
		}()
	}()
	return nil
}

func (r *elasticapmReceiver) Start(ctx context.Context, host component.Host) error {
	var err error
	r.serverHTTP, err = r.cfg.ServerConfig.ToServer(
		ctx,
		host,
		r.settings.TelemetrySettings,
		r.httpMux,
	)
	if err != nil {
		return err
	}

	err = r.startHTTPServer(ctx, r.cfg.ServerConfig, host)
	return err
}

func (r *elasticapmReceiver) Shutdown(ctx context.Context) error {
	var err error

	if r.serverHTTP != nil {
		err = r.serverHTTP.Shutdown(ctx)
	}

	r.shutdownWG.Wait()
	return err
}

func (r *elasticapmReceiver) registerTraceConsumer(nextConsumer consumer.Traces) error {
	if nextConsumer == nil {
		return nil
	}

	r.nextConsumer = nextConsumer

	if r.httpMux != nil {
		r.httpMux.HandleFunc(r.cfg.EventsURLPath, wrapper(r.handleEvents))
		r.httpMux.HandleFunc(r.cfg.RUMEventsUrlPath, wrapper(r.handleRUMEvents))
	}

	return nil
}

func wrapper(f http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "Only POST requests are supported")
			return
		}

		switch req.Header.Get("Content-Type") {
		// Only parse ndjson
		case "application/x-ndjson":
			f(w, req)
		default:
			writeError(w, http.StatusUnsupportedMediaType, "Only application/ndjson is supported")
			return
		}
	}
}

func (r *elasticapmReceiver) handleEvents(w http.ResponseWriter, req *http.Request) {
	traceData, _ := r.handleTraces(w, req, &modelpb.APMEvent{}, r.traceReceiver)
	if traceData != nil {
		r.nextConsumer.ConsumeTraces(req.Context(), *traceData)
	}
}

func (r *elasticapmReceiver) handleRUMEvents(w http.ResponseWriter, req *http.Request) {
	baseEvent := &modelpb.APMEvent{
		Timestamp: timestamppb.Now(),
	}
	traceData, _ := r.handleTraces(w, req, baseEvent, r.traceReceiver)
	if traceData != nil {
		r.nextConsumer.ConsumeTraces(req.Context(), *traceData)
	}
}

// Process a batch of events, returning the events and any error.
// The baseEvent is extended by the metadata in the batch and used as the base event for all events in the batch.
func (r *elasticapmReceiver) processBatch(reader io.Reader, baseEvent *modelpb.APMEvent) ([]*modelpb.APMEvent, error) {
	events := []*modelpb.APMEvent{}
	processor := elasticapm.NewProcessor(elasticapm.Config{
		MaxEventSize: r.cfg.MaxEventSize,
		Semaphore:    semaphore.NewWeighted(1),
		Logger:       r.settings.Logger,
	})

	batchProcessor := modelpb.ProcessBatchFunc(func(ctx context.Context, batch *modelpb.Batch) error {
		events = append(events, (*batch)...)
		return nil
	})

	result := elasticapm.Result{}

	err := processor.HandleStream(
		context.Background(),
		false,
		baseEvent,
		reader,
		r.cfg.BatchSize,
		batchProcessor,
		&result,
	)
	if err != nil {
		return nil, err
	}

	return events, nil
}

func (r *elasticapmReceiver) handleTraces(w http.ResponseWriter, req *http.Request, baseEvent *modelpb.APMEvent, tracesReceiver *receiverhelper.ObsReport) (*ptrace.Traces, error) {
	events, err := r.processBatch(req.Body, baseEvent)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Unable to decode events. Do you have valid ndjson?")
		return nil, err
	}

	traceData := ptrace.NewTraces()
	resourceSpans := traceData.ResourceSpans().AppendEmpty()
	scopeSpans := resourceSpans.ScopeSpans().AppendEmpty()
	spans := scopeSpans.Spans()
	resource := resourceSpans.Resource()

	translator.ConvertMetadata(baseEvent, resource)

	for _, event := range events {
		switch event.Type() {
		case modelpb.TransactionEventType:
			translator.ConvertTransaction(event, spans.AppendEmpty())
		case modelpb.SpanEventType:
			translator.ConvertSpan(event, spans.AppendEmpty())
		case modelpb.ErrorEventType:
			fmt.Println("Ignoring error")
		case modelpb.MetricEventType:
			fmt.Println("Ignoring metricset")
		case modelpb.LogEventType:
			fmt.Println("Ignoring log")
		default:
			fmt.Println("Unknown event type")
		}
	}

	return &traceData, nil
}

func writeError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("content-type", "text/plain")
	w.WriteHeader(statusCode)
	w.Write([]byte(message))
}
