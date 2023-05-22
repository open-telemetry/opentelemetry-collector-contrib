// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

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
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/atlas/mongodbatlas"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
)

var testPayloads = []string{
	"metric-threshold-closed.yaml",
	"new-primary.yaml",
}

const (
	testSecret = "some_secret"
)

func TestAlertsReceiver(t *testing.T) {
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
					Alerts: AlertConfig{
						Enabled:  true,
						Mode:     alertModeListen,
						Secret:   testSecret,
						Endpoint: testAddr,
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

			payload, err := os.ReadFile(filepath.Join("testdata", "alerts", "sample-payloads", payloadName))
			require.NoError(t, err)

			req, err := http.NewRequest("POST", fmt.Sprintf("http://localhost:%s", testPort), bytes.NewBuffer(payload))
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
			}, 2*time.Second, 10*time.Millisecond)

			logs := sink.AllLogs()[0]

			expectedLogs, err := golden.ReadLogs(filepath.Join("testdata", "alerts", "golden", payloadName))
			require.NoError(t, err)

			require.NoError(t, plogtest.CompareLogs(expectedLogs, logs, plogtest.IgnoreObservedTimestamp()))
		})
	}
}

func TestAlertsReceiverTLS(t *testing.T) {
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
					Alerts: AlertConfig{
						Enabled:  true,
						Secret:   testSecret,
						Mode:     alertModeListen,
						Endpoint: testAddr,
						TLS: &configtls.TLSServerSetting{
							TLSSetting: configtls.TLSSetting{
								CertFile: filepath.Join("testdata", "alerts", "cert", "server.crt"),
								KeyFile:  filepath.Join("testdata", "alerts", "cert", "server.key"),
							},
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

			payload, err := os.ReadFile(filepath.Join("testdata", "alerts", "sample-payloads", payloadName))
			require.NoError(t, err)

			req, err := http.NewRequest("POST", fmt.Sprintf("https://localhost:%s", testPort), bytes.NewBuffer(payload))
			require.NoError(t, err)

			b64HMAC, err := calculateHMACb64(testSecret, payload)
			require.NoError(t, err)

			req.Header.Add(signatureHeaderName, b64HMAC)

			client, err := clientWithCert(filepath.Join("testdata", "alerts", "cert", "ca.crt"))
			require.NoError(t, err)

			resp, err := client.Do(req)
			require.NoError(t, err)

			defer resp.Body.Close()

			require.Equal(t, resp.StatusCode, http.StatusOK)

			require.Eventually(t, func() bool {
				return sink.LogRecordCount() > 0
			}, 2*time.Second, 10*time.Millisecond)

			logs := sink.AllLogs()[0]

			expectedLogs, err := golden.ReadLogs(filepath.Join("testdata", "alerts", "golden", payloadName))
			require.NoError(t, err)

			require.NoError(t, plogtest.CompareLogs(expectedLogs, logs, plogtest.IgnoreObservedTimestamp()))
		})
	}
}

func TestAtlasPoll(t *testing.T) {
	mockClient := mockAlertsClient{}

	alerts := []mongodbatlas.Alert{}
	for _, pl := range testPayloads {
		payloadFile, err := os.ReadFile(filepath.Join("testdata", "alerts", "sample-payloads", pl))
		require.NoError(t, err)

		alert := mongodbatlas.Alert{}
		err = json.Unmarshal(payloadFile, &alert)
		require.NoError(t, err)

		alerts = append(alerts, alert)
	}

	mockClient.On("GetProject", mock.Anything, testProjectName).Return(&mongodbatlas.Project{
		ID:    testProjectID,
		Name:  testProjectName,
		OrgID: testOrgID,
	}, nil)
	mockClient.On("GetAlerts", mock.Anything, testProjectID, mock.Anything).Return(alerts, false, nil)

	sink := &consumertest.LogsSink{}
	fact := NewFactory()

	recv, err := fact.CreateLogsReceiver(
		context.Background(),
		receivertest.NewNopCreateSettings(),
		&Config{
			Alerts: AlertConfig{
				Enabled: true,
				Mode:    alertModePoll,
				Projects: []*ProjectConfig{
					{
						Name: testProjectName,
					},
				},
				PollInterval: 1 * time.Second,
				PageSize:     defaultAlertsPageSize,
				MaxPages:     defaultAlertsMaxPages,
			},
		},
		sink,
	)
	require.NoError(t, err)

	rcvr, ok := recv.(*combinedLogsReceiver)
	require.True(t, ok)
	rcvr.alerts.client = &mockClient

	err = recv.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return sink.LogRecordCount() > 0
	}, 5*time.Second, 10*time.Millisecond)

	err = recv.Shutdown(context.Background())
	require.NoError(t, err)

	logs := sink.AllLogs()[0]
	expectedLogs, err := golden.ReadLogs(filepath.Join("testdata", "alerts", "golden", "retrieved-logs.json"))
	require.NoError(t, err)
	require.NoError(t, plogtest.CompareLogs(expectedLogs, logs, plogtest.IgnoreObservedTimestamp()))
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
