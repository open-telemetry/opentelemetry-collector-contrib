// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadoglogreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadoglogreceiver"

import (
	"compress/gzip"
	"compress/zlib"
	"context"
	"errors"
	"fmt"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/obsreport"
	_ "go.opentelemetry.io/collector/pdata/plog/plogotlp"
	"go.opentelemetry.io/collector/receiver"
	"io"
	"net/http"
	"strings"
)

type datadogReceiver struct {
	address      string
	config       *Config
	params       receiver.CreateSettings
	nextConsumer consumer.Logs
	server       *http.Server
	tReceiver    *obsreport.Receiver
}

func newDataDogReceiver(config *Config, nextConsumer consumer.Logs, params receiver.CreateSettings) (receiver.Traces, error) {
	if nextConsumer == nil {
		return nil, component.ErrNilNextConsumer
	}

	instance, err := obsreport.NewReceiver(obsreport.ReceiverSettings{LongLivedCtx: false, ReceiverID: params.ID, Transport: "http", ReceiverCreateSettings: params})
	if err != nil {
		return nil, err
	}

	return &datadogReceiver{
		params:       params,
		config:       config,
		nextConsumer: nextConsumer,
		server: &http.Server{
			ReadTimeout: config.ReadTimeout,
		},
		tReceiver: instance,
	}, nil
}

func (ddr *datadogReceiver) Start(_ context.Context, host component.Host) error {
	ddmux := http.NewServeMux()
	//	ddmux.HandleFunc("/api/v0.2/traces", ddr.handleV2Traces)
	ddmux.HandleFunc("/api/v2/logs", ddr.handleLogs)
	var err error
	ddr.server, err = ddr.config.HTTPServerSettings.ToServer(
		host,
		ddr.params.TelemetrySettings,
		ddmux,
	)
	if err != nil {
		return fmt.Errorf("failed to create server definition: %w", err)
	}
	hln, err := ddr.config.HTTPServerSettings.ToListener()
	if err != nil {
		return fmt.Errorf("failed to create datadog listener: %w", err)
	}

	ddr.address = hln.Addr().String()

	go func() {
		if err := ddr.server.Serve(hln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			host.ReportFatalError(fmt.Errorf("error starting datadog receiver: %w", err))
		}
	}()
	return nil
}

func (ddr *datadogReceiver) Shutdown(ctx context.Context) (err error) {
	return ddr.server.Shutdown(ctx)
}

func readCloserFromRequest(req *http.Request) (io.ReadCloser, error) {
	rc := struct {
		io.Reader
		io.Closer
	}{
		Reader: req.Body,
		Closer: req.Body,
	}
	if req.Header.Get("Accept-Encoding") == "gzip" {
		gz, err := gzip.NewReader(req.Body)
		if err != nil {
			return nil, err
		}
		defer gz.Close()
		rc.Reader = gz
	}
	return rc, nil
}

func readAndCloseBody(resp http.ResponseWriter, req *http.Request) ([]byte, bool) {
	// Check if the request body is compressed
	var reader io.Reader = req.Body
	if strings.Contains(req.Header.Get("Content-Encoding"), "gzip") {
		// Decompress gzip
		gz, err := gzip.NewReader(req.Body)
		if err != nil {
			return nil, false
		}
		defer gz.Close()
		reader = gz
	} else if strings.Contains(req.Header.Get("Content-Encoding"), "deflate") {
		// Decompress deflate
		zlibReader, err := zlib.NewReader(req.Body)
		if err != nil {
			return nil, false
		}
		defer zlibReader.Close()
		reader = zlibReader
	}

	body, err := io.ReadAll(reader)
	if err != nil {
		return nil, false
	}
	if err = req.Body.Close(); err != nil {
		return nil, false
	}
	return body, true
}

type HTTPLogItem struct {
	// The integration name associated with your log: the technology from which the log originated.
	// When it matches an integration name, Datadog automatically installs the corresponding parsers and facets.
	// See [reserved attributes](https://docs.datadoghq.com/logs/log_configuration/attributes_naming_convention/#reserved-attributes).
	Ddsource string `json:"ddsource,omitempty"`
	// Tags associated with your logs.
	Ddtags string `json:"ddtags,omitempty"`
	// The name of the originating host of the log.
	Hostname string `json:"hostname,omitempty"`
	// The message [reserved attribute](https://docs.datadoghq.com/logs/log_configuration/attributes_naming_convention/#reserved-attributes)
	// of your log. By default, Datadog ingests the value of the message attribute as the body of the log entry.
	// That value is then highlighted and displayed in the Logstream, where it is indexed for full text search.
	Message string `json:"message"`
	// The name of the application or service generating the log events.
	// It is used to switch from Logs to APM, so make sure you define the same value when you use both products.
	// See [reserved attributes](https://docs.datadoghq.com/logs/log_configuration/attributes_naming_convention/#reserved-attributes).
	Service string `json:"service,omitempty"`
	// UnparsedObject contains the raw value of the object if there was an error when deserializing into the struct
	UnparsedObject       map[string]interface{} `json:"-"`
	AdditionalProperties map[string]string

	Status string `json:"status,omitempty"`

	Timestamp int64 `json:"timestamp,omitempty"`
}

func (ddr *datadogReceiver) handleLogs(w http.ResponseWriter, req *http.Request) {
	body, err := readAndCloseBody(w, req)
	if !err {
		http.Error(w, "Unable to unmarshal reqs", http.StatusBadRequest)
		ddr.params.Logger.Error("Unable to unmarshal reqs")
		return
	}
	v2Logs, err1 := handlePayload(body)
	if err1 != nil {
		http.Error(w, "Unable to unmarshal reqs", http.StatusBadRequest)
		return
	}
	obsCtx := ddr.tReceiver.StartLogsOp(req.Context())
	for _, log := range v2Logs {
		otelLog, err1 := toLogs(log, req)
		if err1 != nil {
			http.Error(w, "Logs consumer errored out", http.StatusInternalServerError)
			ddr.params.Logger.Error("Logs consumer errored out")
		}
		errs := ddr.nextConsumer.ConsumeLogs(obsCtx, otelLog.Logs())
		if errs != nil {
			http.Error(w, "Logs consumer errored out", http.StatusInternalServerError)
			ddr.params.Logger.Error("Logs consumer errored out")
		} else {
			_, _ = w.Write([]byte("OK"))
		}
	}
	_, _ = w.Write([]byte("OK"))
}
