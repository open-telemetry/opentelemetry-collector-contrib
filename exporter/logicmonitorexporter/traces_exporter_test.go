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

package logicmonitorexporter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

var (
	TestSpanStartTime      = time.Date(2020, 2, 11, 20, 26, 12, 321, time.UTC)
	TestSpanStartTimestamp = pcommon.NewTimestampFromTime(TestSpanStartTime)

	TestSpanEventTime      = time.Date(2020, 2, 11, 20, 26, 13, 123, time.UTC)
	TestSpanEventTimestamp = pcommon.NewTimestampFromTime(TestSpanEventTime)

	TestSpanEndTime      = time.Date(2020, 2, 11, 20, 26, 13, 789, time.UTC)
	TestSpanEndTimestamp = pcommon.NewTimestampFromTime(TestSpanEndTime)
)

type tracesResponse struct {
	Ok      int    `json:"linesOk"`
	Invalid int    `json:"linesInvalid"`
	Error   string `json:"error"`
}

type TraceMockHTTPClient struct {
	URL            string
	Client         *http.Client
	IsTimeoutSet   bool
	RequestTimeOut time.Duration
}

func Test_newTraceExporter(t *testing.T) {

	type args struct {
		config    *Config
		logger    *zap.Logger
		buildInfo component.BuildInfo
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			"newTracesExporter: success",
			args{
				config: &Config{
					ExporterSettings: config.NewExporterSettings(config.NewComponentID("logicmonitor")),
					APIToken:         map[string]string{"access_id": "testid", "access_key": "testkey"},
				},
				logger:    zap.NewNop(),
				buildInfo: component.BuildInfo{},
			},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("LOGICMONITOR_ACCOUNT", "localdev")
			set := componenttest.NewNopExporterCreateSettings()
			_, err := newTracesExporter(tt.args.config, set)
			if (err != nil) != tt.wantErr {
				t.Errorf("newTracesExporter() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func Test_newTraceExporter_InvalidURL(t *testing.T) {

	type args struct {
		config    *Config
		logger    *zap.Logger
		buildInfo component.BuildInfo
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			"newTracesExporter: success",
			args{
				config: &Config{
					ExporterSettings: config.NewExporterSettings(config.NewComponentID("logicmonitor")),
					APIToken:         map[string]string{"access_id": "testid", "access_key": "testkey"},
				},
				logger:    zap.NewNop(),
				buildInfo: component.BuildInfo{},
			},
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("LOGICMONITOR_ACCOUNT", "localdev")
			tt.args.config.URL = "&(*)8#RE/48df$#rest"
			set := componenttest.NewNopExporterCreateSettings()
			_, err := newTracesExporter(tt.args.config, set)
			if (err != nil) != tt.wantErr {
				t.Errorf("newTracesExporter() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func (c *TraceMockHTTPClient) MakeRequest(ctx context.Context, version, method, baseURI, uri, configURL string, timeout time.Duration, pBytes *bytes.Buffer, headers map[string]string) (*APIResponse, error) {
	var err error
	var req *http.Request
	var body []byte

	if method == http.MethodPost && pBytes != nil {
		req, err = http.NewRequest(method, c.URL, pBytes)
	} else {
		req, err = http.NewRequest(method, c.URL, nil)
	}
	if err != nil {
		return nil, fmt.Errorf("creation of request failed with error %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)

	if c.IsTimeoutSet {
		ctx, cancel = context.WithTimeout(req.Context(), c.RequestTimeOut)
	}

	defer cancel()
	req = req.WithContext(ctx)

	req.Header.Set("X-version", version)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	resp, err := c.Client.Do(req)

	if err != nil {
		return nil, fmt.Errorf("sending request to %s failed with error %w", c.URL, err)
	}
	defer resp.Body.Close()

	body, err = io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response %s, failed with error %w", c.URL, err)
	}
	apiResp := APIResponse{body, resp.Header, resp.StatusCode, resp.ContentLength}
	return &apiResp, nil
}

func (c *TraceMockHTTPClient) GetContent(url string) (*http.Response, error) {
	return nil, nil
}

func TestPushTraceData(t *testing.T) {

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		response := tracesResponse{
			Ok:      0,
			Invalid: 0,
		}
		body, _ := json.Marshal(response)
		_, _ = w.Write(body)
	}))

	type args struct {
		ctx   context.Context
		trace ptrace.Traces
	}

	type fields struct {
		// Input configuration.
		config *Config
		logger *zap.Logger
		client HTTPClient
	}

	cfg := &Config{
		URL:      ts.URL,
		APIToken: map[string]string{"access_id": "testid", "access_key": "testkey"},
	}

	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		{
			name: "Send Trace data: Successful",
			fields: fields{
				logger: zap.NewNop(),
				config: cfg,
				client: &TraceMockHTTPClient{
					URL:    ts.URL,
					Client: ts.Client(),
				},
			},
			args: args{
				ctx:   context.Background(),
				trace: GenerateTracesOneSpan(),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {

			e := &tracesExporter{
				logger: test.fields.logger,
				config: test.fields.config,
				client: test.fields.client,
			}

			err := e.pushTraces(test.args.ctx, test.args.trace)

			if err != nil {
				t.Errorf("traces exporter.pushTraces() error = %v", err)
				return
			}
		})
	}
}

func TestTraceExport_ConnectionRefused(t *testing.T) {

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	type args struct {
		ctx   context.Context
		trace ptrace.Traces
	}

	type fields struct {
		// Input configuration.
		config *Config
		logger *zap.Logger
		client HTTPClient
	}

	cfg := &Config{
		APIToken: map[string]string{"access_id": "testid", "access_key": "testkey"},
	}

	test := struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		name: "Send Trace data: Connection Refused",
		fields: fields{
			logger: zap.NewNop(),
			config: cfg,
			client: &TraceMockHTTPClient{
				URL:    "http://test.logicmonitor.com/v1/traces",
				Client: ts.Client(),
			},
		},
		args: args{
			ctx:   context.Background(),
			trace: GenerateTracesOneSpan(),
		},
		wantErr: true,
	}

	t.Run(test.name, func(t *testing.T) {

		e := &tracesExporter{
			logger: test.fields.logger,
			config: test.fields.config,
			client: test.fields.client,
		}
		err := e.pushTraces(test.args.ctx, test.args.trace)
		if (err != nil) != test.wantErr {
			t.Errorf("traceexporter.pushTrace() error = %v, wantErr %v", err, test.wantErr)
			return
		}
	})
}

func TestPushTraceData_404(t *testing.T) {

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))

	type args struct {
		ctx   context.Context
		trace ptrace.Traces
	}

	type fields struct {
		// Input configuration.
		config *Config
		logger *zap.Logger
		client HTTPClient
	}

	cfg := &Config{
		URL:      ts.URL,
		APIToken: map[string]string{"access_id": "testid", "access_key": "testkey"},
	}

	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "Send Trace data: Error",
			fields: fields{
				logger: zap.NewNop(),
				config: cfg,
				client: &TraceMockHTTPClient{
					URL:    ts.URL,
					Client: ts.Client(),
				},
			},
			args: args{
				ctx:   context.Background(),
				trace: GenerateTracesOneSpan(),
			},
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {

			e := &tracesExporter{
				logger: test.fields.logger,
				config: test.fields.config,
				client: test.fields.client,
			}

			err := e.pushTraces(test.args.ctx, test.args.trace)
			if (err != nil) != test.wantErr {
				t.Errorf("traceexporter.pushTrace() error = %v, wantErr %v", err, test.wantErr)
				return
			}
		})
	}
}

