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
	"log"
	"math"
	"net/http"
	"time"

	"go.uber.org/zap"
)

const (
	defaultMaxRetries              = 3
	defaultTimeout                 = 1 * time.Second
	defaultBackoffRetryBaseInMills = 200
	maxHttpResponseLength          = 5 * 1024 * 1024 // 5MB
)

type HttpClient struct {
	maxRetries              int
	backoffRetryBaseInMills int
	client                  clientProvider
}

type HttpClientProvider interface {
	Request(path string, ctx context.Context, logger *zap.Logger) ([]byte, error)
}

type clientProvider interface {
	Do(request *http.Request) (*http.Response, error)
}

func New() HttpClientProvider {

	client := &http.Client{Timeout: defaultTimeout}

	return NewHttp(defaultMaxRetries, defaultBackoffRetryBaseInMills, client)

}

func NewHttp(maxRetries int, backoffRetryBaseInMills int, provider clientProvider) HttpClientProvider {
	httpClient := &HttpClient{
		maxRetries:              maxRetries,
		backoffRetryBaseInMills: backoffRetryBaseInMills,
		client:                  provider,
	}

	return httpClient
}

func (h *HttpClient) backoffSleep(currentRetryCount int) {
	backoffInMillis := int64(float64(h.backoffRetryBaseInMills) * math.Pow(2, float64(currentRetryCount)))
	sleepDuration := time.Millisecond * time.Duration(backoffInMillis)
	if sleepDuration > 60*1000 {
		sleepDuration = 60 * 1000
	}
	time.Sleep(sleepDuration)
}

func (h *HttpClient) Request(endpoint string, ctx context.Context, logger *zap.Logger) (body []byte, err error) {
	for i := 0; i < h.maxRetries; i++ {
		body, err = h.request(endpoint, ctx)
		if err != nil {
			log.Printf("W! retry [%d/%d], unable to get http response from %s, error: %v", i, h.maxRetries, endpoint, err)
			h.backoffSleep(i)
		}
	}
	return
}

func (h *HttpClient) request(endpoint string, ctx context.Context) ([]byte, error) {
	resp, err := h.clientGet(endpoint, ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to get response from %s, error: %v", endpoint, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unable to get response from %s, status code: %d", endpoint, resp.StatusCode)
	}

	if resp.ContentLength >= maxHttpResponseLength {
		return nil, fmt.Errorf("get response with unexpected length from %s, response length: %d", endpoint, resp.ContentLength)
	}

	var reader io.Reader
	//value -1 indicates that the length is unknown, see https://golang.org/src/net/http/response.go
	//In this case, we read until the limit is reached
	//This might happen with chunked responses from ECS Introspection API
	if resp.ContentLength == -1 {
		reader = io.LimitReader(resp.Body, maxHttpResponseLength)
	} else {
		reader = resp.Body
	}

	body, err := ioutil.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("unable to read response body from %s, error: %v", endpoint, err)
	}

	if len(body) == maxHttpResponseLength {
		return nil, fmt.Errorf("response from %s, execeeds the maximum length: %v", endpoint, maxHttpResponseLength)
	}
	return body, nil
}

func (h *HttpClient) clientGet(url string, ctx context.Context) (resp *http.Response, err error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	return h.client.Do(req)
}
