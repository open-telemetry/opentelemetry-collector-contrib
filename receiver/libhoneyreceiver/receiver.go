// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package libhoneyreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/libhoneyreceiver"

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"strings"
	"sync"

	"github.com/klauspost/compress/zstd"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/errorutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/libhoneyreceiver/internal/codec"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/libhoneyreceiver/internal/parser"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/libhoneyreceiver/internal/response"
)

type libhoneyReceiver struct {
	cfg        *Config
	server     *http.Server
	nextTraces consumer.Traces
	nextLogs   consumer.Logs
	shutdownWG sync.WaitGroup
	obsreport  *receiverhelper.ObsReport
	settings   *receiver.Settings
}

func newLibhoneyReceiver(cfg *Config, set *receiver.Settings) (*libhoneyReceiver, error) {
	r := &libhoneyReceiver{
		cfg:        cfg,
		nextTraces: nil,
		settings:   set,
	}

	var err error
	r.obsreport, err = receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{
		ReceiverID:             set.ID,
		Transport:              "http",
		ReceiverCreateSettings: *set,
	})
	if err != nil {
		return nil, err
	}

	return r, nil
}

func (r *libhoneyReceiver) startHTTPServer(ctx context.Context, host component.Host) error {
	// If HTTP is not enabled, nothing to start.
	if !r.cfg.HTTP.HasValue() {
		return nil
	}

	httpMux := http.NewServeMux()

	httpCfg := r.cfg.HTTP.Get()
	r.settings.Logger.Info("r.nextTraces is not null so httpTracesReceiver was added", zap.Int("paths", len(httpCfg.TracesURLPaths)))
	for _, path := range httpCfg.TracesURLPaths {
		httpMux.HandleFunc(path, func(resp http.ResponseWriter, req *http.Request) {
			r.handleEvent(resp, req)
		})
		r.settings.Logger.Debug("Added path to HTTP server", zap.String("path", path))
	}

	if r.cfg.AuthAPI != "" {
		httpMux.HandleFunc("/1/auth", func(resp http.ResponseWriter, req *http.Request) {
			r.handleAuth(resp, req)
		})
	}

	var err error
	if r.server, err = httpCfg.ToServer(ctx, host.GetExtensions(), r.settings.TelemetrySettings, httpMux); err != nil {
		return err
	}

	r.settings.Logger.Info("Starting HTTP server", zap.String("endpoint", httpCfg.NetAddr.Endpoint))
	var hln net.Listener
	if hln, err = httpCfg.ToListener(ctx); err != nil {
		return err
	}

	r.shutdownWG.Go(func() {
		if err := r.server.Serve(hln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			componentstatus.ReportStatus(host, componentstatus.NewFatalErrorEvent(err))
		}
	})
	return nil
}

func (r *libhoneyReceiver) Start(ctx context.Context, host component.Host) error {
	if err := r.startHTTPServer(ctx, host); err != nil {
		return errors.Join(err, r.Shutdown(ctx))
	}

	return nil
}

// Shutdown is a method to turn off receiving.
func (r *libhoneyReceiver) Shutdown(ctx context.Context) error {
	var err error

	if r.server != nil {
		err = r.server.Shutdown(ctx)
	}

	r.shutdownWG.Wait()
	return err
}

func (r *libhoneyReceiver) registerTraceConsumer(tc consumer.Traces) {
	r.nextTraces = tc
}

func (r *libhoneyReceiver) registerLogConsumer(tc consumer.Logs) {
	r.nextLogs = tc
}

func (r *libhoneyReceiver) handleAuth(resp http.ResponseWriter, req *http.Request) {
	r.settings.Logger.Debug("handling auth request", zap.String("auth_api", r.cfg.AuthAPI))
	authURL := fmt.Sprintf("%s/1/auth", r.cfg.AuthAPI)
	authReq, err := http.NewRequest(http.MethodGet, authURL, http.NoBody)
	if err != nil {
		errJSON, _ := json.Marshal(`{"error": "failed to create auth request"}`)
		writeResponse(resp, "json", http.StatusBadRequest, errJSON)
		return
	}
	authReq.Header.Set("x-honeycomb-team", req.Header.Get("x-honeycomb-team"))
	var authClient http.Client
	authResp, err := authClient.Do(authReq)
	if err != nil {
		errJSON, _ := json.Marshal(fmt.Sprintf(`"error": "failed to send request to auth api endpoint", "message", %q}`, err.Error()))
		writeResponse(resp, "json", http.StatusBadRequest, errJSON)
		return
	}
	defer authResp.Body.Close()

	switch {
	case authResp.StatusCode == http.StatusUnauthorized:
		errJSON, _ := json.Marshal(`"error": "received 401 response for authInfo request from Honeycomb API - check your API key"}`)
		writeResponse(resp, "json", http.StatusBadRequest, errJSON)
		return
	case authResp.StatusCode > 299:
		errJSON, _ := json.Marshal(fmt.Sprintf(`"error": "bad response code from API", "status_code", %d}`, authResp.StatusCode))
		writeResponse(resp, "json", http.StatusBadRequest, errJSON)
		return
	}
	authRawBody, _ := io.ReadAll(authResp.Body)
	_, err = resp.Write(authRawBody)
	if err != nil {
		r.settings.Logger.Info("couldn't write http response")
	}
}

