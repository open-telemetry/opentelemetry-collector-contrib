// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package datasetexporter

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"github.com/jarcoal/httpmock"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/testdata"
	"github.com/scalyr/dataset-go/pkg/api/add_events"
	"github.com/scalyr/dataset-go/pkg/api/request"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"io"
	"net/http"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/maxatome/go-testdeep/helpers/tdsuite"
	"github.com/maxatome/go-testdeep/td"
)

const RetryBase = time.Second

type SuiteLogsE2EExporter struct{}

func (s *SuiteLogsE2EExporter) Setup(t *td.T) error {
	// block all HTTP requests
	httpmock.Activate()
	return nil
}

func (s *SuiteLogsE2EExporter) PreTest(t *td.T, testName string) error {
	os.Clearenv()
	return nil
}

func (s *SuiteLogsE2EExporter) PostTest(t *td.T, testName string) error {
	os.Clearenv()
	httpmock.Reset()
	return nil
}

func (s *SuiteLogsE2EExporter) Destroy(t *td.T) error {
	os.Clearenv()
	httpmock.DeactivateAndReset()
	return nil
}

func TestSuiteLogsE2EExporter(t *testing.T) {
	td.NewT(t)
	tdsuite.Run(t, &SuiteLogsE2EExporter{})
}

func extract(req *http.Request) (add_events.AddEventsRequest, error) {
	data, _ := io.ReadAll(req.Body)
	b := bytes.NewBuffer(data)
	reader, _ := gzip.NewReader(b)

	var resB bytes.Buffer
	_, _ = resB.ReadFrom(reader)

	cer := &add_events.AddEventsRequest{}
	err := json.Unmarshal(resB.Bytes(), cer)
	return *cer, err
}

func (s *SuiteLogsE2EExporter) TestConsumeLogsShouldSucceed(assert, require *td.T) {
	createSettings := exportertest.NewNopCreateSettings()
	config := &Config{
		DatasetURL:      "https://example.com",
		APIKey:          "key-lib",
		MaxDelayMs:      "1",
		GroupBy:         []string{"attributes.container_id"},
		RetrySettings:   exporterhelper.NewDefaultRetrySettings(),
		QueueSettings:   exporterhelper.NewDefaultQueueSettings(),
		TimeoutSettings: exporterhelper.NewDefaultTimeoutSettings(),
	}

	attempt := atomic.Uint64{}
	wasSuccessful := atomic.Bool{}
	addRequest := add_events.AddEventsRequest{}
	httpmock.RegisterResponder(
		"POST",
		"https://example.com/api/addEvents",
		func(req *http.Request) (*http.Response, error) {
			attempt.Add(1)
			cer, err := extract(req)
			addRequest = cer

			assert.CmpNoError(err, "Error reading request: %v", err)
			if !assert.Nil(cer.SessionInfo) {
				assert.Cmp("b", cer.SessionInfo.ServerType)
				assert.Cmp("a", cer.SessionInfo.ServerId)
			}

			wasSuccessful.Store(true)
			return httpmock.NewJsonResponse(200, map[string]interface{}{
				"status":       "success",
				"bytesCharged": 42,
			})
		})

	lr := testdata.GenerateLogsOneLogRecord()

	exporterInstance = nil
	logs, err := createLogsExporter(context.Background(), createSettings, config)
	if assert.CmpNoError(err) {
		err = logs.Start(context.Background(), componenttest.NewNopHost())
		assert.CmpNoError(err)

		assert.NotNil(logs)
		err = logs.ConsumeLogs(context.Background(), lr)
		assert.Nil(err)
		time.Sleep(time.Second)
		err = logs.Shutdown(context.Background())
		assert.Nil(err)
	}

	assert.True(wasSuccessful.Load())
	info := httpmock.GetCallCountInfo()
	assert.CmpDeeply(info, map[string]int{"POST https://example.com/api/addEvents": 1})
	assert.Cmp(
		addRequest,
		add_events.AddEventsRequest{
			AuthParams: request.AuthParams{
				Token: "key-lib",
			},
			AddEventsRequestParams: add_events.AddEventsRequestParams{
				Session: assert.Anchor(td.HasSuffix("d41d8cd98f00b204e9800998ecf8427e"), "").(string),
				Events:  []*add_events.Event{testLEventReq},
				Threads: []*add_events.Thread{testLThread},
				Logs:    []*add_events.Log{testLLog},
			},
		})

}
