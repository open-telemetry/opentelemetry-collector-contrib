// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration

package cloudflarereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/cloudflarereceiver"

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
)

const (
	testSecret = "test-secret"
)

var testPayloads = []string{
	"all_fields",
	"multiple_log_payload",
}

func TestReceiverTLS(t *testing.T) {
	for _, payloadName := range testPayloads {
		t.Run(payloadName, func(t *testing.T) {
			testAddr := testutil.GetAvailableLocalAddress(t)
			sink := &consumertest.LogsSink{}
			fact := NewFactory()

			_, testPort, err := net.SplitHostPort(testAddr)
			require.NoError(t, err)

			recv, err := fact.CreateLogsReceiver(
				context.Background(),
				receivertest.NewNopCreateSettings(),
				&Config{
					Logs: LogsConfig{
						Secret:   testSecret,
						Endpoint: testAddr,
						TLS: &configtls.TLSServerSetting{
							TLSSetting: configtls.TLSSetting{
								CertFile: filepath.Join("testdata", "cert", "server.crt"),
								KeyFile:  filepath.Join("testdata", "cert", "server.key"),
							},
						},
						TimestampField: "EdgeStartTimestamp",
						Attributes: map[string]string{
							"ClientIP": "http_request.client_ip",
						},
					},
				},
				sink,
			)
			require.NoError(t, err)

			err = recv.Start(context.Background(), componenttest.NewNopHost())
			require.NoError(t, err)

			defer func() {
				require.NoError(t, recv.Shutdown(context.Background()))
			}()

			payload, err := os.ReadFile(filepath.Join("testdata", "sample-payloads", fmt.Sprintf("%s.txt", payloadName)))
			require.NoError(t, err)

			req, err := http.NewRequest("POST", fmt.Sprintf("https://localhost:%s", testPort), bytes.NewBuffer(payload))
			require.NoError(t, err)

			client, err := clientWithCert(filepath.Join("testdata", "cert", "ca.crt"))
			require.NoError(t, err)

			// try first without secret to see failure
			resp, err := client.Do(req)
			require.NoError(t, err)
			require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
			resp.Body.Close()

			// add header to see success
			req.Header.Add(secretHeaderName, testSecret)
			resp, err = client.Do(req)
			require.NoError(t, err)
			resp.Body.Close()

			require.Equal(t, http.StatusOK, resp.StatusCode)

			require.Eventually(t, func() bool {
				return sink.LogRecordCount() > 0
			}, 2*time.Second, 10*time.Millisecond)

			logs := sink.AllLogs()[0]

			expectedLogs, err := golden.ReadLogs(filepath.Join("testdata", "processed", fmt.Sprintf("%s.json", payloadName)))
			require.NoError(t, err)

			require.NoError(t, plogtest.CompareLogs(expectedLogs, logs, plogtest.IgnoreObservedTimestamp()))
		})
	}
}

func clientWithCert(path string) (*http.Client, error) {
	b, err := os.ReadFile(filepath.Clean(path))
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
