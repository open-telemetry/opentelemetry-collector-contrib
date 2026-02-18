// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cloudflarereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/cloudflarereceiver"

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	rcvr "go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/errorutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/cloudflarereceiver/internal/metadata"
)

type logsReceiver struct {
	logger   *zap.Logger
	cfg      *LogsConfig
	server   *http.Server
	consumer consumer.Logs
	wg       *sync.WaitGroup
	id       component.ID // ID of the receiver component
	obsrecv  *receiverhelper.ObsReport
}

const secretHeaderName = "X-CF-Secret"

func newLogsReceiver(params rcvr.Settings, cfg *Config, consumer consumer.Logs) (*logsReceiver, error) {
	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{
		ReceiverID:             params.ID,
		Transport:              "http",
		ReceiverCreateSettings: params,
	})
	if err != nil {
		return nil, err
	}

	recv := &logsReceiver{
		cfg:      &cfg.Logs,
		consumer: consumer,
		logger:   params.Logger,
		wg:       &sync.WaitGroup{},
		obsrecv:  obsrecv,
		id:       params.ID,
	}

	recv.server = &http.Server{
		Handler:           http.HandlerFunc(recv.handleRequest),
		ReadHeaderTimeout: 20 * time.Second,
	}

	if recv.cfg.TLS != nil {
		tlsConfig, err := recv.cfg.TLS.LoadTLSConfig(context.Background())
		if err != nil {
			return nil, err
		}

		recv.server.TLSConfig = tlsConfig
	}

	return recv, nil
}

func (l *logsReceiver) Start(ctx context.Context, host component.Host) error {
	return l.startListening(ctx, host)
}

func (l *logsReceiver) Shutdown(ctx context.Context) error {
	l.logger.Debug("Shutting down server")
	err := l.server.Shutdown(ctx)
	if err != nil {
		return err
	}

	l.logger.Debug("Waiting for shutdown to complete.")
	l.wg.Wait()
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

	l.wg.Go(func() {
		if l.cfg.TLS != nil {
			l.logger.Debug("Starting ServeTLS",
				zap.String("address", l.cfg.Endpoint),
				zap.String("certfile", l.cfg.TLS.CertFile),
				zap.String("keyfile", l.cfg.TLS.KeyFile))

			err := l.server.ServeTLS(listener, l.cfg.TLS.CertFile, l.cfg.TLS.KeyFile)

			l.logger.Debug("ServeTLS done")

			if !errors.Is(err, http.ErrServerClosed) {
				l.logger.Error("ServeTLS failed", zap.Error(err))
				componentstatus.ReportStatus(host, componentstatus.NewFatalErrorEvent(err))
			}
		} else {
			l.logger.Debug("Starting Serve",
				zap.String("address", l.cfg.Endpoint))

			err := l.server.Serve(listener)

			l.logger.Debug("Serve done")

			if !errors.Is(err, http.ErrServerClosed) {
				l.logger.Error("Serve failed", zap.Error(err))
				componentstatus.ReportStatus(host, componentstatus.NewFatalErrorEvent(err))
			}
		}
	})
	return nil
}

func (l *logsReceiver) handleRequest(rw http.ResponseWriter, req *http.Request) {
	if l.cfg.Secret != "" {
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
		var err error
		payload, err = io.ReadAll(req.Body)
		if err != nil {
			rw.WriteHeader(http.StatusUnprocessableEntity)
			l.logger.Debug("Failed to read alerts payload", zap.Error(err), zap.String("remote", req.RemoteAddr))
			return
		}
	}

	if string(payload) == "test" {
		l.logger.Info("Received test request from Cloudflare")
		rw.WriteHeader(http.StatusOK)
		return
	}

	logs, err := parsePayload(payload)
	if err != nil {
		rw.WriteHeader(http.StatusUnprocessableEntity)
		l.logger.Error("Failed to convert cloudflare request payload to maps", zap.Error(err))
		return
	}

	pLogs := l.processLogs(pcommon.NewTimestampFromTime(time.Now()), logs)
	obsCtx := l.obsrecv.StartLogsOp(req.Context())
	if err := l.consumer.ConsumeLogs(obsCtx, pLogs); err != nil {
		l.obsrecv.EndLogsOp(obsCtx, metadata.Type.String(), pLogs.LogRecordCount(), err)
		errorutil.HTTPError(rw, err)
		l.logger.Error("Failed to consumer alert as log", zap.Error(err))
		return
	}

	l.obsrecv.EndLogsOp(obsCtx, metadata.Type.String(), pLogs.LogRecordCount(), nil)
	rw.WriteHeader(http.StatusOK)
}

func parsePayload(payload []byte) ([]map[string]any, error) {
	lines := bytes.Split(payload, []byte("\n"))
	logs := make([]map[string]any, 0, len(lines))
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		var log map[string]any
		err := json.Unmarshal(line, &log)
		if err != nil {
			return logs, err
		}
		logs = append(logs, log)
	}
	return logs, nil
}

