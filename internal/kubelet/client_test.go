// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

// TODO review if tests should succeed on Windows

package kubelet

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/configtls"
	"go.uber.org/zap"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
)

const (
	certPath             = "./testdata/testcert.crt"
	keyFile              = "./testdata/testkey.key"
	errSignedByUnknownCA = "tls: failed to verify certificate: x509: certificate signed by unknown authority"
)

func TestClient(t *testing.T) {
	tr := &fakeRoundTripper{}
	baseURL := "http://localhost:9876"
	client := &clientImpl{
		baseURL:    baseURL,
		httpClient: http.Client{Transport: tr},
	}
	require.False(t, tr.closed)
	resp, err := client.Get("/foo")
	require.NoError(t, err)
	require.Equal(t, "hello", string(resp))
	require.True(t, tr.closed)
	require.Equal(t, baseURL+"/foo", tr.url)
	require.Len(t, tr.header, 1)
	require.Equal(t, "application/json", tr.header["Content-Type"][0])
	require.Equal(t, http.MethodGet, tr.method)
}

func TestNewTLSClientProvider(t *testing.T) {
	p, err := NewClientProvider("localhost:9876", &ClientConfig{
		APIConfig: k8sconfig.APIConfig{
			AuthType: k8sconfig.AuthTypeTLS,
		},
		Config: configtls.Config{
			CAFile:   certPath,
			CertFile: certPath,
			KeyFile:  keyFile,
		},
	}, zap.NewNop())
	require.NoError(t, err)
	client, err := p.BuildClient()
	require.NoError(t, err)
	c := client.(*clientImpl)
	tcc := c.httpClient.Transport.(*http.Transport).TLSClientConfig
	require.Len(t, tcc.Certificates, 1)
	require.NotNil(t, tcc.RootCAs)
}

func TestNewSAClientProvider(t *testing.T) {
	p, err := NewClientProvider("localhost:9876", &ClientConfig{
		APIConfig: k8sconfig.APIConfig{
			AuthType: k8sconfig.AuthTypeServiceAccount,
		},
	}, zap.NewNop())
	require.NoError(t, err)
	require.NotNil(t, p)
	_, ok := p.(*saClientProvider)
	require.True(t, ok)
}

func TestNewKubeConfigClientProvider(t *testing.T) {
	p, err := NewClientProvider("localhost:9876", &ClientConfig{
		APIConfig: k8sconfig.APIConfig{
			AuthType: k8sconfig.AuthTypeKubeConfig,
		},
	}, zap.NewNop())
	require.NoError(t, err)
	require.NotNil(t, p)
	_, ok := p.(*kubeConfigClientProvider)
	require.True(t, ok)
}

func TestDefaultTLSClient(t *testing.T) {
	endpoint := "localhost:9876"
	client, err := defaultTLSClient(endpoint, true, &x509.CertPool{}, nil, nil, zap.NewNop())
	require.NoError(t, err)
	require.NotNil(t, client.httpClient.Transport)
	require.Equal(t, "https://"+endpoint, client.baseURL)
}

func TestSvcAcctClient(t *testing.T) {
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		// Check if call is authenticated using token from test file
		assert.Equal(t, "Bearer s3cr3t", req.Header.Get("Authorization"))
		_, err := rw.Write([]byte(`OK`))
		assert.NoError(t, err)
	}))
	cert, err := tls.LoadX509KeyPair(certPath, keyFile)
	require.NoError(t, err)
	server.TLS = &tls.Config{Certificates: []tls.Certificate{cert}}
	server.StartTLS()
	defer server.Close()

	p := &saClientProvider{
		endpoint:   server.Listener.Addr().String(),
		caCertPath: certPath,
		tokenPath:  "./testdata/token",
		cfg: &ClientConfig{
			InsecureSkipVerify: false,
		},
		logger: zap.NewNop(),
	}
	client, err := p.BuildClient()
	require.NoError(t, err)
	resp, err := client.Get("/")
	require.NoError(t, err)
	require.Equal(t, []byte(`OK`), resp)
}

func TestSAClientCustomCA(t *testing.T) {
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		// Check if call is authenticated using token from test file
		assert.Equal(t, "Bearer s3cr3t", req.Header.Get("Authorization"))
		_, err := rw.Write([]byte(`OK`))
		assert.NoError(t, err)
	}))
	cert, err := tls.LoadX509KeyPair(certPath, keyFile)
	require.NoError(t, err)
	server.TLS = &tls.Config{Certificates: []tls.Certificate{cert}}
	server.StartTLS()
	defer server.Close()

	p := &saClientProvider{
		endpoint:   server.Listener.Addr().String(),
		caCertPath: certPath,
		tokenPath:  "./testdata/token",
		cfg: &ClientConfig{
			InsecureSkipVerify: false,
			Config: configtls.Config{
				CAFile: certPath,
			},
		},
		logger: zap.NewNop(),
	}
	client, err := p.BuildClient()
	require.NoError(t, err)
	resp, err := client.Get("/")
	require.NoError(t, err)
	require.Equal(t, []byte(`OK`), resp)
}

