// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kubelet // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/kubelet"

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"go.uber.org/zap"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/transport"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/sanitize"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
)

const (
	svcAcctCACertPath   = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
	svcAcctTokenPath    = "/var/run/secrets/kubernetes.io/serviceaccount/token" // #nosec
	defaultSecurePort   = "10250"
	defaultReadOnlyPort = "10255"
)

type Client interface {
	Get(path string) ([]byte, error)
}

func NewClientProvider(endpoint string, cfg *ClientConfig, logger *zap.Logger) (ClientProvider, error) {
	switch cfg.AuthType {
	case k8sconfig.AuthTypeTLS:
		return &tlsClientProvider{
			endpoint: endpoint,
			cfg:      cfg,
			logger:   logger,
		}, nil
	case k8sconfig.AuthTypeServiceAccount:
		return &saClientProvider{
			endpoint:           endpoint,
			caCertPath:         svcAcctCACertPath,
			tokenPath:          svcAcctTokenPath,
			insecureSkipVerify: cfg.InsecureSkipVerify,
			logger:             logger,
		}, nil
	case k8sconfig.AuthTypeNone:
		return &readOnlyClientProvider{
			endpoint: endpoint,
			logger:   logger,
		}, nil
	case k8sconfig.AuthTypeKubeConfig:
		return &kubeConfigClientProvider{
			endpoint: endpoint,
			cfg:      cfg,
			logger:   logger,
		}, nil
	default:
		return nil, fmt.Errorf("AuthType [%s] not supported", cfg.AuthType)
	}
}

type ClientProvider interface {
	BuildClient() (Client, error)
}

type kubeConfigClientProvider struct {
	endpoint string
	cfg      *ClientConfig
	logger   *zap.Logger
}

func (p *kubeConfigClientProvider) BuildClient() (Client, error) {
	authConf, err := k8sconfig.CreateRestConfig(p.cfg.APIConfig)
	if err != nil {
		return nil, err
	}
	if p.cfg.InsecureSkipVerify {
		// Override InsecureSkipVerify from kubeconfig
		authConf.CAFile = ""
		authConf.CAData = nil
		authConf.Insecure = true
	}

	client, err := rest.HTTPClientFor(authConf)
	if err != nil {
		return nil, err
	}

	joinPath, err := url.JoinPath(authConf.Host, "/api/v1/nodes/", p.endpoint, "/proxy/")
	if err != nil {
		return nil, err
	}
	return &clientImpl{
		baseURL:    joinPath,
		httpClient: *client,
		tok:        nil,
		logger:     p.logger,
	}, nil
}

type readOnlyClientProvider struct {
	endpoint string
	logger   *zap.Logger
}

func (p *readOnlyClientProvider) BuildClient() (Client, error) {
	tr := defaultTransport()
	endpoint, err := buildEndpoint(p.endpoint, false, p.logger)
	if err != nil {
		return nil, err
	}
	return &clientImpl{
		baseURL:    endpoint,
		httpClient: http.Client{Transport: tr},
		tok:        nil,
		logger:     p.logger,
	}, nil
}

type tlsClientProvider struct {
	endpoint string
	cfg      *ClientConfig
	logger   *zap.Logger
}

func (p *tlsClientProvider) BuildClient() (Client, error) {
	rootCAs, err := systemCertPoolPlusPath(p.cfg.CAFile)
	if err != nil {
		return nil, err
	}
	clientCert, err := tls.LoadX509KeyPair(p.cfg.CertFile, p.cfg.KeyFile)
	if err != nil {
		return nil, err
	}
	return defaultTLSClient(
		p.endpoint,
		p.cfg.InsecureSkipVerify,
		rootCAs,
		[]tls.Certificate{clientCert},
		nil,
		p.logger,
	)
}

type saClientProvider struct {
	endpoint           string
	caCertPath         string
	tokenPath          string
	insecureSkipVerify bool
	logger             *zap.Logger
}

