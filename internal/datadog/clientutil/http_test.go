// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package clientutil // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/datadog/clientutil"

import (
	"crypto/tls"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configcompression"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configtls"
)

var buildInfo = component.BuildInfo{
	Command: "otelcontribcol",
	Version: "1.0",
}

func TestNewHTTPClient(t *testing.T) {
	hcsEmpty := confighttp.ClientConfig{}
	client1 := NewHTTPClient(hcsEmpty)
	defaultTransport := &http.Transport{
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   5,
		IdleConnTimeout:       45 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ForceAttemptHTTP2:     false,
		TLSClientConfig:       &tls.Config{InsecureSkipVerify: false},
	}
	if diff := cmp.Diff(
		defaultTransport,
		client1.Transport.(*http.Transport),
		cmpopts.IgnoreUnexported(http.Transport{}, tls.Config{}),
		cmpopts.IgnoreFields(http.Transport{}, "Proxy", "DialContext")); diff != "" {
		t.Errorf("Mismatched transports -want +got %s", diff)
	}
	assert.Equal(t, time.Duration(0), client1.Timeout)

	idleConnTimeout := 30 * time.Second
	maxIdleConn := 300
	maxIdleConnPerHost := 150
	maxConnPerHost := 250
	hcs := confighttp.ClientConfig{
		ReadBufferSize:      100,
		WriteBufferSize:     200,
		Timeout:             10 * time.Second,
		IdleConnTimeout:     idleConnTimeout,
		MaxIdleConns:        maxIdleConn,
		MaxIdleConnsPerHost: maxIdleConnPerHost,
		MaxConnsPerHost:     maxConnPerHost,
		DisableKeepAlives:   true,
		TLS:                 configtls.ClientConfig{InsecureSkipVerify: true},
		ProxyURL:            "proxy",

		// The rest are ignored
		Endpoint:             "endpoint",
		Compression:          configcompression.TypeSnappy,
		HTTP2ReadIdleTimeout: 15 * time.Second,
		HTTP2PingTimeout:     20 * time.Second,
	}
	client2 := NewHTTPClient(hcs)
	expectedTransport := &http.Transport{
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ReadBufferSize:        100,
		WriteBufferSize:       200,
		MaxIdleConns:          maxIdleConn,
		MaxIdleConnsPerHost:   maxIdleConnPerHost,
		MaxConnsPerHost:       maxConnPerHost,
		IdleConnTimeout:       idleConnTimeout,
		DisableKeepAlives:     true,
		ForceAttemptHTTP2:     false,
		TLSClientConfig:       &tls.Config{InsecureSkipVerify: true},
	}
	if diff := cmp.Diff(
		expectedTransport,
		client2.Transport.(*http.Transport),
		cmpopts.IgnoreUnexported(http.Transport{}, tls.Config{}),
		cmpopts.IgnoreFields(http.Transport{}, "Proxy", "DialContext")); diff != "" {
		t.Errorf("Mismatched transports -want +got %s", diff)
	}
	assert.Equal(t, 10*time.Second, client2.Timeout)

	// Checking that the client config can receive ProxyUrl and
	// it will be passed to the http client.
	hcsForC3 := confighttp.ClientConfig{
		ReadBufferSize:      100,
		WriteBufferSize:     200,
		Timeout:             10 * time.Second,
		IdleConnTimeout:     idleConnTimeout,
		MaxIdleConns:        maxIdleConn,
		MaxIdleConnsPerHost: maxIdleConnPerHost,
		MaxConnsPerHost:     maxConnPerHost,
		DisableKeepAlives:   true,
		TLS:                 configtls.ClientConfig{InsecureSkipVerify: true},
		ProxyURL:            "http://datadog-proxy.myorganization.com:3128",

		// The rest are ignored
		Endpoint:             "endpoint",
		Compression:          configcompression.TypeSnappy,
		HTTP2ReadIdleTimeout: 15 * time.Second,
		HTTP2PingTimeout:     20 * time.Second,
	}
	ddURL, _ := url.Parse("https://datadoghq.com")
	parsedProxy, _ := url.Parse("http://datadog-proxy.myorganization.com:3128")
	client3 := NewHTTPClient(hcsForC3)
	tr3 := client3.Transport.(*http.Transport)
	url3, _ := tr3.Proxy(&http.Request{
		URL: ddURL,
	})
	assert.Equal(t, url3, parsedProxy)

	// Checking that the client config can receive ProxyUrl to override the
	// environment variable.
	t.Setenv("HTTPS_PROXY", "http://datadog-proxy-from-env.myorganization.com:3128")
	client4 := NewHTTPClient(hcsForC3)
	tr4 := client4.Transport.(*http.Transport)
	url4, _ := tr4.Proxy(&http.Request{
		URL: ddURL,
	})
	assert.Equal(t, url4, parsedProxy)

	// Checking that in the absence of ProxyUrl in the client config, the
	// environment variable is used for the http proxy.
	hcsForC5 := confighttp.ClientConfig{
		ReadBufferSize:      100,
		WriteBufferSize:     200,
		Timeout:             10 * time.Second,
		IdleConnTimeout:     idleConnTimeout,
		MaxIdleConns:        maxIdleConn,
		MaxIdleConnsPerHost: maxIdleConnPerHost,
		MaxConnsPerHost:     maxConnPerHost,
		DisableKeepAlives:   true,
		TLS:                 configtls.ClientConfig{InsecureSkipVerify: true},

		// The rest are ignored
		Endpoint:             "endpoint",
		Compression:          configcompression.TypeSnappy,
		HTTP2ReadIdleTimeout: 15 * time.Second,
		HTTP2PingTimeout:     20 * time.Second,
	}
	parsedEnvProxy, _ := url.Parse("http://datadog-proxy-from-env.myorganization.com:3128")
	client5 := NewHTTPClient(hcsForC5)
	tr5 := client5.Transport.(*http.Transport)
	url5, _ := tr5.Proxy(&http.Request{
		URL: ddURL,
	})
	assert.Equal(t, url5, parsedEnvProxy)
}

func TestUserAgent(t *testing.T) {
	assert.Equal(t, "otelcontribcol/1.0", UserAgent(buildInfo))
}

func TestDDHeaders(t *testing.T) {
	header := http.Header{}
	apiKey := "apikey"
	SetDDHeaders(header, buildInfo, apiKey)
	assert.Equal(t, header.Get("DD-Api-Key"), apiKey)
	assert.Equal(t, "otelcontribcol/1.0", header.Get("USer-Agent"))
}