func (l *logsReceiver) processLogs(now pcommon.Timestamp, logs []map[string]any) plog.Logs {
	pLogs := plog.NewLogs()

	// Group logs by ZoneName field if it was configured so it can be used as a resource attribute
	groupedLogs := make(map[string][]map[string]any)
	for _, log := range logs {
		zone := ""
		if v, ok := log["ZoneName"]; ok {
			if stringV, ok := v.(string); ok {
				zone = stringV
			}
		}
		groupedLogs[zone] = append(groupedLogs[zone], log)
	}

	for zone, logGroup := range groupedLogs {
		resourceLogs := pLogs.ResourceLogs().AppendEmpty()
		if zone != "" {
			resource := resourceLogs.Resource()
			resource.Attributes().PutStr("cloudflare.zone", zone)
		}
		scopeLogs := resourceLogs.ScopeLogs().AppendEmpty()
		scopeLogs.Scope().SetName(metadata.ScopeName)

		for _, log := range logGroup {
			logRecord := scopeLogs.LogRecords().AppendEmpty()
			logRecord.SetObservedTimestamp(now)

			if v, ok := log[l.cfg.TimestampField]; ok {
				switch l.cfg.TimestampFormat {
				case "unix":
					var sec int64
					switch val := v.(type) {
					case int:
						sec = int64(val)
					case int64:
						sec = val
					case float64:
						sec = int64(val)
					case string:
						i, err := strconv.ParseInt(val, 10, 64)
						if err != nil {
							l.logger.Warn("unable to parse "+l.cfg.TimestampField+" as unix seconds", zap.Error(err), zap.String("value", val))
							continue
						}
						sec = i
					default:
						l.logger.Warn("unable to parse "+l.cfg.TimestampField, zap.String("unsupported type", fmt.Sprintf("%T", v)))
						continue
					}
					logRecord.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(sec, 0)))
				case "unixnano":
					var nano int64
					switch val := v.(type) {
					case int:
						nano = int64(val)
					case int64:
						nano = val
					case float64:
						nano = int64(val)
					case string:
						i, err := strconv.ParseInt(val, 10, 64)
						if err != nil {
							l.logger.Warn("unable to parse "+l.cfg.TimestampField+" as unixnano", zap.Error(err), zap.String("value", val))
							continue
						}
						nano = i
					default:
						l.logger.Warn("unable to parse "+l.cfg.TimestampField, zap.String("unsupported type", fmt.Sprintf("%T", v)))
						continue
					}
					logRecord.SetTimestamp(pcommon.NewTimestampFromTime(time.Unix(0, nano)))
				case "rfc3339":
					strVal, ok := v.(string)
					if !ok {
						l.logger.Warn("unable to parse "+l.cfg.TimestampField+" as rfc3339, not a string", zap.Any("value", v), zap.String("type", fmt.Sprintf("%T", v)))
						continue
					}
					ts, err := time.Parse(time.RFC3339, strVal)
					if err != nil {
						l.logger.Warn("unable to parse "+l.cfg.TimestampField+" as rfc3339", zap.Error(err), zap.String("value", strVal))
						continue
					}
					logRecord.SetTimestamp(pcommon.NewTimestampFromTime(ts))
				default:
					l.logger.Warn("unknown timestamp_format configuration", zap.String("timestamp_format", l.cfg.TimestampFormat))
				}
			} else {
				l.logger.Warn("unable to parse "+l.cfg.TimestampField, zap.Any("value", v))
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
			for field, v := range log {
				attrName := field
				if len(l.cfg.Attributes) != 0 {
					// Only process fields that are in the config mapping
					mappedAttr, ok := l.cfg.Attributes[field]
					if !ok {
						// Skip fields not in mapping when we have a config
						continue
					}
					attrName = mappedAttr
				}
				// else if l.cfg.Attributes is empty, default to processing all fields with no renaming

				switch v := v.(type) {
				case string:
					attrs.PutStr(attrName, v)
				case int:
					attrs.PutInt(attrName, int64(v))
				case int64:
					attrs.PutInt(attrName, v)
				case float64:
					attrs.PutDouble(attrName, v)
				case bool:
					attrs.PutBool(attrName, v)
				case map[string]any:
					// Flatten the map and add each field with a prefixed key
					flattened := make(map[string]any)
					flattenMap(v, attrName+l.cfg.Separator, l.cfg.Separator, flattened)
					for k, val := range flattened {
						switch v := val.(type) {
						case string:
							attrs.PutStr(k, v)
						case int:
							attrs.PutInt(k, int64(v))
						case int64:
							attrs.PutInt(k, v)
						case float64:
							attrs.PutDouble(k, v)
						case bool:
							attrs.PutBool(k, v)
						default:
							l.logger.Warn("unable to translate flattened field to attribute, unsupported type",
								zap.String("field", k),
								zap.Any("value", v),
								zap.String("type", fmt.Sprintf("%T", v)))
						}
					}
				default:
					l.logger.Warn("unable to translate field to attribute, unsupported type",
						zap.String("field", field),
						zap.Any("value", v),
						zap.String("type", fmt.Sprintf("%T", v)))
				}
			}

			err := logRecord.Body().SetEmptyMap().FromRaw(log)
			if err != nil {
				l.logger.Warn("unable to set body", zap.Error(err))
			}
		}
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

// flattenMap recursively flattens a map[string]any into a single level map
// with keys joined by the specified separator
func flattenMap(input map[string]any, prefix, separator string, result map[string]any) {
	for k, v := range input {
		// Replace hyphens with underscores in the key. Content-Type becomes Content_Type
		k = strings.ReplaceAll(k, "-", "_")
		newKey := prefix + k
		switch val := v.(type) {
		case map[string]any:
			// Recursively flatten nested maps
			flattenMap(val, newKey+separator, separator, result)
		default:
			result[newKey] = v
		}
	}
}
