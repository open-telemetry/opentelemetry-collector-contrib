package lmexporter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"go.opentelemetry.io/collector/config"

	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"
)

type logsResponse struct {
	Ok      int    `json:"linesOk"`
	Invalid int    `json:"linesInvalid"`
	Error   string `json:"error"`
}

type LogMockHTTPClient struct {
	URL            string
	Client         *http.Client
	IsTimeoutSet   bool
	RequestTimeOut time.Duration
}

func Test_newLogsExporter(t *testing.T) {

	type args struct {
		config *Config
		logger *zap.Logger
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			"newLMExporter: success",
			args{
				config: &Config{
					ExporterSettings: config.NewExporterSettings(config.NewComponentID("lmexporter")),
					URL:              "https://test.logicmonitor.com/rest",
					APIToken:         map[string]string{},
				},
				logger: zap.NewNop(),
			},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("LOGICMONITOR_ACCOUNT", "localdev")
			_, err := newLogsExporter(tt.args.config, tt.args.logger)
			if (err != nil) != tt.wantErr {
				t.Errorf("newLogsExporter() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func (c *LogMockHTTPClient) MakeRequest(version, method, baseURI, uri, configURL string, timeout time.Duration, pBytes *bytes.Buffer, headers map[string]string) (*APIResponse, error) {
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

	ctx, cancel := context.WithTimeout(req.Context(), timeout)

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
	statusCode := resp.StatusCode
	defer resp.Body.Close()

	if statusCode != http.StatusOK {
		return nil, fmt.Errorf("Request failed with status code: %d ", statusCode)
	}

	body, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response %s, failed with error %w", c.URL, err)
	}
	apiResp := APIResponse{body, resp.Header, resp.StatusCode, resp.ContentLength}
	return &apiResp, nil
}

func (c *LogMockHTTPClient) GetContent(url string) (*http.Response, error) {
	return nil, nil
}

func TestPushLogData(t *testing.T) {

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		response := logsResponse{
			Ok:      0,
			Invalid: 0,
		}
		body, _ := json.Marshal(response)
		w.Write(body)
	}))

	type args struct {
		ctx context.Context
		lg  pdata.Logs
	}

	type fields struct {
		// Input configuration.
		config *Config
		logger *zap.Logger
		client HttpClient
	}

	cfg := &Config{
		URL:      ts.URL,
		APIToken: map[string]string{},
	}

	test := struct {
		name   string
		fields fields
		args   args
	}{
		name: "Send Log data",
		fields: fields{
			logger: zap.NewNop(),
			config: cfg,
			client: &LogMockHTTPClient{
				URL:    ts.URL,
				Client: ts.Client(),
			},
		},
		args: args{
			ctx: context.Background(),
			lg:  createLogData(1),
		},
	}

	t.Run(test.name, func(t *testing.T) {

		e := &exporterImp{
			logger: test.fields.logger,
			config: test.fields.config,
			client: test.fields.client,
		}

		err := e.PushLogData(test.args.ctx, test.args.lg)

		if err != nil {
			t.Errorf("lmexporter.PushLogsData() error = %v", err)
			return
		}
	})
}

func TestExport(t *testing.T) {

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		response := logsResponse{
			Ok:      0,
			Invalid: 0,
		}
		body, _ := json.Marshal(response)
		w.Write(body)
	}))

	type args struct {
		ctx context.Context
		lg  pdata.Logs
	}

	type fields struct {
		// Input configuration.
		config *Config
		logger *zap.Logger
		client HttpClient
	}

	cfg := &Config{
		URL:      "https://test.logicmonitor.com/rest",
		APIToken: map[string]string{},
	}

	test := struct {
		name   string
		fields fields
		args   args
	}{
		name: "Test log export",
		fields: fields{
			logger: zap.NewNop(),
			config: cfg,
			client: &LogMockHTTPClient{
				URL:    ts.URL,
				Client: ts.Client(),
			},
		},
		args: args{
			ctx: context.Background(),
			lg:  createLogData(1),
		},
	}

	t.Run(test.name, func(t *testing.T) {

		e := &exporterImp{
			logger: test.fields.logger,
			config: test.fields.config,
			client: test.fields.client,
		}
		_, _, err := e.export("test")
		if err != nil {
			t.Errorf("lmexporter.PushLogsData() error = %v", err)
			return
		}
	})
}

func TestExport_Timeout(t *testing.T) {

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		response := logsResponse{
			Ok:      0,
			Invalid: 0,
		}
		body, _ := json.Marshal(response)
		w.Write(body)
	}))

	type args struct {
		ctx context.Context
		lg  pdata.Logs
	}

	type fields struct {
		// Input configuration.
		config *Config
		logger *zap.Logger
		client HttpClient
	}

	cfg := &Config{
		URL:      "https://test.logicmonitor.com/rest/",
		APIToken: map[string]string{},
	}

	test := struct {
		name   string
		fields fields
		args   args
	}{
		name: "Export with Timeout",
		fields: fields{
			logger: zap.NewNop(),
			config: cfg,
			client: &LogMockHTTPClient{
				URL:            ts.URL,
				Client:         ts.Client(),
				IsTimeoutSet:   true,
				RequestTimeOut: 1 * time.Nanosecond,
			},
		},
		args: args{
			ctx: context.Background(),
			lg:  createLogData(1),
		},
	}

	t.Run(test.name, func(t *testing.T) {

		e := &exporterImp{
			logger: test.fields.logger,
			config: test.fields.config,
			client: test.fields.client,
		}
		_, _, err := e.export("test")
		if err == nil {
			t.Errorf("lmexporter.PushLogsData() error = %v", err)
			return
		}
	})
}

func createLogData(numberOfLogs int) pdata.Logs {
	logs := pdata.NewLogs()
	logs.ResourceLogs().AppendEmpty() // Add an empty ResourceLogs
	rl := logs.ResourceLogs().AppendEmpty()
	rl.InstrumentationLibraryLogs().AppendEmpty() // Add an empty InstrumentationLibraryLogs
	ill := rl.InstrumentationLibraryLogs().AppendEmpty()

	for i := 0; i < numberOfLogs; i++ {
		ts := pdata.Timestamp(int64(i) * time.Millisecond.Nanoseconds())
		logRecord := ill.LogRecords().AppendEmpty()
		logRecord.Body().SetStringVal("mylog")
		logRecord.Attributes().InsertString("service.name", "myapp")
		logRecord.Attributes().InsertString("my-label", "myapp-type")
		logRecord.Attributes().InsertString("host.name", "myhost")
		logRecord.Attributes().InsertString("custom", "custom")
		logRecord.SetTimestamp(ts)
	}
	ill.LogRecords().AppendEmpty()

	return logs
}
