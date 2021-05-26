// Copyright OpenTelemetry Authors
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

// Copyright 2020, OpenTelemetry Authors
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

package dimensions

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"sync/atomic"
)

// ReqSender is a direct port of
// https://github.com/signalfx/signalfx-agent/blob/main/pkg/core/writer/requests/sender.go.
type ReqSender struct {
	client               *http.Client
	requests             chan *http.Request
	workerCount          uint
	ctx                  context.Context
	additionalDimensions map[string]string

	RunningWorkers         int64
	TotalRequestsStarted   int64
	TotalRequestsCompleted int64
	TotalRequestsFailed    int64
}

func NewReqSender(ctx context.Context, client *http.Client,
	workerCount uint, diagnosticDimensions map[string]string) *ReqSender {
	return &ReqSender{
		client:               client,
		additionalDimensions: diagnosticDimensions,
		// Unbuffered so that it blocks clients
		requests:    make(chan *http.Request),
		workerCount: workerCount,
		ctx:         ctx,
	}
}

// Send sends the request. Not thread-safe.
func (rs *ReqSender) Send(req *http.Request) {
	// Slight optimization to avoid spinning up unnecessary workers if there
	// aren't ever that many dim updates. Once workers start, they remain for the
	// duration of the agent.
	select {
	case rs.requests <- req:
		return
	default:
		if atomic.LoadInt64(&rs.RunningWorkers) < int64(rs.workerCount) {
			go rs.processRequests()
		}

		// Block until we can get through a request
		rs.requests <- req
	}
}

func (rs *ReqSender) processRequests() {
	atomic.AddInt64(&rs.RunningWorkers, int64(1))
	defer atomic.AddInt64(&rs.RunningWorkers, int64(-1))

	for {
		select {
		case <-rs.ctx.Done():
			return
		case req := <-rs.requests:
			atomic.AddInt64(&rs.TotalRequestsStarted, int64(1))
			if err := rs.sendRequest(req); err != nil {
				atomic.AddInt64(&rs.TotalRequestsFailed, int64(1))
				continue
			}
			atomic.AddInt64(&rs.TotalRequestsCompleted, int64(1))
		}
	}
}

func (rs *ReqSender) sendRequest(req *http.Request) error {
	body, statusCode, err := sendRequest(rs.client, req)
	// If it was successful there is nothing else to do.
	if statusCode == 200 {
		onRequestSuccess(req, body)
		return nil
	}

	if err != nil {
		err = fmt.Errorf("error making HTTP request to %s: %v", req.URL.String(), err)
	} else {
		err = fmt.Errorf("unexpected status code %d on response for request to %s: %s", statusCode, req.URL.String(), string(body))
	}

	onRequestFailed(req, statusCode, err)

	return err
}

type key int

const RequestFailedCallbackKey key = 1
const RequestSuccessCallbackKey key = 2

type RequestFailedCallback func(statusCode int, err error)
type RequestSuccessCallback func([]byte)

func onRequestSuccess(req *http.Request, body []byte) {
	ctx := req.Context()
	cb, ok := ctx.Value(RequestSuccessCallbackKey).(RequestSuccessCallback)
	if !ok {
		return
	}
	cb(body)
}
func onRequestFailed(req *http.Request, statusCode int, err error) {
	ctx := req.Context()
	cb, ok := ctx.Value(RequestFailedCallbackKey).(RequestFailedCallback)
	if !ok {
		return
	}
	cb(statusCode, err)
}

func sendRequest(client *http.Client, req *http.Request) ([]byte, int, error) {
	resp, err := client.Do(req)

	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	return body, resp.StatusCode, err
}