// writeLibhoneyError writes a bad request error response in the appropriate format for libhoney clients
func writeLibhoneyError(resp http.ResponseWriter, enc codec.Encoder, errorMsg string) {
	errorResponse := []response.ResponseInBatch{{
		ErrorStr: errorMsg,
		Status:   http.StatusBadRequest,
	}}

	responseBody, err := enc.MarshalResponse(errorResponse)
	if err != nil {
		// Fallback to generic error if we can't marshal the response
		errorutil.HTTPError(resp, err)
		return
	}
	writeResponse(resp, enc.ContentType(), http.StatusBadRequest, responseBody)
}

// decompressBody handles decompression based on Content-Encoding header
// Returns an io.ReadCloser that must be closed by the caller
func decompressBody(body io.ReadCloser, contentEncoding string) (io.ReadCloser, error) {
	switch contentEncoding {
	case "", "identity":
		// No compression
		return body, nil
	case "gzip":
		gzipReader, err := gzip.NewReader(body)
		if err != nil {
			return nil, fmt.Errorf("failed to create gzip reader: %w", err)
		}
		return gzipReader, nil
	case "deflate":
		return flate.NewReader(body), nil
	case "zstd":
		zstdReader, err := zstd.NewReader(body)
		if err != nil {
			return nil, fmt.Errorf("failed to create zstd reader: %w", err)
		}
		return zstdReader.IOReadCloser(), nil
	default:
		return nil, fmt.Errorf("unsupported content encoding: %s", contentEncoding)
	}
}

