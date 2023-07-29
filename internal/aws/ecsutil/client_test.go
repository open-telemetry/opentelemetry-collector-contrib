// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ecsutil

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
)

func TestClient(t *testing.T) {
	tr := &fakeRoundTripper{}
	baseURL, _ := url.Parse("http://localhost:8080")
	client := &clientImpl{
		baseURL:    *baseURL,
		httpClient: http.Client{Transport: tr},
	}
	require.False(t, tr.closed)
	resp, err := client.Get("/stats")
	require.NoError(t, err)
	require.Equal(t, "hello", string(resp))
	require.True(t, tr.closed)
	require.Equal(t, baseURL.String()+"/stats", tr.url)
	require.Equal(t, 1, len(tr.header))
	require.Equal(t, "application/json", tr.header["Content-Type"][0])
	require.Equal(t, "GET", tr.method)
}

func TestNewClientProvider(t *testing.T) {
	baseURL, _ := url.Parse("http://localhost:8080")
	provider := NewClientProvider(*baseURL, confighttp.HTTPClientSettings{}, componenttest.NewNopHost(), componenttest.NewNopTelemetrySettings())
	require.NotNil(t, provider)
	_, ok := provider.(*defaultClientProvider)
	require.True(t, ok)

	client, err := provider.BuildClient()
	require.NoError(t, err)
	require.Equal(t, "http://localhost:8080", client.(*clientImpl).baseURL.String())
}

func TestDefaultClient(t *testing.T) {
	endpoint, _ := url.Parse("http://localhost:8080")
	client, err := defaultClient(*endpoint, confighttp.HTTPClientSettings{}, componenttest.NewNopHost(), componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)
	require.NotNil(t, client.httpClient.Transport)
	require.Equal(t, "http://localhost:8080", client.baseURL.String())
}

func TestBuildReq(t *testing.T) {
	endpoint, _ := url.Parse("http://localhost:8080")
	p := &defaultClientProvider{
		baseURL:  *endpoint,
		host:     componenttest.NewNopHost(),
		settings: componenttest.NewNopTelemetrySettings(),
	}
	cl, err := p.BuildClient()
	require.NoError(t, err)

	req, err := cl.(*clientImpl).buildReq("/test")
	require.NoError(t, err)
	require.NotNil(t, req)
	require.Equal(t, "application/json", req.Header["Content-Type"][0])
}

func TestBuildBadReq(t *testing.T) {
	endpoint, _ := url.Parse("http://localhost:8080")
	p := &defaultClientProvider{
		baseURL:  *endpoint,
		host:     componenttest.NewNopHost(),
		settings: componenttest.NewNopTelemetrySettings(),
	}
	cl, err := p.BuildClient()
	require.NoError(t, err)
	_, err = cl.(*clientImpl).buildReq(" ")
	require.Error(t, err)
}

func TestGetBad(t *testing.T) {
	endpoint, _ := url.Parse("http://localhost:8080")
	p := &defaultClientProvider{
		baseURL:  *endpoint,
		host:     componenttest.NewNopHost(),
		settings: componenttest.NewNopTelemetrySettings(),
	}
	cl, err := p.BuildClient()
	require.NoError(t, err)
	resp, err := cl.(*clientImpl).Get(" ")
	require.Error(t, err)
	require.Nil(t, resp)
}

func TestFailedRT(t *testing.T) {
	tr := &fakeRoundTripper{failOnRT: true}
	endpoint, _ := url.Parse("http://localhost:8080")
	client := &clientImpl{
		baseURL:    *endpoint,
		httpClient: http.Client{Transport: tr},
	}
	_, err := client.Get("/test")
	require.Error(t, err)
}

func TestErrOnRead(t *testing.T) {
	tr := &fakeRoundTripper{errOnRead: true}
	endpoint, _ := url.Parse("http://localhost:8080")
	client := &clientImpl{
		baseURL:    *endpoint,
		httpClient: http.Client{Transport: tr},
		settings:   componenttest.NewNopTelemetrySettings(),
	}
	resp, err := client.Get("/foo")
	require.Error(t, err)
	require.Nil(t, resp)
}

func TestErrCode(t *testing.T) {
	tr := &fakeRoundTripper{errCode: true}
	endpoint, _ := url.Parse("http://localhost:8080")
	client := &clientImpl{
		baseURL:    *endpoint,
		httpClient: http.Client{Transport: tr},
		settings:   componenttest.NewNopTelemetrySettings(),
	}
	resp, err := client.Get("/foo")
	require.Error(t, err)
	require.Nil(t, resp)
}

var _ http.RoundTripper = (*fakeRoundTripper)(nil)

type fakeRoundTripper struct {
	closed     bool
	header     http.Header
	method     string
	url        string
	failOnRT   bool
	errOnClose bool
	errOnRead  bool
	errCode    bool
}

func (f *fakeRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if f.failOnRT {
		return nil, fmt.Errorf("failOnRT == true")
	}
	f.header = req.Header
	f.method = req.Method
	f.url = req.URL.String()
	var reader io.Reader
	if f.errOnRead {
		reader = &failingReader{}
	} else {
		reader = strings.NewReader("hello")
	}
	statusCode := 200
	if f.errCode {
		statusCode = 503
	}
	return &http.Response{
		StatusCode: statusCode,
		Body: &fakeReadCloser{
			Reader: reader,
			onClose: func() error {
				f.closed = true
				if f.errOnClose {
					return fmt.Errorf("error on close")
				}
				return nil
			},
		},
	}, nil
}

var _ io.Reader = (*failingReader)(nil)

type failingReader struct{}

func (f *failingReader) Read([]byte) (n int, err error) {
	return 0, fmt.Errorf("error on read")
}

var _ io.ReadCloser = (*fakeReadCloser)(nil)

type fakeReadCloser struct {
	io.Reader
	onClose func() error
}

func (f *fakeReadCloser) Close() error {
	return f.onClose()
}
