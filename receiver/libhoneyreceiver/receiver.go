// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package libhoneyreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/libhoneyreceiver"

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"

	"github.com/vmihailenco/msgpack/v5"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/errorutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/libhoneyreceiver/internal/simplespan"
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

type TeamInfo struct {
	Slug string `json:"slug"`
}

type EnvironmentInfo struct {
	Slug string `json:"slug"`
	Name string `json:"name"`
}

type AuthInfo struct {
	APIKeyAccess map[string]bool `json:"api_key_access"`
	Team         TeamInfo        `json:"team"`
	Environment  EnvironmentInfo `json:"environment"`
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
	if r.cfg.HTTP == nil {
		return nil
	}

	httpMux := http.NewServeMux()

	r.settings.Logger.Info("r.nextTraces is not null so httpTracesReciever was added", zap.Int("paths", len(r.cfg.HTTP.TracesURLPaths)))
	for _, path := range r.cfg.HTTP.TracesURLPaths {
		httpMux.HandleFunc(path, func(resp http.ResponseWriter, req *http.Request) {
			r.handleSomething(resp, req)
		})
		r.settings.Logger.Debug("Added path to HTTP server", zap.String("path", path))
	}

	if r.cfg.AuthAPI != "" {
		httpMux.HandleFunc("/1/auth", func(resp http.ResponseWriter, req *http.Request) {
			authURL := fmt.Sprintf("%s/1/auth", r.cfg.AuthAPI)
			authReq, err := http.NewRequest(http.MethodGet, authURL, nil)
			if err != nil {
				errJson, _ := json.Marshal(`{"error": "failed to create AuthInfo request"}`)
				writeResponse(resp, "json", http.StatusBadRequest, errJson)
				return
			}
			authReq.Header.Set("x-honeycomb-team", req.Header.Get("x-honeycomb-team"))
			var authClient http.Client
			authResp, err := authClient.Do(authReq)
			if err != nil {
				errJson, _ := json.Marshal(fmt.Sprintf(`"error": "failed to send request to auth api endpoint", "message", "%s"}`, err.Error()))
				writeResponse(resp, "json", http.StatusBadRequest, errJson)
				return
			}
			defer authResp.Body.Close()

			switch {
			case authResp.StatusCode == http.StatusUnauthorized:
				errJson, _ := json.Marshal(`"error": "received 401 response for AuthInfo request from Honeycomb API - check your API key"}`)
				writeResponse(resp, "json", http.StatusBadRequest, errJson)
				return
			case authResp.StatusCode > 299:
				errJson, _ := json.Marshal(fmt.Sprintf(`"error": "bad response code from API", "status_code", %d}`, authResp.StatusCode))
				writeResponse(resp, "json", http.StatusBadRequest, errJson)
				return
			}
			authRawBody, _ := io.ReadAll(authResp.Body)
			resp.Write(authRawBody)
		})
	}

	var err error
	if r.server, err = r.cfg.HTTP.ToServer(ctx, host, r.settings.TelemetrySettings, httpMux); err != nil {
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

		if err := r.server.Serve(hln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			componentstatus.ReportStatus(host, componentstatus.NewFatalErrorEvent(err))
		}
	}()
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

func (r *libhoneyReceiver) handleSomething(resp http.ResponseWriter, req *http.Request) {
	enc, ok := readContentType(resp, req)
	if !ok {
		return
	}

	dataset, err := getDatasetFromRequest(req.RequestURI)
	if err != nil {
		r.settings.Logger.Info("No dataset found in URL", zap.String("req.RequstURI", req.RequestURI))
	}

	for _, p := range r.cfg.HTTP.TracesURLPaths {
		dataset = strings.Replace(dataset, p, "", 1)
		r.settings.Logger.Debug("dataset parsed", zap.String("dataset.parsed", dataset))
	}

	body, err := io.ReadAll(req.Body)
	if err != nil {
		errorutil.HTTPError(resp, err)
	}
	if err = req.Body.Close(); err != nil {
		errorutil.HTTPError(resp, err)
	}

	simpleSpans := make([]simplespan.SimpleSpan, 0)
	switch req.Header.Get("Content-Type") {
	case "application/x-msgpack", "application/msgpack":
		decoder := msgpack.NewDecoder(bytes.NewReader(body))
		decoder.UseLooseInterfaceDecoding(true)
		decoder.Decode(&simpleSpans)
		if len(simpleSpans) > 0 {
			r.settings.Logger.Debug("Decoding with msgpack worked", zap.Time("timestamp.first.msgpacktimestamp", *simpleSpans[0].MsgPackTimestamp), zap.String("timestamp.first.time", simpleSpans[0].Time))
			r.settings.Logger.Debug("span zero", zap.String("span.data", simpleSpans[0].DebugString()))
		}
	case jsonContentType:
		err = json.Unmarshal(body, &simpleSpans)
		if err != nil {
			errorutil.HTTPError(resp, err)
		}
		if len(simpleSpans) > 0 {
			r.settings.Logger.Debug("Decoding with json worked", zap.Time("timestamp.first.msgpacktimestamp", *simpleSpans[0].MsgPackTimestamp), zap.String("timestamp.first.time", simpleSpans[0].Time))
		}
	}

	otlpLogs, err := toPsomething(dataset, simpleSpans, *r.cfg, *r.settings.Logger)
	if err != nil {
		errorutil.HTTPError(resp, err)
		return
	}

	numLogs := otlpLogs.LogRecordCount()
	if numLogs > 0 {
		ctx := r.obsreport.StartLogsOp(context.Background())
		err = r.nextLogs.ConsumeLogs(ctx, otlpLogs)
		r.obsreport.EndLogsOp(ctx, "protobuf", numLogs, err)
	}

	if err != nil {
		errorutil.HTTPError(resp, err)
		return
	}

	noErrors := []byte(`{"errors":[]}`)
	writeResponse(resp, enc.contentType(), http.StatusAccepted, noErrors)
}
