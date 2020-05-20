// Copyright 2020, OpenTelemetry Authors
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

package kubelet

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"

	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/k8sconfig"
)

// Config for a kubelet Client. Mostly for talking to the kubelet HTTP endpoint.
type ClientConfig struct {
	k8sconfig.APIConfig `mapstructure:",squash"`
	// Path to the CA cert. For a client this verifies the server certificate.
	// For a server this verifies client certificates. If empty uses system root CA.
	// (optional)
	CAFile string `mapstructure:"ca_file"`
	// Path to the TLS cert to use for TLS required connections. (optional)
	CertFile string `mapstructure:"cert_file"`
	// Path to the TLS key to use for TLS required connections. (optional)
	// TODO replace with open-telemetry/opentelemetry-collector#933 when done
	KeyFile string `mapstructure:"key_file"`
	// InsecureSkipVerify controls whether the client verifies the server's
	// certificate chain and host name.
	InsecureSkipVerify bool `mapstructure:"insecure_skip_verify"`
}

// A kubelet client, which mostly handles auth when talking to a kubelet server.
// Marshaling/unmarshaling should be performed by the caller.
type Client struct {
	c        *http.Client
	endpoint string
	token    string
	logger   *zap.Logger
}

func NewClient(endpoint string, cfg *ClientConfig, logger *zap.Logger) (*Client, error) {
	var client *Client
	var err error
	switch cfg.AuthType {
	case k8sconfig.AuthTypeTLS:
		client, err = tlsClient(endpoint, cfg)
	case k8sconfig.AuthTypeServiceAccount:
		client, err = serviceAccountClient(endpoint)
	default:
		return nil, errors.New("cfg.AuthType not found")
	}
	if err != nil {
		return nil, err
	}
	client.logger = logger
	return client, nil
}

func tlsClient(endpoint string, cfg *ClientConfig) (*Client, error) {
	rootCAs, err := systemCertPoolPlusPath(cfg.CAFile)
	if err != nil {
		return nil, err
	}

	clientCert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
	if err != nil {
		return nil, err
	}

	tr := defaultTransport()
	tr.TLSClientConfig = &tls.Config{
		RootCAs:            rootCAs,
		Certificates:       []tls.Certificate{clientCert},
		InsecureSkipVerify: cfg.InsecureSkipVerify,
	}
	return &Client{
		c:        &http.Client{Transport: tr},
		endpoint: endpoint,
	}, nil
}

func serviceAccountClient(endpoint string) (*Client, error) {
	const svcAcctTokenPath = "/var/run/secrets/kubernetes.io/serviceaccount/token"
	token, err := ioutil.ReadFile(svcAcctTokenPath)
	if err != nil {
		return nil, err
	}
	const caCertPath = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
	rootCAs, err := systemCertPoolPlusPath(caCertPath)
	if err != nil {
		return nil, err
	}

	tr := defaultTransport()
	tr.TLSClientConfig = &tls.Config{
		RootCAs: rootCAs,
	}
	return &Client{
		c:        &http.Client{Transport: tr},
		endpoint: endpoint,
		token:    string(token),
	}, nil
}

func defaultTransport() *http.Transport {
	return http.DefaultTransport.(*http.Transport).Clone()
}

func systemCertPoolPlusPath(certPath string) (*x509.CertPool, error) {
	sysCerts, err := systemCertPool()
	if err != nil {
		return nil, err
	}
	return certPoolPlusPath(sysCerts, certPath)
}

func certPoolPlusPath(certPool *x509.CertPool, certPath string) (*x509.CertPool, error) {
	certBytes, err := ioutil.ReadFile(certPath)
	if err != nil {
		return nil, err
	}
	ok := certPool.AppendCertsFromPEM(certBytes)
	if !ok {
		return nil, errors.New("AppendCertsFromPEM failed")
	}
	return certPool, nil
}

func systemCertPool() (*x509.CertPool, error) {
	return x509.SystemCertPool()
}

func (c *Client) Get(path string) ([]byte, error) {
	return c.request("GET", path, nil)
}

func (c *Client) Post(path string, marshaled []byte) ([]byte, error) {
	var reader *bytes.Buffer
	if marshaled != nil {
		reader = bytes.NewBuffer(marshaled)
	}
	return c.request("POST", path, reader)
}

func (c *Client) request(method string, path string, reader io.Reader) ([]byte, error) {
	c.logger.Debug(
		"kubelet request",
		zap.String("method", method),
		zap.String("path", path),
		zap.Int("token length", len(c.token)),
	)

	url := "https://" + c.endpoint + path
	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("bearer %s", c.token))
	}
	resp, err := c.c.Do(req)
	if err != nil {
		return nil, err
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}
