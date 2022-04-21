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
	client, err := m.config.HTTPClientSettings.ToClient(host.GetExtensions(), m.settings)
	if err != nil {
		return err
	}

	m.client = client

	return nil
}

func (m *mezmoExporter) stop(context.Context) (err error) {
	m.wg.Wait()
	return nil
}

func (m *mezmoExporter) logDataToMezmo(ld plog.Logs) error {
	var errs error

	var hostname = "fixme.com"
	var lines []MezmoLogLine

	// Batch up the log lines
	resourceLogs := ld.ResourceLogs()
	for i := 0; i < resourceLogs.Len(); i++ {
		ills := resourceLogs.At(i).ScopeLogs()
		//resource := resourceLogs.At(i).Resource()
		for j := 0; j < ills.Len(); j++ {
			logs := ills.At(j).LogRecords()

			for k := 0; k < logs.Len(); k++ {
				var log = logs.At(k)

				//fmt.Printf("Body:            [%s]\n", log.Body().StringVal())
				//fmt.Printf("Severity Number: [%v]\n", log.SeverityNumber().String())
				//fmt.Printf("Severity Text:   [%v]\n", log.SeverityText())
				//fmt.Printf("Timestamp:       [%v]\n", log.Timestamp())
				//fmt.Printf("Trace ID:        [%v]\n", log.TraceID())
				//fmt.Printf("Span ID:	        [%v]\n", log.SpanID())
				//fmt.Printf("Log Attributes:\n")

				var attrs = map[string]string{}

				attrs["trace.id"] = log.TraceID().HexString()
				attrs["span.id"] = log.SpanID().HexString()
				log.Attributes().Range(func(k string, v pcommon.Value) bool {
					attrs[k] = v.StringVal()
					//fmt.Printf("\t%s: [%s]\n", k, attrs[k])
					return true
				})

				var s pcommon.Value
				s, _ = log.Attributes().Get("appname")
				var app = s.StringVal()

				var line = MezmoLogLine{
					Timestamp: log.Timestamp().AsTime().UTC().UnixMilli(),
					Line:      log.Body().StringVal(),
					App:       app,
					Level:     log.SeverityNumber().String(),
					Meta:      attrs,
				}
				lines = append(lines, line)
			}
		}
	}

	// Send them to Mezmo
	var mezmoLogBody = MezmoLogBody{
		Lines: lines,
	}

	var postBody []byte
	if postBody, errs = json.Marshal(mezmoLogBody); errs != nil {
		return fmt.Errorf("error Creating JSON payload: %s", errs)
	}

	url := fmt.Sprintf("%s?hostname=%s", m.config.Endpoint, hostname)

	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(postBody))
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("apikey", m.config.IngestionKey)

	var res *http.Response
	if res, errs = http.DefaultClient.Do(req); errs != nil {
		return fmt.Errorf("failed to POST log to Mezmo: %s", errs)
	}

	return res.Body.Close()
}