func TestSAClientBadTLS(t *testing.T) {
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(rw http.ResponseWriter, _ *http.Request) {
		_, _ = rw.Write([]byte(`OK`))
	}))
	cert, err := tls.LoadX509KeyPair(certPath, keyFile)
	require.NoError(t, err)
	server.TLS = &tls.Config{Certificates: []tls.Certificate{cert}}
	server.StartTLS()
	defer server.Close()

	p := &saClientProvider{
		endpoint:   server.Listener.Addr().String(),
		caCertPath: "./testdata/mismatch.crt",
		tokenPath:  "./testdata/token",
		cfg: &ClientConfig{
			InsecureSkipVerify: false,
		},
		logger: zap.NewNop(),
	}
	client, err := p.BuildClient()
	require.NoError(t, err)
	_, err = client.Get("/")
	require.ErrorContains(t, err, errSignedByUnknownCA)
}

func TestNewKubeConfigClient(t *testing.T) {
	tests := []struct {
		name    string
		cluster string
		context string
	}{
		{
			name:    "current context",
			cluster: "my-cluster-1",
			context: "",
		},
		{
			name:    "override context",
			cluster: "my-cluster-2",
			context: "my-context-2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewUnstartedServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
				// Check if call is authenticated using provided kubeconfig
				assert.Equal(t, "Bearer my-token", req.Header.Get("Authorization"))
				assert.Equal(t, "/api/v1/nodes/nodename/proxy/", req.URL.EscapedPath())
				// Send response to be tested
				_, err := rw.Write([]byte(`OK`))
				assert.NoError(t, err)
			}))
			server.StartTLS()
			defer server.Close()

			kubeConfig, err := clientcmd.LoadFromFile("testdata/kubeconfig")
			require.NoError(t, err)
			kubeConfig.Clusters[tt.cluster].Server = "https://" + server.Listener.Addr().String()
			tempKubeConfig := filepath.Join(t.TempDir(), "kubeconfig")
			require.NoError(t, clientcmd.WriteToFile(*kubeConfig, tempKubeConfig))
			t.Setenv("KUBECONFIG", tempKubeConfig)

			p, err := NewClientProvider("nodename", &ClientConfig{
				APIConfig: k8sconfig.APIConfig{
					AuthType: k8sconfig.AuthTypeKubeConfig,
					Context:  tt.context,
				},
				InsecureSkipVerify: true,
			}, zap.NewNop())
			require.NoError(t, err)
			require.NotNil(t, p)
			client, err := p.BuildClient()
			require.NoError(t, err)
			resp, err := client.Get("/")
			require.NoError(t, err)
			require.Equal(t, []byte(`OK`), resp)
		})
	}
}

func TestBuildEndpoint(t *testing.T) {
	tests := []struct {
		name          string
		endpoint      string
		useSecurePort bool
		wantRegex     string
	}{
		{
			name:          "default secure",
			endpoint:      "",
			useSecurePort: true,
			wantRegex:     `^https://.+:10250$`,
		},
		{
			name:          "default read only",
			endpoint:      "",
			useSecurePort: false,
			wantRegex:     `^http://.+:10255$`,
		},
		{
			name:          "prepended https",
			endpoint:      "hostname:12345",
			useSecurePort: true,
			wantRegex:     `^https://hostname:12345$`,
		},
		{
			name:          "prepended http",
			endpoint:      "hostname:12345",
			useSecurePort: false,
			wantRegex:     `^http://hostname:12345$`,
		},
		{
			name:          "unchanged",
			endpoint:      "https://host.name:12345",
			useSecurePort: true,
			wantRegex:     `^https://host\.name:12345$`,
		},
	}
	for _, tt := range tests {
		got, err := buildEndpoint(tt.endpoint, tt.useSecurePort, zap.NewNop())
		require.NoError(t, err)
		matched, err := regexp.MatchString(tt.wantRegex, got)
		require.NoError(t, err)
		assert.True(t, matched, "endpoint %s doesn't match regexp %v", got, tt.wantRegex)
	}
}

func TestBadAuthType(t *testing.T) {
	_, err := NewClientProvider("foo", &ClientConfig{
		APIConfig: k8sconfig.APIConfig{
			AuthType: "bar",
		},
	}, zap.NewNop())
	require.Error(t, err)
}

