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

package endpoint

import (
	"context"
	"io"
	"net/http"
	"net/url"

	json "github.com/json-iterator/go"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/splunkenterprisereceiver/internal/request"
)

// Endpoint is a FQDN for a Splunk rest endpoint
type Endpoint string

// Option is an additional set of options
// that can be provided to query the endpoint that
// is not strictly needed.
type Option struct {
	Query      url.Values
	Method     string
	Body       io.Reader
	Client     *http.Client
	ReqFactory request.Factory
}

func NewEndpoint(hostname string, path string) (Endpoint, error) {
	u, err := url.Parse(hostname)
	if err != nil {
		return "", err
	}
	if u.Scheme == "" {
		u.Scheme = "http"
	}
	u.Path = path
	return Endpoint(u.String()), nil
}

// MarshalContent performs a HTTP call to the hostname + endpoint and will marshal the json into v.
// Additional options can be passed in to allow modifying querying the endpoint.
func (e Endpoint) MarshalContent(ctx context.Context, v any, opts ...func(eo *Option)) error {
	eo := &Option{
		Query: url.Values{
			"output_mode": []string{"json"},
		},
		Method:     http.MethodGet,
		Body:       http.NoBody,
		Client:     http.DefaultClient,
		ReqFactory: request.NewFactory(),
	}

	for _, opt := range opts {
		opt(eo)
	}

	u, err := url.Parse(string(e))
	if err != nil {
		return err
	}

	u.RawQuery = eo.Query.Encode()

	req, err := eo.ReqFactory.NewRequest(ctx, eo.Method, u.String(), eo.Body)
	if err != nil {
		return err
	}

	resp, err := eo.Client.Do(req)
	if err != nil {
		return err
	}

	return json.NewDecoder(resp.Body).Decode(v)
}
