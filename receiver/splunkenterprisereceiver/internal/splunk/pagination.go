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

package splunk

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"

	json "github.com/json-iterator/go"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/splunkenterprisereceiver/internal/request"
)

type requestData[T any] struct {
	Paging struct {
		Offset int `json:"offset"`
		Total  int `json:"total"`
	} `json:"paging"`
	Entry struct {
		Content []T `json:"content"`
	} `json:"entry"`
}

type RequestOption struct {
	Hostname   string
	Method     string
	Query      url.Values
	Body       io.Reader
	Client     *http.Client
	ReqFactory request.Factory
	Count      int
}

type RequestOptionFunc func(ro *RequestOption)

func NewDefaultRequestOption() RequestOption {
	return RequestOption{
		Hostname: "https://127.0.0.1:8089",
		Method:   http.MethodGet,
		Query: url.Values{
			"output_mode": []string{"json"},
		},
		Body:       http.NoBody,
		Client:     http.DefaultClient,
		ReqFactory: request.NewFactory(),
		Count:      100,
	}
}

func WithHostname(hostname string) RequestOptionFunc {
	return func(ro *RequestOption) {
		ro.Hostname = hostname
	}
}

func WithMethod(method string) RequestOptionFunc {
	return func(ro *RequestOption) {
		ro.Method = method
	}
}

func WithQuery(query url.Values) RequestOptionFunc {
	return func(ro *RequestOption) {
		for k, v := range query {
			ro.Query[k] = v
		}
	}
}

func WithBody(body io.Reader) RequestOptionFunc {
	return func(ro *RequestOption) {
		ro.Body = body
	}
}

func WithClient(client *http.Client) RequestOptionFunc {
	return func(ro *RequestOption) {
		ro.Client = client
	}
}

func WithRequestFactory(rf request.Factory) RequestOptionFunc {
	return func(ro *RequestOption) {
		ro.ReqFactory = rf
	}
}

func WithCount(count int) RequestOptionFunc {
	return func(ro *RequestOption) {
		ro.Count = count
	}
}

func FetchPaginatedData[T any](ctx context.Context, path string, opts ...RequestOptionFunc) ([]T, error) {
	ro := NewDefaultRequestOption()
	for _, opt := range opts {
		opt(&ro)
	}

	u, err := url.Parse(ro.Hostname)
	if err != nil {
		return nil, err
	}
	u.Path = path

	var (
		total   = 1
		results = make([]T, 0, ro.Count)
	)
	for len(results) < total {
		q := ro.Query
		q.Set("offset", fmt.Sprint(len(results)))
		q.Set("count", fmt.Sprint(ro.Count))

		snapshot := *u
		snapshot.RawQuery = q.Encode()

		req, err := ro.ReqFactory.NewRequest(ctx, ro.Method, u.String(), ro.Body)
		if err != nil {
			return nil, err
		}

		resp, err := ro.Client.Do(req)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode/100 != 2 {
			return nil, multierr.Combine(
				resp.Body.Close(),
				fmt.Errorf("failed to read data with status code %d", resp.StatusCode),
			)
		}

		var rd requestData[T]
		err = json.NewDecoder(resp.Body).Decode(&rd)
		if multierr.Combine(err, resp.Body.Close()) != nil {
			return nil, err
		}
		total = rd.Paging.Total
		results = append(results, rd.Entry.Content...)
	}

	return results, nil
}