func (r *libhoneyReceiver) handleEvent(resp http.ResponseWriter, req *http.Request) {
	enc, ok := readContentType(resp, req)
	if !ok {
		return
	}

	dataset, err := parser.GetDatasetFromRequest(req.RequestURI)
	if err != nil {
		r.settings.Logger.Info("No dataset found in URL", zap.String("req.RequestURI", req.RequestURI))
	}

	httpCfg := r.cfg.HTTP.Get()
	for _, p := range httpCfg.TracesURLPaths {
		dataset = strings.Replace(dataset, p, "", 1)
		r.settings.Logger.Debug("dataset parsed", zap.String("dataset.parsed", dataset))
	}

	apiKey := req.Header.Get("x-honeycomb-team")
	maskedKey := ""
	if len(apiKey) > 8 {
		maskedKey = apiKey[:4] + "..." + apiKey[len(apiKey)-4:]
	} else if apiKey != "" {
		maskedKey = "***"
	}

	// Buffer the compressed body first (like api.honeycomb.io does)
	// This separates network issues from decompression issues
	contentEncoding := req.Header.Get("Content-Encoding")
	var compressedBody []byte
	func() {
		defer func() {
			if panicVal := recover(); panicVal != nil {
				r.settings.Logger.Error("Panic during network read",
					zap.Any("panic", panicVal),
					zap.String("content-encoding", contentEncoding))
				err = errors.New("failed to read from network")
			}
		}()
		compressedBody, err = io.ReadAll(req.Body)
	}()

	if err != nil {
		r.settings.Logger.Error("Failed to read request body from network",
			zap.Error(err),
			zap.String("content-encoding", contentEncoding),
			zap.String("content-type", req.Header.Get("Content-Type")),
			zap.Int64("content-length", req.ContentLength),
			zap.String("endpoint", req.RequestURI),
			zap.String("api-key-masked", maskedKey))
		writeLibhoneyError(resp, enc, "failed to read request body")
		return
	}

	// Now decompress from the complete buffered data
	bodyReader, err := decompressBody(io.NopCloser(bytes.NewReader(compressedBody)), contentEncoding)
	if err != nil {
		r.settings.Logger.Error("Failed to decompress request body",
			zap.Error(err),
			zap.String("content-encoding", contentEncoding),
			zap.String("content-type", req.Header.Get("Content-Type")),
			zap.Int("compressed-size", len(compressedBody)),
			zap.String("endpoint", req.RequestURI),
			zap.String("api-key-masked", maskedKey))
		writeLibhoneyError(resp, enc, "failed to decompress request body")
		return
	}
	defer func() {
		if bodyReader != nil {
			_ = bodyReader.Close()
		}
	}()

	var body []byte
	func() {
		defer func() {
			if panicVal := recover(); panicVal != nil {
				// Log the panic but don't expose internal details to the client
				r.settings.Logger.Error("Panic during decompression (likely malformed compressed data)",
					zap.Any("panic", panicVal),
					zap.String("content-encoding", contentEncoding),
					zap.String("content-type", req.Header.Get("Content-Type")),
					zap.Int("compressed-size", len(compressedBody)),
					zap.String("endpoint", req.RequestURI),
					zap.String("api-key-masked", maskedKey))
				err = errors.New("failed to decompress: panic during decompression")
			}
		}()
		body, err = io.ReadAll(bodyReader)
	}()

	if err != nil {
		r.settings.Logger.Error("Failed to decompress buffered body",
			zap.Error(err),
			zap.String("content-encoding", contentEncoding),
			zap.String("content-type", req.Header.Get("Content-Type")),
			zap.Int("compressed-size", len(compressedBody)),
			zap.String("endpoint", req.RequestURI),
			zap.String("api-key-masked", maskedKey))
		writeLibhoneyError(resp, enc, "failed to decompress request body")
		return
	}
	func() {
		defer func() {
			if panicVal := recover(); panicVal != nil {
				r.settings.Logger.Error("Panic during request body close",
					zap.Any("panic", panicVal))
				writeLibhoneyError(resp, enc, "failed to close request body")
				err = errors.New("panic during body close")
			}
		}()
		err = req.Body.Close()
	}()
	if err != nil {
		if !strings.Contains(err.Error(), "panic during body close") {
			r.settings.Logger.Error("Failed to close request body", zap.Error(err))
			writeLibhoneyError(resp, enc, "failed to close request body")
		}
		return
	}
	// Decode libhoney events from request body
	libhoneyevents, err := codec.DecodeEvents(req.Header.Get("Content-Type"), body, req.Header)
	if err != nil {
		r.settings.Logger.Error("Failed to decode events",
			zap.Error(err),
			zap.String("content-type", req.Header.Get("Content-Type")),
			zap.String("endpoint", req.RequestURI),
			zap.String("api-key-masked", maskedKey))
		writeLibhoneyError(resp, enc, fmt.Sprintf("failed to decode events: %v", err))
		return
	}

	// Log successful decoding
	if len(libhoneyevents) > 0 {
		if libhoneyevents[0].MsgPackTimestamp != nil {
			r.settings.Logger.Debug("Events decoded successfully",
				zap.Time("timestamp.first.msgpacktimestamp", *libhoneyevents[0].MsgPackTimestamp),
				zap.String("timestamp.first.time", libhoneyevents[0].Time),
				zap.String("content-type", req.Header.Get("Content-Type")))
		} else {
			r.settings.Logger.Debug("Events decoded successfully",
				zap.String("timestamp.first.time", libhoneyevents[0].Time),
				zap.String("content-type", req.Header.Get("Content-Type")))
		}
	}

	// Parse events and track which original indices contributed to each OTLP entity
	otlpLogs, otlpTraces, indexMapping, parsingResults := parser.ToPdata(dataset, libhoneyevents, r.cfg.FieldMapConfig, *r.settings.Logger)

	// Use the request context which already contains client metadata when IncludeMetadata is enabled
	ctx := req.Context()

	// Start with parsing results, then apply batch processing results for span events/links
	results := parsingResults
	hasFailures := false

	// Check if any parsing failures occurred
	for _, result := range results {
		if result.Status != 0 && result.Status != http.StatusAccepted {
			hasFailures = true
		}
	}

	// Process logs - only override parsing results if consumer fails
	numLogs := otlpLogs.LogRecordCount()
	if numLogs > 0 {
		if r.nextLogs != nil {
			ctx = r.obsreport.StartLogsOp(ctx)
			err = r.nextLogs.ConsumeLogs(ctx, otlpLogs)
			r.obsreport.EndLogsOp(ctx, "protobuf", numLogs, err)
			// Only override parsing results if consumer failed
			if err != nil {
				applyConsumerResultsToSuccessfulEvents(results, indexMapping.LogIndices, err)
				hasFailures = true
			}
		} else {
			dropErr := errors.New("no log consumer configured")
			r.settings.Logger.Warn("Dropping log records - no log consumer configured", zap.Int("dropped_logs", numLogs))
			r.obsreport.EndLogsOp(ctx, "protobuf", numLogs, dropErr)
			// Override even successful parsing results since consumer is not configured
			applyConsumerResultsToSuccessfulEvents(results, indexMapping.LogIndices, dropErr)
			hasFailures = true
		}
	}

	// Process traces - only override parsing results if consumer fails
	numTraces := otlpTraces.SpanCount()
	if numTraces > 0 {
		if r.nextTraces != nil {
			ctx = r.obsreport.StartTracesOp(ctx)
			err = r.nextTraces.ConsumeTraces(ctx, otlpTraces)
			r.obsreport.EndTracesOp(ctx, "protobuf", numTraces, err)
			// Only override parsing results if consumer failed
			if err != nil {
				applyConsumerResultsToSuccessfulEvents(results, indexMapping.TraceIndices, err)
				hasFailures = true
			}
		} else {
			dropErr := errors.New("no trace consumer configured")
			r.settings.Logger.Warn("Dropping trace spans - no trace consumer configured", zap.Int("dropped_spans", numTraces))
			r.obsreport.EndTracesOp(ctx, "protobuf", numTraces, dropErr)
			// Override even successful parsing results since consumer is not configured
			applyConsumerResultsToSuccessfulEvents(results, indexMapping.TraceIndices, dropErr)
			hasFailures = true
		}
	}

	if err != nil {
		errorutil.HTTPError(resp, err)
		return
	}

	// Write response
	if hasFailures {
		writePartialResponse(resp, enc, results)
	} else {
		writeSuccessResponse(resp, enc, len(libhoneyevents))
	}
}

