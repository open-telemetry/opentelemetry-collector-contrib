// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cloudflarereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/cloudflarereceiver"

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	rcvr "go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"
)

type logsReceiver struct {
	logger   *zap.Logger
	cfg      *Config
	server   *http.Server
	consumer consumer.Logs
	wg       *sync.WaitGroup
	doneChan chan bool
	id       component.ID // ID of the receiver component
}

const (
	secretHeaderName = "X-CF-Secret"
)

func newLogsReceiver(params rcvr.CreateSettings, cfg *Config, consumer consumer.Logs) (*logsReceiver, error) {
	recv := &logsReceiver{
		cfg:      cfg,
		consumer: consumer,
		logger:   params.Logger,
		wg:       &sync.WaitGroup{},
		doneChan: make(chan bool),
		id:       params.ID,
	}

	tlsConfig, err := cfg.TLS.LoadTLSConfig()
	if err != nil {
		return nil, err
	}

	s := &http.Server{
		TLSConfig: tlsConfig,
		Handler:   http.HandlerFunc(recv.handleRequest),
	}

	recv.server = s
	return recv, nil
}

func (l *logsReceiver) Start(ctx context.Context, host component.Host) error {
	go l.startListening(ctx, host)
	return nil
}

func (l *logsReceiver) Shutdown(ctx context.Context) error {
	l.logger.Debug("Shutting down server")
	err := l.server.Shutdown(ctx)
	if err != nil {
		return err
	}

	l.logger.Debug("Waiting for shutdown to complete.")
	l.wg.Wait()

	l.logger.Debug("shutting down logs receiver")
	close(l.doneChan)
	return nil
}

func (l *logsReceiver) startListening(ctx context.Context, host component.Host) error {
	l.logger.Debug("starting receiver HTTP server")
	// We use l.server.Serve* over l.server.ListenAndServe*
	// So that we can catch and return errors relating to binding to network interface on start.
	var lc net.ListenConfig

	listener, err := lc.Listen(ctx, "tcp", l.cfg.Endpoint)
	if err != nil {
		return err
	}

	l.wg.Add(1)
	go func() {
		defer l.wg.Done()

		l.logger.Debug("Starting ServeTLS",
			zap.String("address", l.cfg.Endpoint),
			zap.String("certfile", l.cfg.TLS.CertFile),
			zap.String("keyfile", l.cfg.TLS.KeyFile))

		err := l.server.ServeTLS(listener, l.cfg.TLS.CertFile, l.cfg.TLS.KeyFile)

		l.logger.Debug("Serve TLS done")

		if err != http.ErrServerClosed {
			l.logger.Error("ServeTLS failed", zap.Error(err))
			host.ReportFatalError(err)
		}
	}()
	return nil
}

func (l *logsReceiver) handleRequest(rw http.ResponseWriter, req *http.Request) {
	if len(l.cfg.Secret) > 0 {
		secretHeader := req.Header.Get(secretHeaderName)
		if secretHeader == "" {
			rw.WriteHeader(http.StatusUnauthorized)
			l.logger.Debug("Got payload with no Secret when it was specified in config, dropping...")
			return
		} else if secretHeader != l.cfg.Secret {
			rw.WriteHeader(http.StatusUnauthorized)
			l.logger.Debug("Got payload with invalid Secret, dropping...")
			return
		}
	}

	var payload []byte
	var err error
	if req.Header.Get("Content-Encoding") == "gzip" {
		reader, err := gzip.NewReader(req.Body)
		if err != nil {
			rw.WriteHeader(http.StatusUnprocessableEntity)
			l.logger.Debug("Got payload with gzip, but failed to read", zap.Error(err))
			return
		}
		defer reader.Close()
		// Read the decompressed response body
		payload, err = io.ReadAll(reader)
		if err != nil {
			rw.WriteHeader(http.StatusUnprocessableEntity)
			l.logger.Debug("Got payload with gzip, but failed to read", zap.Error(err))
			return
		}
	} else {
		payload, err = io.ReadAll(req.Body)
		if err != nil {
			rw.WriteHeader(http.StatusUnprocessableEntity)
			l.logger.Debug("Failed to read alerts payload", zap.Error(err), zap.String("remote", req.RemoteAddr))
			return
		}
	}

	strPayload := string(payload)

	if strPayload == "test" {
		l.logger.Info("Received test request from Cloudflare")
		rw.WriteHeader(http.StatusOK)
		return
	}

	logs, err := parsePayload(strPayload)
	if err != nil {
		rw.WriteHeader(http.StatusUnprocessableEntity)
		l.logger.Error("Failed to convert cloudflare request payload to maps", zap.Error(err))
		return
	}

	if err := l.consumer.ConsumeLogs(req.Context(), l.processLogs(pcommon.NewTimestampFromTime(time.Now()), logs)); err != nil {
		rw.WriteHeader(http.StatusInternalServerError)
		l.logger.Error("Failed to consumer alert as log", zap.Error(err))
		return
	}

	rw.WriteHeader(http.StatusOK)
}

