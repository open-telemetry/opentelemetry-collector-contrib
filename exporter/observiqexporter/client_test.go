// Copyright  OpenTelemetry Authors
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

package observiqexporter

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/model/pdata"
	conventions "go.opentelemetry.io/collector/model/semconv/v1.5.0"
	"go.uber.org/zap"
)

const testURL = "http://example.com"

type testRoundTripper func(req *http.Request) *http.Response

func (t testRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return t(req), nil
}

type testFailRoundTripper struct {
	err error
}

func (t testFailRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return nil, t.err
}

type requestVerificationFunc func(t *testing.T, req *http.Request)

func newTestHTTPClient(t *testing.T, respCode *int, respBody *string, testFunc *requestVerificationFunc, err error) *http.Client {
	var rt http.RoundTripper = testRoundTripper(func(req *http.Request) *http.Response {
		if testFunc != nil && *testFunc != nil {
			(*testFunc)(t, req)
		}

		return &http.Response{
			StatusCode: *respCode,
			Body:       ioutil.NopCloser(bytes.NewBufferString(*respBody)),
			Header:     make(http.Header),
		}
	})

	if err != nil {
		rt = testFailRoundTripper{err: err}
	}

	return &http.Client{
		Transport: rt,
	}
}

func newTestClient(config *Config, httpClient *http.Client) *client {
	return &client{
		client:       httpClient,
		logger:       zap.NewNop(),
		config:       config,
		buildVersion: component.NewDefaultBuildInfo().Version,
	}
}

func createLogData() pdata.Logs {
	logs := pdata.NewLogs()
	logs.ResourceLogs().EnsureCapacity(1)

	now := timeNow()

	rl := logs.ResourceLogs().AppendEmpty()
	rl.InstrumentationLibraryLogs().EnsureCapacity(1)

	ill := rl.InstrumentationLibraryLogs().AppendEmpty()
	ill.Logs().EnsureCapacity(1)

	logRecord := ill.Logs().AppendEmpty()

	logRecord.SetTimestamp(pdata.Timestamp(now.UnixNano()))
	logRecord.Body().SetStringVal("message")
	logRecord.Attributes().InsertString(conventions.AttributeNetHostIP, "1.1.1.1")
	logRecord.Attributes().InsertInt(conventions.AttributeNetHostPort, 4000)
	logRecord.Attributes().InsertInt("recordNum", 0)

	return logs
}

type observIQLogAddRequest struct {
	Logs []observIQLogAddRequestLog `json:"logs"`
}

type observIQLogAddRequestLog struct {
	ID    string           `json:"id"`
	Size  int              `json:"size"`
	Entry observIQLogEntry `json:"entry"`
}

func verifyFirstElementIsEntryFunc(e observIQLogEntry) requestVerificationFunc {
	return func(t *testing.T, req *http.Request) {
		gunzip, err := gzip.NewReader(req.Body)

		require.NoError(t, err)

		parsedReq := observIQLogAddRequest{}
		err = json.NewDecoder(gunzip).Decode(&parsedReq)

		require.NoError(t, err)
		require.NotEmpty(t, parsedReq.Logs)
		require.Equal(t, e, parsedReq.Logs[0].Entry)
	}
}

