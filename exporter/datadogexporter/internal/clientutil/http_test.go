// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package clientutil // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/clientutil"

import (
	"crypto/tls"
	"net/http"
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

var (
	buildInfo = component.BuildInfo{
		Command: "otelcontribcol",
		Version: "1.0",
	}
)

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
		IdleConnTimeout:     &idleConnTimeout,
		MaxIdleConns:        &maxIdleConn,
		MaxIdleConnsPerHost: &maxIdleConnPerHost,
		MaxConnsPerHost:     &maxConnPerHost,
		DisableKeepAlives:   true,
		TLSSetting:          configtls.ClientConfig{InsecureSkipVerify: true},

		// The rest are ignored
		Endpoint:             "endpoint",
		ProxyURL:             "proxy",
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
}

func TestUserAgent(t *testing.T) {

	assert.Equal(t, UserAgent(buildInfo), "otelcontribcol/1.0")
}

func TestDDHeaders(t *testing.T) {
	header := http.Header{}
	apiKey := "apikey"
	SetDDHeaders(header, buildInfo, apiKey)
	assert.Equal(t, header.Get("DD-Api-Key"), apiKey)
	assert.Equal(t, header.Get("USer-Agent"), "otelcontribcol/1.0")

}
