// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mezmoexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/mezmoexporter"

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
)

type mezmoExporter struct {
	config          *Config
	settings        component.TelemetrySettings
	client          *http.Client
	userAgentString string
	log             *zap.Logger
	wg              sync.WaitGroup
}

type mezmoLogLine struct {
	Timestamp int64             `json:"timestamp"`
	Line      string            `json:"line"`
	App       string            `json:"app"`
	Level     string            `json:"level"`
	Meta      map[string]string `json:"meta"`
}

type mezmoLogBody struct {
	Lines []mezmoLogLine `json:"lines"`
}

func newLogsExporter(config *Config, settings component.TelemetrySettings, buildInfo component.BuildInfo, logger *zap.Logger) *mezmoExporter {
	e := &mezmoExporter{
		config:          config,
		settings:        settings,
		userAgentString: fmt.Sprintf("mezmo-otel-exporter/%s", buildInfo.Version),
		log:             logger,
	}
	return e
}

func (m *mezmoExporter) pushLogData(_ context.Context, ld plog.Logs) error {
	m.wg.Add(1)
	defer m.wg.Done()

	return m.logDataToMezmo(ld)
}

func (m *mezmoExporter) start(ctx context.Context, host component.Host) (err error) {
	m.client, err = m.config.ClientConfig.ToClient(ctx, host, m.settings)
	return err
}

func (m *mezmoExporter) stop(context.Context) (err error) {
	if m.client == nil {
		return nil
	}
	m.wg.Wait()
	m.client.CloseIdleConnections()
	m.client = nil
	return nil
}

func (m *mezmoExporter) logDataToMezmo(ld plog.Logs) error {
	var errs error

	var lines []mezmoLogLine

	// Convert the log resources to mezmo lines...
	resourceLogs := ld.ResourceLogs()
	for i := 0; i < resourceLogs.Len(); i++ {
		resource := resourceLogs.At(i).Resource()
		resourceHostName, hasResourceHostName := resource.Attributes().Get("host.name")
		scopeLogs := resourceLogs.At(i).ScopeLogs()

		for j := 0; j < scopeLogs.Len(); j++ {
			logs := scopeLogs.At(j).LogRecords()

			for k := 0; k < logs.Len(); k++ {
				log := logs.At(k)

				// Convert Attributes to meta fields being mindful of the maxMetaDataSize restriction
				attrs := map[string]string{}
				if hasResourceHostName {
					attrs["hostname"] = resourceHostName.AsString()
				}

				if traceID := log.TraceID(); !traceID.IsEmpty() {
					attrs["trace.id"] = hex.EncodeToString(traceID[:])
				}

				if spanID := log.SpanID(); !spanID.IsEmpty() {
					attrs["span.id"] = hex.EncodeToString(spanID[:])
				}

				for k, v := range log.Attributes().All() {
					attrs[k] = truncateString(v.Str(), maxMetaDataSize)
				}

				s, _ := log.Attributes().Get("appname")
				app := s.Str()

				tstamp := log.Timestamp().AsTime().UTC().UnixMilli()
				if tstamp == 0 {
					tstamp = time.Now().UTC().UnixMilli()
				}

				logLevel := truncateString(log.SeverityText(), maxLogLevelLen)
				if logLevel == "" {
					logLevel = "info"
				}

				line := mezmoLogLine{
					Timestamp: tstamp,
					Line:      truncateString(log.Body().Str(), maxMessageSize),
					App:       truncateString(app, maxAppnameLen),
					Level:     logLevel,
					Meta:      attrs,
				}
				lines = append(lines, line)
			}
		}
	}

	// Send them to Mezmo in batches < 10MB in size
	var b strings.Builder
	b.WriteString("{\"lines\": [")

	var lineBytes []byte
	for i, line := range lines {
		if lineBytes, errs = json.Marshal(line); errs != nil {
			return fmt.Errorf("error Creating JSON payload: %w", errs)
		}

		newBufSize := b.Len() + len(lineBytes)
		if newBufSize >= maxBodySize-2 {
			str := b.String()
			str = str[:len(str)-1] + "]}"
			if errs = m.sendLinesToMezmo(str); errs != nil {
				return errs
			}
			b.Reset()
			b.WriteString("{\"lines\": [")
		}

		b.Write(lineBytes)

		if i < len(lines)-1 {
			b.WriteRune(',')
		}
	}

	return m.sendLinesToMezmo(b.String() + "]}")
}

func (m *mezmoExporter) sendLinesToMezmo(post string) (errs error) {
	req, _ := http.NewRequest(http.MethodPost, m.config.IngestURL, strings.NewReader(post))
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("User-Agent", m.userAgentString)
	req.Header.Add("apikey", string(m.config.IngestKey))

	var res *http.Response
	if res, errs = m.client.Do(req); errs != nil {
		return fmt.Errorf("failed to POST log to Mezmo: %w", errs)
	}
	if res.StatusCode >= 400 {
		m.log.Error(fmt.Sprintf("got http status (%s): %s", req.URL.Path, res.Status))
		if checkLevel := m.log.Check(zap.DebugLevel, "http response"); checkLevel != nil {
			responseBody, _ := io.ReadAll(res.Body)
			checkLevel.Write(zap.String("response", string(responseBody)))
		}
	}

	return res.Body.Close()
}
