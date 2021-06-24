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
	"context"
	"errors"
	"io/ioutil"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/translator/conventions"
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

func newTestHTTPClient(respCode *int, respBody *string, err error) *http.Client {
	var rt http.RoundTripper = testRoundTripper(func(req *http.Request) *http.Response {
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

func newTestClient(config *Config, httpClient *http.Client, clock clock) *client {
	return &client{
		client: httpClient,
		logger: zap.NewNop(),
		config: config,
		clock:  clock,
	}
}

func createLogData(clock clock) pdata.Logs {
	logs := pdata.NewLogs()
	logs.ResourceLogs().Resize(1)

	now := clock.Now()

	rl := logs.ResourceLogs().At(0)
	rl.InstrumentationLibraryLogs().Resize(1)

	ill := rl.InstrumentationLibraryLogs().At(0)
	ill.Logs().Resize(1)

	logRecord := ill.Logs().At(0)

	logRecord.SetTimestamp(pdata.Timestamp(now.UnixNano()))
	logRecord.Body().SetStringVal("message")
	logRecord.Attributes().InsertString(conventions.AttributeNetHostIP, "1.1.1.1")
	logRecord.Attributes().InsertInt(conventions.AttributeNetHostPort, 4000)
	logRecord.Attributes().InsertInt("recordNum", 0)

	return logs
}

func TestClientSendLogs(t *testing.T) {
	type testCaseRequest struct {
		// Inputs
		logs           pdata.Logs
		responseStatus int
		respBody       string
		clockOverride  clock
		//Outputs
		shouldError      bool
		errorIsPermanant bool
	}
	staticClock := newMockClock(time.Unix(0, 0))
	staticClockAfterThrottle := newMockClock(staticClock.time.Add(throttleDuration + 1*time.Nanosecond))

	testCases := []struct {
		name        string
		config      Config
		clock       clock
		clientError error
		reqs        []testCaseRequest
	}{
		{
			name: "Happy path",
			config: Config{
				Endpoint:  testURL,
				AgentName: "agent",
			},
			clock: staticClock,
			reqs: []testCaseRequest{
				{
					logs:           createLogData(staticClock),
					responseStatus: 200,
					respBody:       "",
				},
			},
		},
		{
			name: "throttling",
			config: Config{
				Endpoint:  testURL,
				AgentName: "agent",
			},
			clock: staticClock,
			reqs: []testCaseRequest{
				{
					logs:             createLogData(staticClock),
					responseStatus:   401,
					respBody:         "",
					shouldError:      true,
					errorIsPermanant: false,
				},
				{
					logs:             createLogData(staticClock),
					responseStatus:   200, // Client is throttled, so the client will never get to this point
					respBody:         "",
					shouldError:      true,
					errorIsPermanant: false,
				},
				{
					logs:             createLogData(staticClock),
					responseStatus:   200,
					respBody:         "",
					shouldError:      false,
					errorIsPermanant: false,
					clockOverride:    staticClockAfterThrottle,
				},
			},
		},
		{
			name: "bad request errors permanent",
			config: Config{
				Endpoint:  testURL,
				AgentName: "agent",
			},
			clock: staticClock,
			reqs: []testCaseRequest{
				{
					logs:             createLogData(staticClock),
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
				AgentName: "agent",
			},
			clock: staticClock,
			reqs: []testCaseRequest{
				{
					logs:             createLogData(staticClock),
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
				AgentName: "agent",
			},
			clock:       staticClock,
			clientError: errors.New("dial tcp failed"),
			reqs: []testCaseRequest{
				{
					logs:             createLogData(staticClock),
					shouldError:      true,
					errorIsPermanant: false,
				},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			var respCode int
			var respBody string

			httpClient := newTestHTTPClient(&respCode, &respBody, testCase.clientError)
			c := newTestClient(&testCase.config, httpClient, testCase.clock)

			err := c.start(context.Background(), componenttest.NewNopHost())
			require.NoError(t, err)

			for _, req := range testCase.reqs {
				respCode = req.responseStatus
				respBody = req.respBody

				if req.clockOverride != nil {
					c.clock = req.clockOverride
				}

				err = c.sendLogs(context.Background(), req.logs)

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
