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

package utils

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"time"
)

var (
	JSONHeaders map[string]string = map[string]string{
		"Content-Type":     "application/json",
		"Content-Encoding": "gzip",
	}
	ProtobufHeaders map[string]string = map[string]string{
		"Content-Type":     "application/x-protobuf",
		"Content-Encoding": "identity",
	}
)

// NewClient returns a http.Client configured with the Agent options.
func NewHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				// Disable RFC 6555 Fast Fallback ("Happy Eyeballs")
				FallbackDelay: -1 * time.Nanosecond,
			}).DialContext,
			MaxIdleConns: 100,
			// Not supported by intake
			ForceAttemptHTTP2: false,
			TLSClientConfig:   &tls.Config{InsecureSkipVerify: false},
		},
	}
}

// SetExtraHeaders appends a header map to HTTP headers.
func SetExtraHeaders(h http.Header, extras map[string]string) {
	for key, value := range extras {
		h.Set(key, value)
	}
}

func SetDDHeaders(reqHeader http.Header, apiKey string) {
	// userAgent is the computed user agent we'll use when
	// communicating with Datadog
	var userAgent = fmt.Sprintf(
		"%s/%s/%s (+%s)",
		"otel-collector-exporter", "0.1", "1", "http://localhost",
	)

	reqHeader.Set("DD-Api-Key", apiKey)
	reqHeader.Set("User-Agent", userAgent)
}
