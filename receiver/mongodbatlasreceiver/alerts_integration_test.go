// Copyright  The OpenTelemetry Authors
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

//go:build integration

package mongodbatlasreceiver

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha1" // #nosec G505 -- SHA1 is the algorithm mongodbatlas uses, it must be used to calculate the HMAC signature
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap/zaptest"
)

var testPayloads = []string{
	"metric-threshold-closed.json",
	"new-primary.json",
}

const (
	testPort   = 14732
	testSecret = "some_secret"
)

func TestAlertsReceiver(t *testing.T) {
	for _, payloadName := range testPayloads {
		t.Run(payloadName, func(t *testing.T) {
			sink := &consumertest.LogsSink{}
			ar, err := newAlertsReceiver(zaptest.NewLogger(t), AlertConfig{
				Enabled:  true,
				Secret:   testSecret,
				Endpoint: fmt.Sprintf("127.0.0.1:%d", testPort),
			}, sink)

			require.NoError(t, err)

			err = ar.Start(context.Background(), componenttest.NewNopHost())
			require.NoError(t, err)

			defer func() {
				assert.NoError(t, ar.Shutdown(context.Background()))
			}()

			payloadFile, err := os.Open(filepath.Join("testdata", "alerts", "sample-payloads", payloadName))
			require.NoError(t, err)
			defer payloadFile.Close()

			payload, err := io.ReadAll(payloadFile)
			require.NoError(t, err)

			req, err := http.NewRequest("POST", fmt.Sprintf("http://127.0.0.1:%d", testPort), bytes.NewBuffer(payload))
			require.NoError(t, err)

			b64HMAC, err := calculateHMACb64(testSecret, payload)
			require.NoError(t, err)

			req.Header.Add(signatureHeaderName, b64HMAC)

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)

			defer resp.Body.Close()

			require.Equal(t, resp.StatusCode, http.StatusOK)

			require.Eventually(t, func() bool {
				return sink.LogRecordCount() > 0
			}, 10*time.Second, 10*time.Millisecond)

			logs := sink.AllLogs()[0]

			expectedLogs, err := readLogs(filepath.Join("testdata", "alerts", "golden", payloadName))
			require.NoError(t, err)

			require.NoError(t, compareLogs(*expectedLogs, logs))
		})
	}
}

func TestAlertsReceiverTLS(t *testing.T) {
	for _, payloadName := range testPayloads {
		t.Run(payloadName, func(t *testing.T) {
			sink := &consumertest.LogsSink{}
			ar, err := newAlertsReceiver(zaptest.NewLogger(t), AlertConfig{
				Enabled:  true,
				Secret:   testSecret,
				Endpoint: fmt.Sprintf("127.0.0.1:%d", testPort),
				TLS: &configtls.TLSServerSetting{
					TLSSetting: configtls.TLSSetting{
						CertFile: filepath.Join("testdata", "alerts", "cert", "cert.pem"),
						KeyFile:  filepath.Join("testdata", "alerts", "cert", "key.pem"),
					},
				},
			}, sink)

			require.NoError(t, err)

			err = ar.Start(context.Background(), componenttest.NewNopHost())
			require.NoError(t, err)

			defer func() {
				assert.NoError(t, ar.Shutdown(context.Background()))
			}()

			payloadFile, err := os.Open(filepath.Join("testdata", "alerts", "sample-payloads", payloadName))
			require.NoError(t, err)
			defer payloadFile.Close()

			payload, err := io.ReadAll(payloadFile)
			require.NoError(t, err)

			req, err := http.NewRequest("POST", fmt.Sprintf("https://127.0.0.1:%d", testPort), bytes.NewBuffer(payload))
			require.NoError(t, err)

			b64HMAC, err := calculateHMACb64(testSecret, payload)
			require.NoError(t, err)

			req.Header.Add(signatureHeaderName, b64HMAC)

			client, err := clientWithCert(filepath.Join("testdata", "alerts", "cert", "cert.pem"))
			require.NoError(t, err)

			resp, err := client.Do(req)
			require.NoError(t, err)

			defer resp.Body.Close()

			require.Equal(t, resp.StatusCode, http.StatusOK)

			require.Eventually(t, func() bool {
				return sink.LogRecordCount() > 0
			}, 10*time.Second, 10*time.Millisecond)

			logs := sink.AllLogs()[0]

			expectedLogs, err := readLogs(filepath.Join("testdata", "alerts", "golden", payloadName))
			require.NoError(t, err)

			require.NoError(t, compareLogs(*expectedLogs, logs))
		})
	}
}

func calculateHMACb64(secret string, payload []byte) (string, error) {
	h := hmac.New(sha1.New, []byte(secret))
	h.Write(payload)
	b := h.Sum(nil)

	var buf bytes.Buffer
	enc := base64.NewEncoder(base64.StdEncoding, &buf)
	_, err := enc.Write(b)
	if err != nil {
		return "", err
	}

	err = enc.Close()
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}

func readLogs(path string) (*plog.Logs, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	b, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}

	logs, err := plog.NewJSONUnmarshaler().UnmarshalLogs(b)
	return &logs, err
}

func clientWithCert(path string) (*http.Client, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	b, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}

	roots := x509.NewCertPool()
	ok := roots.AppendCertsFromPEM(b)
	if !ok {
		return nil, errors.New("failed to append certficate as root certificate")
	}

	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs: roots,
			},
		},
	}, nil
}
