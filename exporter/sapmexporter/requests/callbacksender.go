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

package requests

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
)

// HTTPClient interface
type HTTPClient interface {
	Do(r *http.Request) (*http.Response, error)
}

type (
	// RequestFailedCallback is called when the request fails.
	RequestFailedCallback func(body []byte, statusCode int, err error)
	// RequestSuccessCallback is called on HTTP request success (all status codes).
	RequestSuccessCallback func(statusCode int, body []byte)
	key                    int
)

const (
	requestFailedCallbackKey  key = 1
	requestSuccessCallbackKey key = 2
)

// CallbackSender implements HTTPSender to send an HTTP request on the configured client. It sends
// callbacks on request success/failure taken from the request context.
type CallbackSender struct {
	Client HTTPClient
}

// SendRequest is called to send the given request on the configured HTTP client.
func (rs *CallbackSender) SendRequest(req *http.Request) error {
	resp, err := rs.Client.Do(req)

	if err != nil {
		onRequestFailed(req, nil, 0, err)
		return err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)

	// TODO: note that semantics of onRequestSuccess and onRequestFailed changed in ported code. Make sure
	// consumers of this use the new semantics.
	if err != nil {
		err = fmt.Errorf("error making HTTP request to %s: %v", req.URL.String(), err)
		onRequestFailed(req, body, resp.StatusCode, err)
		return err
	}

	onRequestSuccess(req, resp.StatusCode, body)
	return nil
}

func onRequestSuccess(req *http.Request, statusCode int, body []byte) {
	ctx := req.Context()
	cb, ok := ctx.Value(requestSuccessCallbackKey).(RequestSuccessCallback)
	if ok {
		cb(statusCode, body)
	}
}
func onRequestFailed(req *http.Request, body []byte, statusCode int, err error) {
	ctx := req.Context()
	cb, ok := ctx.Value(requestFailedCallbackKey).(RequestFailedCallback)
	if ok {
		cb(body, statusCode, err)
	}
}

// ContextWithSuccess attaches the provided function to the context to be called on success.
func ContextWithSuccess(ctx context.Context, f RequestSuccessCallback) context.Context {
	return context.WithValue(ctx, requestSuccessCallbackKey, f)
}

// ContextWithFailed attaches the provided function to the context to be called on failure.
func ContextWithFailed(ctx context.Context, f RequestFailedCallback) context.Context {
	return context.WithValue(ctx, requestFailedCallbackKey, f)
}
