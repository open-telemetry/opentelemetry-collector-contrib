// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package headerssetterextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/headerssetterextension"

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"go.opentelemetry.io/collector/config/configauth"
	"google.golang.org/grpc/credentials"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/headerssetterextension/internal/source"
)

type Header struct {
	key    string
	source source.Source
}

func newHeadersSetterExtension(cfg *Config) (configauth.ClientAuthenticator, error) {
	if cfg == nil {
		return nil, errors.New("extension configuration is not provided")
	}

	headers := make([]Header, 0, len(cfg.HeadersConfig))
	for _, header := range cfg.HeadersConfig {
		var s source.Source
		if header.Value != nil {
			s = &source.StaticSource{
				Value: *header.Value,
			}
		} else if header.FromContext != nil {
			s = &source.ContextSource{
				Key: *header.FromContext,
			}
		}
		headers = append(headers, Header{key: *header.Key, source: s})
	}

	return configauth.NewClientAuthenticator(
		configauth.WithClientRoundTripper(
			func(base http.RoundTripper) (http.RoundTripper, error) {
				return &headersRoundTripper{
					base:    base,
					headers: headers,
				}, nil
			}),
		configauth.WithPerRPCCredentials(func() (credentials.PerRPCCredentials, error) {
			return &headersPerRPC{headers: headers}, nil
		}),
	), nil

}

// headersPerRPC is a gRPC credentials.PerRPCCredentials implementation sets
// headers with values extracted from provided sources.
type headersPerRPC struct {
	headers []Header
}

// GetRequestMetadata returns the request metadata to be used with the RPC.
func (h *headersPerRPC) GetRequestMetadata(
	ctx context.Context,
	_ ...string,
) (map[string]string, error) {

	metadata := make(map[string]string, len(h.headers))
	for _, header := range h.headers {
		value, err := header.source.Get(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to determine the source: %w", err)
		}
		metadata[header.key] = value
	}
	return metadata, nil
}

// RequireTransportSecurity always returns true for this implementation.
func (h *headersPerRPC) RequireTransportSecurity() bool {
	return true
}

// headersRoundTripper intercepts downstream requests and sets headers with
// values extracted from configured sources.
type headersRoundTripper struct {
	base    http.RoundTripper
	headers []Header
}

// RoundTrip copies the original request and sets headers of the new requests
// with values extracted from configured sources.
func (h *headersRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	req2 := req.Clone(req.Context())
	if req2.Header == nil {
		req2.Header = make(http.Header)
	}
	for _, header := range h.headers {
		value, err := header.source.Get(req.Context())
		if err != nil {
			return nil, fmt.Errorf("failed to determine the source: %w", err)
		}
		req2.Header.Set(header.key, value)
	}
	return h.base.RoundTrip(req2)
}