func parsePayload(payload string) ([]map[string]interface{}, error) {
	var logs []map[string]interface{}
	for _, line := range strings.Split(payload, "\n") {
		if len(line) == 0 {
			continue
		}
		var log map[string]interface{}
		err := json.Unmarshal([]byte(line), &log)
		if err != nil {
			return logs, err
		}
		logs = append(logs, log)
	}
	return logs, nil
}

func (l *logsReceiver) processLogs(now pcommon.Timestamp, logs []map[string]interface{}) plog.Logs {
	pLogs := plog.NewLogs()

	resourceLogs := pLogs.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords()
	for _, log := range logs {
		logRecord := resourceLogs.AppendEmpty()
		logRecord.SetObservedTimestamp(now)

		if v, ok := log[l.cfg.TimestampField]; ok {
			if stringV, ok := v.(string); ok {
				ts, err := time.Parse(time.RFC3339, stringV)
				if err != nil {
					l.logger.Warn(fmt.Sprintf("unable to parse %s", l.cfg.TimestampField), zap.Error(err), zap.String("value", stringV))
				} else {
					logRecord.SetTimestamp(pcommon.NewTimestampFromTime(ts))
				}
			} else {
				l.logger.Warn(fmt.Sprintf("unable to parse %s", l.cfg.TimestampField), zap.Any("value", v))
			}
		}

		if v, ok := log["EdgeResponseStatus"]; ok {
			sev := plog.SeverityNumberUnspecified
			switch v := v.(type) {
			case string:
				intV, err := strconv.ParseInt(v, 10, 64)
				if err != nil {
					l.logger.Warn("unable to parse EdgeResponseStatus", zap.Error(err), zap.String("value", v))
				} else {
					sev = severityFromStatusCode(intV)
				}
			case int64:
				sev = severityFromStatusCode(v)
			case float64:
				sev = severityFromStatusCode(int64(v))
			}
			if sev != plog.SeverityNumberUnspecified {
				logRecord.SetSeverityNumber(sev)
				logRecord.SetSeverityText(sev.String())
			}
		}

		attrs := logRecord.Attributes()
		for field, attribute := range l.cfg.FieldAttributeMap {
			if v, ok := log[field]; ok {
				switch v := v.(type) {
				case string:
					attrs.PutStr(attribute, v)
				case int:
					attrs.PutInt(attribute, int64(v))
				case int64:
					attrs.PutInt(attribute, v)
				case float64:
					attrs.PutDouble(attribute, v)
				case bool:
					attrs.PutBool(attribute, v)
				default:
					l.logger.Warn("unable to translate field to attribute, unsupported type", zap.String("field", field), zap.Any("value", v), zap.String("type", fmt.Sprintf("%T", v)))
				}
			}
		}

		logRecord.Body().SetEmptyMap().FromRaw(log)
	}

	return pLogs
}

// severityFromStatusCode translates HTTP status code to OpenTelemetry severity number.
func severityFromStatusCode(statusCode int64) plog.SeverityNumber {
	switch {
	case statusCode < 300:
		return plog.SeverityNumberInfo
	case statusCode < 400:
		return plog.SeverityNumberInfo2
	case statusCode < 500:
		return plog.SeverityNumberWarn
	case statusCode < 600:
		return plog.SeverityNumberError
	default:
		return plog.SeverityNumberUnspecified
	}
}
