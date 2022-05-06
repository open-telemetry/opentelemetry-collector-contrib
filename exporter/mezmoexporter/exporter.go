// Copyright  The OpenTelemetry Authors
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

package mezmoexporter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

type mezmoExporter struct {
	config   *Config
	settings component.TelemetrySettings
	client   *http.Client
	wg       sync.WaitGroup
}

type MezmoLogLine struct {
	Timestamp int64             `json:"timestamp"`
	Line      string            `json:"line"`
	App       string            `json:"app"`
	Level     string            `json:"level"`
	Meta      map[string]string `json:"meta"`
}

type MezmoLogBody struct {
	Lines []MezmoLogLine `json:"lines"`
}

func newLogsExporter(config *Config, settings component.TelemetrySettings) *mezmoExporter {
	var e = &mezmoExporter{
		config:   config,
		settings: settings,
	}
	return e
}

func (m *mezmoExporter) pushLogData(ctx context.Context, ld plog.Logs) error {
	m.wg.Add(1)
	defer m.wg.Done()

	return m.logDataToMezmo(ld)
}

func (m *mezmoExporter) start(_ context.Context, host component.Host) (err error) {
	m.client, err := m.config.HTTPClientSettings.ToClient(host.GetExtensions(), m.settings)
	return err
}

func (m *mezmoExporter) stop(context.Context) (err error) {
	m.wg.Wait()
	return nil
}

func (m *mezmoExporter) logDataToMezmo(ld plog.Logs) error {
	var errs error

	var lines []MezmoLogLine

	// Convert the log resources to mezmo lines...
	resourceLogs := ld.ResourceLogs()
	for i := 0; i < resourceLogs.Len(); i++ {
		ills := resourceLogs.At(i).ScopeLogs()
		//resource := resourceLogs.At(i).Resource()
		for j := 0; j < ills.Len(); j++ {
			logs := ills.At(j).LogRecords()

			for k := 0; k < logs.Len(); k++ {
				log := logs.At(k)

				// Convert Attributes to meta fields being mindful of the maxMetaDataSize restriction
				attrs := map[string]string{}
				attrs["trace.id"] = log.TraceID().HexString()
				attrs["span.id"] = log.SpanID().HexString()
				log.Attributes().Range(func(k string, v pcommon.Value) bool {
					attrs[k] = truncateString(v.StringVal(), maxMetaDataSize)
					return true
				})

				var s pcommon.Value
				s, _ = log.Attributes().Get("appname")
				app := s.StringVal()

				line := MezmoLogLine{
					Timestamp: log.Timestamp().AsTime().UTC().UnixMilli(),
					Line:      truncateString(log.Body().StringVal(), maxMessageSize),
					App:       truncateString(app, maxAppnameLen),
					Level:     truncateString(log.SeverityText(), maxLogLevelLen),
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
			return fmt.Errorf("error Creating JSON payload: %s", errs)
		}

		var newBufSize = b.Len() + len(lineBytes)
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

	str := b.String() + "]}"
	if errs = m.sendLinesToMezmo(str); errs != nil {
		return errs
	}

	return nil
}

func (m *mezmoExporter) sendLinesToMezmo(post string) (errs error) {
	var hostname = "otel"

	url := fmt.Sprintf("%s?hostname=%s", m.config.IngestURL, hostname)

	req, _ := http.NewRequest("POST", url, bytes.NewBuffer([]byte(post)))
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("apikey", m.config.IngestKey)

	var res *http.Response
	if res, errs = http.DefaultClient.Do(req); errs != nil {
		return fmt.Errorf("failed to POST log to Mezmo: %s", errs)
	}

	return res.Body.Close()
}