func TestClientSendLogs(t *testing.T) {
	type testCaseRequest struct {
		// Inputs
		logs           pdata.Logs
		responseStatus int
		respBody       string
		timeoutTimer   bool // Timeout the last set timer created through timeAfterFunc()
		//Outputs
		verifyRequest    requestVerificationFunc // Function is used to verify the request submitted to the client is valid.
		shouldError      bool
		errorIsPermanant bool
	}

	timeNow = func() time.Time {
		return time.Unix(0, 0)
	}

	defaultLogEntry := observIQLogEntry{
		Timestamp: "1970-01-01T00:00:00.000Z",
		Data: map[string]interface{}{
			strings.ReplaceAll(conventions.AttributeNetHostIP, ".", "_"):   "1.1.1.1",
			strings.ReplaceAll(conventions.AttributeNetHostPort, ".", "_"): float64(4000),
			"recordNum": float64(0),
		},
		Message:  "message",
		Severity: "default",
		Agent:    &observIQAgentInfo{ID: "0", Name: "agent", Version: "latest"},
	}

	testCases := []struct {
		name        string
		config      Config
		clientError error
		reqs        []testCaseRequest
	}{
		{
			name: "Happy path",
			config: Config{
				Endpoint:  testURL,
				AgentID:   "0",
				AgentName: "agent",
			},
			reqs: []testCaseRequest{
				{
					logs:           createLogData(),
					responseStatus: 200,
					respBody:       "",
					verifyRequest:  verifyFirstElementIsEntryFunc(defaultLogEntry),
				},
			},
		},
		{
			name: "throttling",
			config: Config{
				Endpoint:  testURL,
				AgentID:   "0",
				AgentName: "agent",
			},
			reqs: []testCaseRequest{
				{
					logs:             createLogData(),
					responseStatus:   401,
					respBody:         "",
					shouldError:      true,
					errorIsPermanant: true,
				},
				{
					logs:             createLogData(),
					responseStatus:   200, // Client is throttled, so the client will never get to this point
					respBody:         "",
					shouldError:      true,
					errorIsPermanant: true,
				},
				{
					logs:             createLogData(),
					responseStatus:   200,
					respBody:         "",
					timeoutTimer:     true,
					shouldError:      false,
					errorIsPermanant: false,
				},
			},
		},
		{
			name: "bad request errors permanent",
			config: Config{
				Endpoint:  testURL,
				AgentID:   "0",
				AgentName: "agent",
			},
			reqs: []testCaseRequest{
				{
					logs:             createLogData(),
					responseStatus:   400,
					respBody:         "",
					shouldError:      true,
					errorIsPermanant: true,
				},
			},
		},
		{
			name: "500 error",
			config: Config{
				Endpoint:  testURL,
				AgentID:   "0",
				AgentName: "agent",
			},
			reqs: []testCaseRequest{
				{
					logs:             createLogData(),
					responseStatus:   500,
					respBody:         "",
					shouldError:      true,
					errorIsPermanant: false,
				},
			},
		},
		{
			name: "client error",
			config: Config{
				Endpoint:  testURL,
				AgentID:   "0",
				AgentName: "agent",
			},
			clientError: errors.New("dial tcp failed"),
			reqs: []testCaseRequest{
				{
					logs:             createLogData(),
					shouldError:      true,
					errorIsPermanant: false,
				},
			},
		},
		{
			name: "APIKey header exists if APIKey is specified",
			config: Config{
				Endpoint:  testURL,
				AgentName: "agent",
				APIKey:    "api-key",
			},
			reqs: []testCaseRequest{
				{
					logs:           createLogData(),
					responseStatus: 200,
					respBody:       "",
					verifyRequest: func(t *testing.T, req *http.Request) {
						require.Equal(t, "api-key", req.Header.Get("x-cabin-api-key"))
					},
				},
			},
		},
		{
			name: "Secret Key header exists if SecretKey is specified",
			config: Config{
				Endpoint:  testURL,
				AgentName: "agent",
				SecretKey: "secret-key",
			},
			reqs: []testCaseRequest{
				{
					logs:           createLogData(),
					responseStatus: 200,
					respBody:       "",
					verifyRequest: func(t *testing.T, req *http.Request) {
						require.Equal(t, "secret-key", req.Header.Get("x-cabin-secret-key"))
					},
				},
			},
		},
	}
	var timerFunc func()
	timeAfterFunc = func(d time.Duration, f func()) *time.Timer {
		timerFunc = f
		return time.NewTimer(d)
	}

	for _, testCase := range testCases {
		timerFunc = nil
		t.Run(testCase.name, func(t *testing.T) {
			var respCode int
			var respBody string
			var verifyFunc requestVerificationFunc

			httpClient := newTestHTTPClient(t, &respCode, &respBody, &verifyFunc, testCase.clientError)
			c := newTestClient(&testCase.config, httpClient)

			for _, req := range testCase.reqs {
				respCode = req.responseStatus
				respBody = req.respBody
				verifyFunc = req.verifyRequest

				if req.timeoutTimer {
					require.NotNil(t, timerFunc)
					timerFunc()
				}

				err := c.sendLogs(context.Background(), req.logs)

				if req.shouldError {
					require.Error(t, err)
					require.Equal(t, req.errorIsPermanant, consumererror.IsPermanent(err))
				} else {
					require.NoError(t, err)
				}

				err = c.stop(context.Background())
				require.NoError(t, err)
			}
		})
	}
}
