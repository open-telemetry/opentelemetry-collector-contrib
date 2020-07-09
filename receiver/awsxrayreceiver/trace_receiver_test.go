// Copyright The OpenTelemetry Authors
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

package awsxrayreceiver

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/testbed/testbed"
	"net/http"

	"go.uber.org/atomic"
	"go.uber.org/zap"
	"io/ioutil"
	"testing"
	"time"
)

func TestReception(t *testing.T) {
	body, err := ioutil.ReadFile("./testdata/sample.json")
	require.NoError(t, err)

	port := testbed.GetAvailablePort(t)

	next := &awsxrayMockTraceConsumer{
		spansReceived: atomic.Uint64{},
		err:           errors.New("consumer error"),
	}

	cfg := &Config{
		Endpoint: fmt.Sprintf("localhost:%d", port),
		TLSCredentials: &configtls.TLSSetting{
			CertFile: "./server.crt",
			KeyFile:  "./server.key",
		},
	}
	ar := setupReceiver(t, cfg, next)

	defer ar.Shutdown(context.Background())

	t.Log("Sending Xray Request")

	var resp *http.Response
	resp, err = sendReq(cfg.Endpoint, body, true)

	require.NoErrorf(t, err, "Failed when sending trace %v", err)

	assert.Equal(t, 200, resp.StatusCode)
}

type awsxrayMockTraceConsumer struct {
	spansReceived atomic.Uint64
	err           error
}

func (tc *awsxrayMockTraceConsumer) ConsumeTestbedTraces(spansCount int) error {
	tc.spansReceived.Add(uint64(spansCount))
	return nil
}

func setupReceiver(t *testing.T, config *Config, next *awsxrayMockTraceConsumer) component.TraceReceiver {
	params := component.ReceiverCreateParams{Logger: zap.L()}
	ar, err := New(next, params, config)
	assert.NoError(t, err, "failed to create the AWSXray receiver")
	t.Log("Starting")

	// NewNopHost swallows errors so using NewErrorWaitingHost to catch any potential errors starting the
	// receiver.
	mh := componenttest.NewErrorWaitingHost()
	require.NoError(t, ar.Start(context.Background(), mh), "failed to start trace reception")

	// If there are errors reported through host.ReportFatalError() this will retrieve it.
	receivedError, receivedErr := mh.WaitForFatalError(500 * time.Millisecond)
	require.NoError(t, receivedErr, "failed to start trace reception")
	require.False(t, receivedError)
	t.Log("Trace Reception Started")
	return ar
}

func sendReq(endpoint string, body []byte, tlsEnabled bool) (*http.Response, error) {

	// build the request
	url := fmt.Sprintf("http://%s%s", endpoint, "/TraceSegments")
	if tlsEnabled {
		url = fmt.Sprintf("https://%s%s", endpoint, "/TraceSegments")
	}
	req, _ := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	req.Header.Set("content-type", "application/json")

	// send the request
	client := &http.Client{}

	if tlsEnabled {
		caCert, errCert := ioutil.ReadFile("./testdata/testcert.crt")
		if errCert != nil {
			return nil, fmt.Errorf("failed to load certificate: %s", errCert.Error())
		}
		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(caCert)
		client.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs: caCertPool,
			},
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		return resp, fmt.Errorf("failed to send request to receiver %v", err)
	}

	return resp, nil
}