func TestPushTraceData_Retry(t *testing.T) {

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "200")
		w.WriteHeader(http.StatusTooManyRequests)
	}))

	type args struct {
		ctx   context.Context
		trace ptrace.Traces
	}

	type fields struct {
		// Input configuration.
		config *Config
		logger *zap.Logger
		client HTTPClient
	}

	cfg := &Config{
		URL:      ts.URL,
		APIToken: map[string]string{"access_id": "testid", "access_key": "testkey"},
	}

	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "Send Trace data: Error",
			fields: fields{
				logger: zap.NewNop(),
				config: cfg,
				client: &TraceMockHTTPClient{
					URL:    ts.URL,
					Client: ts.Client(),
				},
			},
			args: args{
				ctx:   context.Background(),
				trace: GenerateTracesOneSpan(),
			},
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {

			e := &tracesExporter{
				logger: test.fields.logger,
				config: test.fields.config,
				client: test.fields.client,
			}

			err := e.pushTraces(test.args.ctx, test.args.trace)
			if (err != nil) != test.wantErr {
				t.Errorf("traceexporter.pushTrace() error = %v, wantErr %v", err, test.wantErr)
				return
			}
		})
	}
}

func TestPushTraceData_BadRequest(t *testing.T) {

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"code":400,"message":"Bad Request","details":"error details"}`))
	}))

	type args struct {
		ctx   context.Context
		trace ptrace.Traces
	}

	type fields struct {
		// Input configuration.
		config *Config
		logger *zap.Logger
		client HTTPClient
	}

	cfg := &Config{
		URL:      ts.URL,
		APIToken: map[string]string{"access_id": "testid", "access_key": "testkey"},
	}

	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "Send Trace data: Error",
			fields: fields{
				logger: zap.NewNop(),
				config: cfg,
				client: &TraceMockHTTPClient{
					URL:    ts.URL,
					Client: ts.Client(),
				},
			},
			args: args{
				ctx:   context.Background(),
				trace: GenerateTracesOneSpan(),
			},
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {

			e := &tracesExporter{
				logger: test.fields.logger,
				config: test.fields.config,
				client: test.fields.client,
			}

			err := e.pushTraces(test.args.ctx, test.args.trace)
			if (err != nil) != test.wantErr {
				t.Errorf("traceexporter.pushTrace() error = %v, wantErr %v", err, test.wantErr)
				return
			}
		})
	}
}

func GetAvailableLocalAddress(t *testing.T) string {
	ln, _ := net.Listen("tcp", "localhost:0")
	// There is a possible race if something else takes this same port before
	// the test uses it, however, that is unlikely in practice.
	defer ln.Close()
	return ln.Addr().String()
}

func GenerateTracesOneSpan() ptrace.Traces {
	td := GenerateTracesOneEmptyInstrumentationLibrary()
	rs0ils0 := td.ResourceSpans().At(0).ScopeSpans().At(0)
	fillSpanOne(rs0ils0.Spans().AppendEmpty())
	return td
}

func GenerateTracesOneEmptyInstrumentationLibrary() ptrace.Traces {
	td := GenerateTracesNoLibraries()
	td.ResourceSpans().At(0).ScopeSpans().AppendEmpty()
	return td
}

func GenerateTracesOneEmptyResourceSpans() ptrace.Traces {
	td := ptrace.NewTraces()
	td.ResourceSpans().AppendEmpty()
	return td
}

func GenerateTracesNoLibraries() ptrace.Traces {
	td := GenerateTracesOneEmptyResourceSpans()
	rs0 := td.ResourceSpans().At(0)
	rs0.Resource().Attributes().PutString("service.name", "uop.stage-eu-1")
	rs0.Resource().Attributes().PutString("outsystems.module.version", "903386")
	return td
}

func fillSpanOne(span ptrace.Span) {
	span.SetName("operationA")
	span.SetStartTimestamp(TestSpanStartTimestamp)
	span.SetEndTimestamp(TestSpanEndTimestamp)
	span.SetDroppedAttributesCount(1)
	evs := span.Events()
	ev0 := evs.AppendEmpty()
	ev0.SetTimestamp(TestSpanEventTimestamp)
	ev0.SetName("event-with-attr")
	span.Attributes().PutInt("span_index", 3)
	span.Attributes().PutString("code.function", "myFunction")
	ev0.SetDroppedAttributesCount(2)
	ev1 := evs.AppendEmpty()
	ev1.SetTimestamp(TestSpanEventTimestamp)
	ev1.SetName("event")
	ev1.SetDroppedAttributesCount(2)
	span.SetDroppedEventsCount(1)
	status := span.Status()
	status.SetCode(ptrace.StatusCodeError)
	status.SetMessage("status-cancelled")
}