func (p *saClientProvider) BuildClient() (Client, error) {
	rootCAs, err := systemCertPoolPlusPath(p.caCertPath)
	if err != nil {
		return nil, err
	}
	tok, err := os.ReadFile(p.tokenPath)
	if err != nil {
		return nil, fmt.Errorf("unable to read token file %s: %w", p.tokenPath, err)
	}
	tr := defaultTransport()
	tr.TLSClientConfig = &tls.Config{
		RootCAs:            rootCAs,
		InsecureSkipVerify: p.insecureSkipVerify,
	}
	endpoint, err := buildEndpoint(p.endpoint, true, p.logger)
	if err != nil {
		return nil, err
	}
	rt, err := transport.NewBearerAuthWithRefreshRoundTripper(string(tok), p.tokenPath, tr)
	if err != nil {
		return nil, err
	}

	return &clientImpl{
		baseURL: endpoint,
		httpClient: http.Client{
			Transport: rt,
		},
		tok:    nil,
		logger: p.logger,
	}, nil
}

func defaultTLSClient(
	endpoint string,
	insecureSkipVerify bool,
	rootCAs *x509.CertPool,
	certificates []tls.Certificate,
	tok []byte,
	logger *zap.Logger,
) (*clientImpl, error) {
	tr := defaultTransport()
	tr.TLSClientConfig = &tls.Config{
		RootCAs:            rootCAs,
		Certificates:       certificates,
		InsecureSkipVerify: insecureSkipVerify,
	}
	endpoint, err := buildEndpoint(endpoint, true, logger)
	if err != nil {
		return nil, err
	}
	return &clientImpl{
		baseURL:    endpoint,
		httpClient: http.Client{Transport: tr},
		tok:        tok,
		logger:     logger,
	}, nil
}

// buildEndpoint builds a kubelet endpoint based on value provided by user and whether secure or read-only endpoint
// should be used.
func buildEndpoint(endpoint string, useSecurePort bool, logger *zap.Logger) (string, error) {
	if endpoint == "" {
		// This will work if hostNetwork is turned on, in which case the pod has access
		// to the node's loopback device.
		// https://kubernetes.io/docs/concepts/policy/pod-security-policy/#host-namespaces
		host, err := os.Hostname()
		if err != nil {
			return "", fmt.Errorf("unable to get hostname for default endpoint: %w", err)
		}

		if useSecurePort {
			endpoint = fmt.Sprintf("https://%s:%s", host, defaultSecurePort)
		} else {
			endpoint = fmt.Sprintf("http://%s:%s", host, defaultReadOnlyPort)
		}
		logger.Warn("Kubelet endpoint not defined, using default endpoint " + endpoint)
		return endpoint, nil
	}

	if !strings.HasPrefix(endpoint, "http://") && !strings.HasPrefix(endpoint, "https://") {
		if useSecurePort {
			return "https://" + endpoint, nil
		}
		return "http://" + endpoint, nil
	}

	return endpoint, nil
}

func defaultTransport() *http.Transport {
	return http.DefaultTransport.(*http.Transport).Clone()
}

// clientImpl

var _ Client = (*clientImpl)(nil)

type clientImpl struct {
	baseURL    string
	httpClient http.Client
	logger     *zap.Logger
	tok        []byte
}

func (c *clientImpl) Get(path string) ([]byte, error) {
	req, err := c.buildReq(path)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		closeErr := resp.Body.Close()
		if closeErr != nil {
			c.logger.Warn("failed to close response body", zap.Error(closeErr))
		}
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read Kubelet response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("kubelet request GET %s failed - %q, response: %q",
			sanitize.URL(req.URL), resp.Status, string(body))
	}

	return body, nil
}

func (c *clientImpl) buildReq(p string) (*http.Request, error) {
	reqURL, err := url.JoinPath(c.baseURL, p)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.tok != nil {
		req.Header.Set("Authorization", fmt.Sprintf("bearer %s", c.tok))
	}
	return req, nil
}
