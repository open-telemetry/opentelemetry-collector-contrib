// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
// Originally copied from https://github.com/signalfx/signalfx-agent/blob/fbc24b0fdd3884bd0bbfbd69fe3c83f49d4c0b77/pkg/apm/requests/sender.go

package requests // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/internal/apm/requests"

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sync/atomic"
)

type ReqSender struct {
	client      *http.Client
	requests    chan *http.Request
	workerCount uint32
	ctx         context.Context
	clientName  string

	RunningWorkers         int64
	TotalRequestsStarted   int64
	TotalRequestsCompleted int64
	TotalRequestsFailed    int64
}

func NewReqSender(ctx context.Context, client *http.Client, workerCount uint, clientName string) *ReqSender {
	return &ReqSender{
		client:     client,
		clientName: clientName,
		// Unbuffered so that it blocks clients
		requests:    make(chan *http.Request),
		workerCount: uint32(workerCount),
		ctx:         ctx,
	}
}

func (rs *ReqSender) Send(req *http.Request) {
	// Slight optimization to avoid spinning up unnecessary workers if there
	// aren't ever that many dim updates. Once workers start, they remain for the
	// duration of the agent.
	select {
	case rs.requests <- req:
		return
	default:
		if atomic.LoadInt64(&rs.RunningWorkers) < int64(atomic.LoadUint32(&rs.workerCount)) {
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
		err = fmt.Errorf("error making HTTP request to %s: %w", req.URL.String(), err)
	} else {
		err = fmt.Errorf("unexpected status code %d on response for request to %s: %s", statusCode, req.URL.String(), string(body))
	}

	onRequestFailed(req, body, statusCode, err)

	return err
}

type key int

const RequestFailedCallbackKey key = 1
const RequestSuccessCallbackKey key = 2

type RequestFailedCallback func(body []byte, statusCode int, err error)
type RequestSuccessCallback func([]byte)

func onRequestSuccess(req *http.Request, body []byte) {
	ctx := req.Context()
	cb, ok := ctx.Value(RequestSuccessCallbackKey).(RequestSuccessCallback)
	if !ok {
		return
	}
	cb(body)
}
func onRequestFailed(req *http.Request, body []byte, statusCode int, err error) {
	ctx := req.Context()
	cb, ok := ctx.Value(RequestFailedCallbackKey).(RequestFailedCallback)
	if !ok {
		return
	}
	cb(body, statusCode, err)
}

func sendRequest(client *http.Client, req *http.Request) ([]byte, int, error) {
	resp, err := client.Do(req)

	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	return body, resp.StatusCode, err
}
