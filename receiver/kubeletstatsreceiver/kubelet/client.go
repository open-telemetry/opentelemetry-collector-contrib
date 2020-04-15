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
	"os"

	"go.uber.org/zap"
)

type AuthType string

const (
	// AuthTypeTLS indicates that client TLS auth is desired
	AuthTypeTLS AuthType = "tls"
	// AuthTypeServiceAccount indicates that the default service account token should be used
	AuthTypeServiceAccount AuthType = "serviceAccount"
)

// Config for a kubelet Client. Mostly for talking to the kubelet HTTP endpoint.
type ClientConfig struct {
	// AuthType should be "tls" or "serviceAccount". TLS requires that the cert/key
	// paths below also be set.
	AuthType AuthType `mapstructure:"auth_type"`
	// CacertPath should be the path to the root CAs. Used to verify the kubelet
	// server's certificates.
	CacertPath string `mapstructure:"ca_cert_path"`
	// ClientKeyPath should be the path to the TLS client cert. Presented to the
	// other side of the kubelet connection.
	ClientCertPath string `mapstructure:"client_cert_path"`
	// ClientKeyPath should be the path to the TLS client key. Must correspond to
	// the ClientCertPath, making a key pair to present to the kubelet.
	ClientKeyPath string `mapstructure:"client_key_path"`
	// InsecureSkipVerify controls whether the client verifies the server's
	// certificate chain and host name.
	InsecureSkipVerify bool `mapstructure:"insecure_skip_verify"`
}

// A kubelet client, which mostly handles auth when talking to a kubelet server.
// Marshaling/unmarshaling should be performed by the caller.
type Client struct {
	c       *http.Client
	baseURL string
	token   string
	logger  *zap.Logger
}

func NewClient(baseURL string, cfg *ClientConfig, logger *zap.Logger) (*Client, error) {
	var client *Client
	var err error
	switch cfg.AuthType {
	case AuthTypeTLS:
		client, err = tlsClient(baseURL, cfg)
	case AuthTypeServiceAccount:
		client, err = serviceAccountClient(baseURL)
	default:
		return nil, errors.New("cfg.AuthType not found")
	}
	if err != nil {
		return nil, err
	}
	if baseURL == "" {
		// this requires HostNetwork to be turned on
		hostname, err := os.Hostname()
		if err != nil {
			return nil, err
		}
		client.baseURL = fmt.Sprintf("http://%s:10250", hostname)
	} else {
		client.baseURL = baseURL
	}
	client.logger = logger
	return client, nil
}

func tlsClient(baseURL string, cfg *ClientConfig) (*Client, error) {
	rootCAs, err := systemCertPoolPlusPath(cfg.CacertPath)
	if err != nil {
		return nil, err
	}

	clientCert, err := tls.LoadX509KeyPair(cfg.ClientCertPath, cfg.ClientKeyPath)
	if err != nil {
		return nil, err
	}

	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.TLSClientConfig = &tls.Config{
		RootCAs:            rootCAs,
		Certificates:       []tls.Certificate{clientCert},
		InsecureSkipVerify: cfg.InsecureSkipVerify,
	}

	return &Client{
		c:       &http.Client{Transport: tr},
		baseURL: baseURL,
	}, nil
}

func serviceAccountClient(baseURL string) (*Client, error) {
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
	return &Client{
		c: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					RootCAs: rootCAs,
				},
			},
		},
		baseURL: baseURL,
		token:   string(token),
	}, nil
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
	// todo windows
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
	req, err := http.NewRequest(method, c.baseURL+path, reader)
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