func readContentType(resp http.ResponseWriter, req *http.Request) (codec.Encoder, bool) {
	if req.Method != http.MethodPost {
		handleUnmatchedMethod(resp)
		return nil, false
	}

	contentType := getMimeTypeFromContentType(req.Header.Get("Content-Type"))
	encoder, err := codec.GetEncoder(contentType)
	if err != nil {
		handleUnmatchedContentType(resp)
		return nil, false
	}
	return encoder, true
}

func writeResponse(w http.ResponseWriter, contentType string, statusCode int, msg []byte) {
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(statusCode)
	_, _ = w.Write(msg)
}

func getMimeTypeFromContentType(contentType string) string {
	mediatype, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return ""
	}
	return mediatype
}

func handleUnmatchedMethod(resp http.ResponseWriter) {
	status := http.StatusMethodNotAllowed
	writeResponse(resp, "text/plain", status, fmt.Appendf(nil, "%v method not allowed, supported: [POST]", status))
}

func handleUnmatchedContentType(resp http.ResponseWriter) {
	status := http.StatusUnsupportedMediaType
	writeResponse(resp, "text/plain", status, fmt.Appendf(nil, "%v unsupported media type, supported: [%s, application/x-msgpack]", status, codec.JSONContentType))
}

// applyConsumerResultsToSuccessfulEvents applies consumer results only to events that succeeded parsing
func applyConsumerResultsToSuccessfulEvents(results []response.ResponseInBatch, indices []int, err error) {
	for _, idx := range indices {
		// Only override if the event was successfully parsed (status == 202)
		if results[idx].Status == http.StatusAccepted {
			if err != nil {
				results[idx] = response.ResponseInBatch{
					Status:   http.StatusServiceUnavailable,
					ErrorStr: err.Error(),
				}
			}
			// If consumer succeeded, keep the existing success status
		}
	}
}

// writeSuccessResponse writes a success response for all events in the batch
func writeSuccessResponse(resp http.ResponseWriter, enc codec.Encoder, numEvents int) {
	successResponse := response.MakeSuccessResponse(numEvents)
	writeLibhoneyResponse(resp, enc, http.StatusOK, successResponse)
}

// writePartialResponse writes a response for mixed success/failure results
func writePartialResponse(resp http.ResponseWriter, enc codec.Encoder, results []response.ResponseInBatch) {
	writeLibhoneyResponse(resp, enc, http.StatusOK, results)
}

// writeLibhoneyResponse marshals and writes a libhoney-format response
func writeLibhoneyResponse(resp http.ResponseWriter, enc codec.Encoder, statusCode int, batchResponse []response.ResponseInBatch) {
	responseBody, err := enc.MarshalResponse(batchResponse)
	if err != nil {
		// Fallback to generic error if we can't marshal the response
		errorutil.HTTPError(resp, err)
		return
	}
	writeResponse(resp, enc.ContentType(), statusCode, responseBody)
}
