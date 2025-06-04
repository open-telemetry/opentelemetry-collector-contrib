// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package clientutil // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/datadog/clientutil"

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
)

var (
	// JSONHeaders headers for JSON requests.
	JSONHeaders = map[string]string{
		"Content-Type":     "application/json",
		"Content-Encoding": "gzip",
	}
	// ProtobufHeaders headers for protobuf requests.
	ProtobufHeaders = map[string]string{
		"Content-Type":     "application/x-protobuf",
		"Content-Encoding": "identity",
	}
)

// NewHTTPClient returns a http.Client configured with a subset of the confighttp.ClientConfig options.
func NewHTTPClient(hcs confighttp.ClientConfig) *http.Client {
	// If the ProxyURL field in the configuration is set, the HTTP client will use the proxy.
	// Otherwise, the HTTP client will use the system's proxy settings.
	httpProxy := http.ProxyFromEnvironment
	if parsedProxyURL, err := url.Parse(hcs.ProxyURL); err == nil && parsedProxyURL.Scheme != "" {
		httpProxy = http.ProxyURL(parsedProxyURL)
	}
	transport := http.Transport{
		Proxy: httpProxy,
		// Default values consistent with https://github.com/DataDog/datadog-agent/blob/f9ae7f4b842f83b23b2dfe3f15d31f9e6b12e857/pkg/util/http/transport.go#L91-L106
		DialContext: (&net.Dialer{
			Timeout: 30 * time.Second,
			// Enables TCP keepalives to detect broken connections
			KeepAlive: 30 * time.Second,
			// Disable RFC 6555 Fast Fallback ("Happy Eyeballs")
			FallbackDelay: -1 * time.Nanosecond,
		}).DialContext,
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 5,
		// This parameter is set to avoid connections sitting idle in the pool indefinitely
		IdleConnTimeout:       45 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		// Not supported by intake
		ForceAttemptHTTP2: false,
		TLSClientConfig:   &tls.Config{InsecureSkipVerify: hcs.TLS.InsecureSkipVerify},
	}
	if hcs.ReadBufferSize > 0 {
		transport.ReadBufferSize = hcs.ReadBufferSize
	}
	if hcs.WriteBufferSize > 0 {
		transport.WriteBufferSize = hcs.WriteBufferSize
	}
	if hcs.MaxIdleConns > 0 {
		transport.MaxIdleConns = hcs.MaxIdleConns
	}
	if hcs.MaxIdleConnsPerHost > 0 {
		transport.MaxIdleConnsPerHost = hcs.MaxIdleConnsPerHost
	}
	if hcs.MaxConnsPerHost > 0 {
		transport.MaxConnsPerHost = hcs.MaxConnsPerHost
	}
	if hcs.IdleConnTimeout > 0 {
		transport.IdleConnTimeout = hcs.IdleConnTimeout
	}
	transport.DisableKeepAlives = hcs.DisableKeepAlives
	return &http.Client{
		Timeout:   hcs.Timeout,
		Transport: &transport,
	}
}

// SetExtraHeaders appends a header map to HTTP headers.
func SetExtraHeaders(h http.Header, extras map[string]string) {
	for key, value := range extras {
		h.Set(key, value)
	}
}

func UserAgent(buildInfo component.BuildInfo) string {
	return fmt.Sprintf("%s/%s", buildInfo.Command, buildInfo.Version)
}

// SetDDHeaders sets the Datadog-specific headers
func SetDDHeaders(reqHeader http.Header, buildInfo component.BuildInfo, apiKey string) {
	reqHeader.Set("DD-Api-Key", apiKey)
	reqHeader.Set("User-Agent", UserAgent(buildInfo))
}
