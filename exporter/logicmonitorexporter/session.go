// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package logicmonitorexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/logicmonitorexporter"

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	httpPrefix = "https://"
	platformEP = ".logicmonitor.com"
)

type accountInfo interface {
	name() string
	id() string
	token(method string, payload []byte, uri string) string
}

type lmv1Account struct {
	accountName string
	accessID    string
	accessKey   string
}

func (a lmv1Account) name() string {
	return a.accountName
}

func (a lmv1Account) id() string {
	return a.accessID
}

func (a lmv1Account) token(method string, data []byte, uri string) string {
	return generateLMv1Token(method, a.accessID, a.accessKey, data, uri).String()
}

type bearerAccount struct {
	accountName string
	key         string
}

func (b bearerAccount) name() string {
	return b.accountName
}

func (b bearerAccount) id() string {
	return strings.Split(b.key, ":")[0]
}

func (b bearerAccount) token(method string, data []byte, uri string) string {
	return "Bearer " + b.key
}

type HTTPClient interface {
	MakeRequest(ctx context.Context, version, method, baseURI, uri, configURL string, timeout time.Duration, pBytes *bytes.Buffer, headers map[string]string) (*APIResponse, error)
	GetContent(url string) (*http.Response, error)
}

// LMhttpClient http client builder for LogicMonitor
type LMhttpClient struct {
	client *http.Client
	aInfo  accountInfo
}

type APIResponse struct {
	Body          []byte
	Headers       http.Header
	StatusCode    int
	ContentLength int64
}

// NewLMHTTPClient returns HttpClient and bearer/lmv1 account
func NewLMHTTPClient(apitoken, headers map[string]string) HTTPClient {
	var aInfo accountInfo
	account := os.Getenv("LOGICMONITOR_ACCOUNT")
	if account == "" {
		log.Fatal("LOGICMONITOR_ACCOUNT environment not set")
	}

	// Fetches token from config file, if present
	// OR reads token from environment variable
	authToken := getToken(apitoken, headers)
	if authToken.accessID != "" && authToken.accessKey != "" {
		aInfo = lmv1Account{accountName: account, accessID: authToken.accessID, accessKey: authToken.accessKey}
	} else if authToken.bearerToken != "" {
		aInfo = bearerAccount{accountName: account, key: authToken.bearerToken}
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: false, MinVersion: tls.VersionTLS12}
	clientTransport := (http.RoundTripper)(transport)
	httpClient := &http.Client{Transport: clientTransport, Timeout: 0}
	return &LMhttpClient{httpClient, aInfo}
}

// MakeRequest over LMHTTPClient
func (c *LMhttpClient) MakeRequest(ctx context.Context, version, method, baseURI, uri, configURL string, timeout time.Duration, pBytes *bytes.Buffer, headers map[string]string) (*APIResponse, error) {
	if c.client == nil {
		log.Fatal("Session not established")
	}
	var req *http.Request
	var err error
	var fullURL string
	var account string

	if c.aInfo == nil {
		account = os.Getenv("LOGICMONITOR_ACCOUNT")
	} else {
		account = c.aInfo.name()
	}
	if configURL != "" {
		fullURL = configURL + uri
	} else {
		fullURL = httpPrefix + account + platformEP + baseURI + uri
	}

	if pBytes == nil {
		pBytes = bytes.NewBuffer(nil)
	}
	req, err = http.NewRequest(method, fullURL, pBytes)
	if err != nil {
		return nil, fmt.Errorf("creation of request failed with error %w", err)
	}

	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req = req.WithContext(reqCtx)
	req.Header.Set("X-version", version)
	req.Header.Set("x-logicmonitor-account", account)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if c.aInfo != nil {
		if method == http.MethodPost && pBytes != nil {
			req.Header.Set("Authorization", c.aInfo.token(method, pBytes.Bytes(), uri))
		} else {
			req.Header.Set("Authorization", c.aInfo.token(method, nil, uri))
		}
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sending request to %s failed with error %w", fullURL, err)
	}

	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response %s, failed with error %w", fullURL, err)
	}
	apiResp := APIResponse{body, resp.Header, resp.StatusCode, resp.ContentLength}
	return &apiResp, nil
}

// GetContent downloads the content from specified url
func (c *LMhttpClient) GetContent(url string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	return c.client.Do(req)
}
