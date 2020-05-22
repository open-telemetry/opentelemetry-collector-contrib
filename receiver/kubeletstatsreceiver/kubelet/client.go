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
	"crypto/tls"
	"crypto/x509"
	"fmt"
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

type Client interface {
	Get(path string) ([]byte, error)
}

func NewClient(endpoint string, cfg *ClientConfig, logger *zap.Logger) (Client, error) {
	if cfg.APIConfig.AuthType != k8sconfig.AuthTypeTLS {
		return nil, fmt.Errorf("AuthType [%s] not supported", cfg.APIConfig.AuthType)
	}
	return newTLSClient(endpoint, cfg, logger)
}

// not unit tested
func newTLSClient(endpoint string, cfg *ClientConfig, logger *zap.Logger) (Client, error) {
	rootCAs, err := systemCertPoolPlusPath(cfg.CAFile)
	if err != nil {
		return nil, err
	}
	clientCert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
	if err != nil {
		return nil, err
	}
	return defaultTLSClient(endpoint, cfg.InsecureSkipVerify, rootCAs, clientCert, logger), nil
}

func defaultTLSClient(endpoint string, insecureSkipVerify bool, rootCAs *x509.CertPool, clientCert tls.Certificate, logger *zap.Logger) *tlsClient {
	tr := defaultTransport()
	tr.TLSClientConfig = &tls.Config{
		RootCAs:            rootCAs,
		Certificates:       []tls.Certificate{clientCert},
		InsecureSkipVerify: insecureSkipVerify,
	}
	return &tlsClient{
		baseURL:    "https://" + endpoint,
		httpClient: http.Client{Transport: tr},
		logger:     logger,
	}
}

func defaultTransport() *http.Transport {
	return http.DefaultTransport.(*http.Transport).Clone()
}

// tlsClient

var _ Client = (*tlsClient)(nil)

type tlsClient struct {
	baseURL    string
	httpClient http.Client
	logger     *zap.Logger
}

func (c *tlsClient) Get(path string) ([]byte, error) {
	url := c.baseURL + path
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
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
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}