func TestTLSMissingCAFile(t *testing.T) {
	p := &tlsClientProvider{
		endpoint: "",
		cfg:      &ClientConfig{},
		logger:   zap.NewNop(),
	}
	_, err := p.BuildClient()
	require.Error(t, err)
}

func TestTLSMissingCertFile(t *testing.T) {
	p := tlsClientProvider{
		endpoint: "",
		cfg: &ClientConfig{
			Config: configtls.Config{
				CAFile: certPath,
			},
		},
		logger: zap.NewNop(),
	}
	_, err := p.BuildClient()
	require.Error(t, err)
}

func TestSABadCertPath(t *testing.T) {
	p := &saClientProvider{
		endpoint:   "foo",
		caCertPath: "bar",
		cfg:        &ClientConfig{},
		tokenPath:  "baz",
		logger:     zap.NewNop(),
	}
	_, err := p.BuildClient()
	require.Error(t, err)
}

func TestSABadTokenPath(t *testing.T) {
	p := &saClientProvider{
		endpoint:   "foo",
		caCertPath: certPath,
		cfg:        &ClientConfig{},
		tokenPath:  "bar",
		logger:     zap.NewNop(),
	}
	_, err := p.BuildClient()
	require.Error(t, err)
}

func TestTLSDefaultEndpoint(t *testing.T) {
	client, err := defaultTLSClient("", true, nil, nil, nil, zap.NewNop())
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(client.baseURL, "https://"))
	require.True(t, strings.HasSuffix(client.baseURL, ":10250"))
}

func TestBuildReq(t *testing.T) {
	p := &saClientProvider{
		endpoint:   "localhost:9876",
		caCertPath: certPath,
		cfg:        &ClientConfig{},
		tokenPath:  "./testdata/token",
		logger:     zap.NewNop(),
	}
	cl, err := p.BuildClient()
	require.NoError(t, err)
	req, err := cl.(*clientImpl).buildReq("/foo")
	require.NoError(t, err)
	require.NotNil(t, req)
}

func TestBuildBadReq(t *testing.T) {
	p := &saClientProvider{
		endpoint:   "[]localhost:9876",
		caCertPath: certPath,
		cfg:        &ClientConfig{},
		tokenPath:  "./testdata/token",
		logger:     zap.NewNop(),
	}
	cl, err := p.BuildClient()
	require.NoError(t, err)
	require.NoError(t, err)
	_, err = cl.(*clientImpl).buildReq("")
	require.Error(t, err)
}

func TestFailedRT(t *testing.T) {
	tr := &fakeRoundTripper{failOnRT: true}
	baseURL := "http://localhost:9876"
	client := &clientImpl{
		baseURL:    baseURL,
		httpClient: http.Client{Transport: tr},
	}
	_, err := client.Get("/foo")
	require.Error(t, err)
}

func TestBadReq(t *testing.T) {
	tr := &fakeRoundTripper{}
	baseURL := "http://[]localhost:9876"
	client := &clientImpl{
		baseURL:    baseURL,
		httpClient: http.Client{Transport: tr},
	}
	_, err := client.Get("")
	require.Error(t, err)
}

func TestErrOnClose(t *testing.T) {
	tr := &fakeRoundTripper{errOnClose: true}
	baseURL := "http://localhost:9876"
	client := &clientImpl{
		baseURL:    baseURL,
		httpClient: http.Client{Transport: tr},
		logger:     zap.NewNop(),
	}
	resp, err := client.Get("/foo")
	require.NoError(t, err)
	require.NotNil(t, resp)
}

func TestErrOnRead(t *testing.T) {
	tr := &fakeRoundTripper{errOnRead: true}
	baseURL := "http://localhost:9876"
	client := &clientImpl{
		baseURL:    baseURL,
		httpClient: http.Client{Transport: tr},
		logger:     zap.NewNop(),
	}
	resp, err := client.Get("/foo")
	require.Error(t, err)
	require.Nil(t, resp)
}

func TestErrCode(t *testing.T) {
	tr := &fakeRoundTripper{errCode: true}
	baseURL := "http://localhost:9876"
	client := &clientImpl{
		baseURL:    baseURL,
		httpClient: http.Client{Transport: tr},
		logger:     zap.NewNop(),
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
		return nil, errors.New("failOnRT == true")
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
					return errors.New("")
				}
				return nil
			},
		},
	}, nil
}

var _ io.Reader = (*failingReader)(nil)

type failingReader struct{}

func (f *failingReader) Read([]byte) (n int, err error) {
	return 0, errors.New("")
}

var _ io.ReadCloser = (*fakeReadCloser)(nil)

type fakeReadCloser struct {
	io.Reader
	onClose func() error
}

func (f *fakeReadCloser) Close() error {
	return f.onClose()
}
