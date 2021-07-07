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

package httpClient

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"net/http"
	"time"

	"github.com/hashicorp/go-retryablehttp"
)

const (
	defaultMaxRetries       = 3
	defaultTimeout          = 1 * time.Second
	defaultBackoffRetryBase = 200 * time.Millisecond
	maxHTTPResponseLength   = 5 * 1024 * 1024 // 5MB
)

type HTTPClient struct {
	client doer
}

type Requester interface {
	Request(ctx context.Context, path string) ([]byte, error)
}

type doer interface {
	Do(request *retryablehttp.Request) (*http.Response, error)
}

func withClientOption(f doer) clientOption {
	return func(h *HTTPClient) {
		h.client = f
	}
}

type clientOption func(*HTTPClient)

func New(options ...clientOption) Requester {

	httpClient := &HTTPClient{
		client: &retryablehttp.Client{
			HTTPClient:   &http.Client{Timeout: defaultTimeout},
			RetryWaitMin: defaultBackoffRetryBase,
			RetryWaitMax: time.Duration(float64(defaultBackoffRetryBase) * math.Pow(2, float64(defaultMaxRetries))),
			RetryMax:     defaultMaxRetries,
		},
	}

	for _, opt := range options {
		opt(httpClient)
	}

	return httpClient
}

func (h *HTTPClient) Request(ctx context.Context, endpoint string) ([]byte, error) {
	resp, err := h.clientGet(ctx, endpoint)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, err
	}
	if resp.ContentLength >= maxHTTPResponseLength {
		return nil, fmt.Errorf("get response with unexpected length from %s, response length: %d", endpoint, resp.ContentLength)
	}

	var reader io.Reader
	//value -1 indicates that the length is unknown, see https://golang.org/src/net/http/response.go
	//In this case, we read until the limit is reached
	//This might happen with chunked responses from ECS Introspection API
	if resp.ContentLength == -1 {
		reader = io.LimitReader(resp.Body, maxHTTPResponseLength)
	} else {
		reader = resp.Body
	}

	body, err := ioutil.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("unable to read response body from %s, error: %v", endpoint, err)
	}

	if len(body) == maxHTTPResponseLength {
		return nil, fmt.Errorf("response from %s, execeeds the maximum length: %v", endpoint, maxHTTPResponseLength)
	}
	return body, nil

}

func (h *HTTPClient) clientGet(ctx context.Context, url string) (resp *http.Response, err error) {
	req, err := retryablehttp.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	return h.client.Do(req.WithContext(ctx))
}
