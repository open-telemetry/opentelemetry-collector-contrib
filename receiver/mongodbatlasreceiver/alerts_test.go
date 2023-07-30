// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mongodbatlasreceiver

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/atlas/mongodbatlas"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/extension/experimental/storage"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
	"go.uber.org/zap/zaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbatlasreceiver/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbatlasreceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbatlasreceiver/internal/model"
)

func TestPayloadToLogRecord(t *testing.T) {
	now := time.Time{}

	testCases := []struct {
		name         string
		payload      string
		expectedLogs func(string) plog.Logs
		expectedErr  string
	}{
		{
			name: "minimal record",
			payload: `{
				"created": "2022-06-03T22:30:31Z",
				"groupId": "some-group-id",
				"humanReadable": "Some event happened",
				"alertConfigId": "123",
				"eventTypeName": "EVENT",
				"id": "some-id",
				"updated": "2022-06-03T22:30:31Z",
				"status": "STATUS"
			}`,
			expectedLogs: func(payload string) plog.Logs {
				logs := plog.NewLogs()
				rl := logs.ResourceLogs().AppendEmpty()
				lr := rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()

				assert.NoError(t, rl.Resource().Attributes().FromRaw(map[string]interface{}{
					"mongodbatlas.group.id":        "some-group-id",
					"mongodbatlas.alert.config.id": "123",
				}))

				assert.NoError(t, lr.Attributes().FromRaw(map[string]interface{}{
					"created":      "2022-06-03T22:30:31Z",
					"message":      "Some event happened",
					"event.domain": "mongodbatlas",
					"event.name":   "EVENT",
					"updated":      "2022-06-03T22:30:31Z",
					"status":       "STATUS",
					"id":           "some-id",
				}))

				lr.SetObservedTimestamp(pcommon.NewTimestampFromTime(now))
				lr.SetTimestamp(pcommon.NewTimestampFromTime(time.Date(2022, time.June, 3, 22, 30, 31, 0, time.UTC)))
				lr.SetSeverityNumber(plog.SeverityNumberInfo)

				lr.Body().SetStr(payload)

				return logs
			},
		},

		{
			name: "optional fields",
			payload: `{
				"created": "2022-06-03T22:30:31Z",
				"groupId": "some-group-id",
				"humanReadable": "Some event happened",
				"alertConfigId": "123",
				"eventTypeName": "EVENT",
				"id": "some-id",
				"updated": "2022-06-03T22:30:35Z",
				"status": "STATUS",
				"replicaSetName": "replica-set",
				"metricName": "my-metric",
				"typeName": "type-name",
				"clusterName": "cluster-name",
				"userAlias": "user-alias",
				"lastNotified": "2022-06-03T22:30:33Z",
				"resolved": "2022-06-03T22:30:34Z",
				"acknowledgementComment": "Scheduled maintenance",
				"acknowledgingUsername": "devops",
				"acknowledgedUntil": "2022-06-03T22:32:34Z",
				"hostnameAndPort": "my-host.mongodb.com:4923",
				"currentValue": {
					"number": 14,
					"units": "RAW"
				}
			}`,
			expectedLogs: func(payload string) plog.Logs {
				logs := plog.NewLogs()
				rl := logs.ResourceLogs().AppendEmpty()
				lr := rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()

				assert.NoError(t, rl.Resource().Attributes().FromRaw(map[string]interface{}{
					"mongodbatlas.group.id":         "some-group-id",
					"mongodbatlas.alert.config.id":  "123",
					"mongodbatlas.cluster.name":     "cluster-name",
					"mongodbatlas.replica_set.name": "replica-set",
				}))

				assert.NoError(t, lr.Attributes().FromRaw(map[string]interface{}{
					"acknowledgement.comment":  "Scheduled maintenance",
					"acknowledgement.until":    "2022-06-03T22:32:34Z",
					"acknowledgement.username": "devops",
					"created":                  "2022-06-03T22:30:31Z",
					"event.name":               "EVENT",
					"event.domain":             "mongodbatlas",
					"id":                       "some-id",
					"last_notified":            "2022-06-03T22:30:33Z",
					"message":                  "Some event happened",
					"metric.name":              "my-metric",
					"metric.units":             "RAW",
					"metric.value":             float64(14),
					"net.peer.name":            "my-host.mongodb.com",
					"net.peer.port":            4923,
					"resolved":                 "2022-06-03T22:30:34Z",
					"status":                   "STATUS",
					"type_name":                "type-name",
					"updated":                  "2022-06-03T22:30:35Z",
					"user_alias":               "user-alias",
				}))

				lr.SetObservedTimestamp(pcommon.NewTimestampFromTime(now))
				lr.SetTimestamp(pcommon.NewTimestampFromTime(time.Date(2022, time.June, 3, 22, 30, 35, 0, time.UTC)))
				lr.SetSeverityNumber(plog.SeverityNumberInfo)

				lr.Body().SetStr(payload)

				return logs
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			logs, err := payloadToLogs(now, []byte(tc.payload))
			if tc.expectedErr != "" {
				require.Error(t, err)
				require.Nil(t, logs)
				require.Contains(t, err.Error(), tc.expectedErr)
			} else {
				require.NoError(t, err)
				require.NotNil(t, logs)
				require.NoError(t, plogtest.CompareLogs(tc.expectedLogs(tc.payload), logs))
			}
		})
	}
}

func TestSeverityFromAlert(t *testing.T) {
	testCases := []struct {
		alert model.Alert
		s     plog.SeverityNumber
	}{
		{
			alert: model.Alert{Status: "OPEN"},
			s:     plog.SeverityNumberWarn,
		},
		{
			alert: model.Alert{Status: "TRACKING"},
			s:     plog.SeverityNumberInfo,
		},
		{
			alert: model.Alert{Status: "CLOSED"},
			s:     plog.SeverityNumberInfo,
		},
		{
			alert: model.Alert{Status: "INFORMATIONAL"},
			s:     plog.SeverityNumberInfo,
		},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%s -> %s", tc.alert.Status, tc.s.String()), func(t *testing.T) {
			s := severityFromAlert(tc.alert)
			require.Equal(t, tc.s, s)
		})
	}
}

func TestVerifyHMACSignature(t *testing.T) {
	testCases := []struct {
		name            string
		secret          string
		payload         []byte
		signatureHeader string
		expectedErr     string
	}{
		{
			name:            "Signature matches",
			secret:          "some_secret",
			payload:         []byte(`{"some_key": "some_value"}`),
			signatureHeader: "n5O6j3X1o1iYgFLcbjjMHBIYkFU=",
		},
		{
			name:            "Signature does not match",
			secret:          "a_different_secret",
			payload:         []byte(`{"some_key": "some_value"}`),
			signatureHeader: "n5O6j3X1o1iYgFLcbjjMHBIYkFU=",
			expectedErr:     "calculated signature does not equal header signature",
		},
		{
			name:            "Signature is not valid base64",
			secret:          "a_different_secret",
			payload:         []byte(`{"some_key": "some_value"}`),
			signatureHeader: "%asd82kmnc~`",
			expectedErr:     "illegal base64 data",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := verifyHMACSignature(tc.secret, tc.payload, tc.signatureHeader)
			if tc.expectedErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.expectedErr)
			} else {
				require.NoError(t, err)
			}

		})
	}
}

func TestHandleRequest(t *testing.T) {
	testCases := []struct {
		name               string
		request            *http.Request
		expectedStatusCode int
		logExpected        bool
		consumerFailure    bool
	}{
		{
			name: "No ContentLength set",
			request: &http.Request{
				ContentLength: -1,
				Method:        "POST",
				URL:           &url.URL{},
				Body:          io.NopCloser(bytes.NewBufferString(`{"id": "an-id"}`)),
				Header: map[string][]string{
					textproto.CanonicalMIMEHeaderKey(signatureHeaderName): {"BGKR1L/1GfvINp8+rr9ISwm7VhQ="},
				},
			},
			logExpected:        false,
			consumerFailure:    false,
			expectedStatusCode: http.StatusLengthRequired,
		},
		{
			name: "ContentLength too large",
			request: &http.Request{
				ContentLength: maxContentLength + 1,
				Method:        "POST",
				URL:           &url.URL{},
				Body:          io.NopCloser(bytes.NewBufferString(`{"id": "an-id"}`)),
				Header: map[string][]string{
					textproto.CanonicalMIMEHeaderKey(signatureHeaderName): {"BGKR1L/1GfvINp8+rr9ISwm7VhQ="},
				},
			},
			logExpected:        false,
			consumerFailure:    false,
			expectedStatusCode: http.StatusRequestEntityTooLarge,
		},
		{
			name: "ContentLength is incorrect for payload",
			request: &http.Request{
				ContentLength: 1,
				Method:        "POST",
				URL:           &url.URL{},
				Body:          io.NopCloser(bytes.NewBufferString(`{"id": "an-id"}`)),
				Header: map[string][]string{
					textproto.CanonicalMIMEHeaderKey(signatureHeaderName): {"BGKR1L/1GfvINp8+rr9ISwm7VhQ="},
				},
			},
			logExpected:        false,
			consumerFailure:    false,
			expectedStatusCode: http.StatusBadRequest,
		},
		{
			name: "ContentLength is larger than actual payload",
			request: &http.Request{
				ContentLength: 32,
				Method:        "POST",
				URL:           &url.URL{},
				Body:          io.NopCloser(bytes.NewBufferString(`{"id": "an-id"}`)),
				Header: map[string][]string{
					textproto.CanonicalMIMEHeaderKey(signatureHeaderName): {"BGKR1L/1GfvINp8+rr9ISwm7VhQ="},
				},
			},
			logExpected:        false,
			consumerFailure:    false,
			expectedStatusCode: http.StatusBadRequest,
		},
		{
			name: "No HMAC signature",
			request: &http.Request{
				ContentLength: 15,
				Method:        "POST",
				URL:           &url.URL{},
				Body:          io.NopCloser(bytes.NewBufferString(`{"id": "an-id"}`)),
			},
			logExpected:        false,
			consumerFailure:    false,
			expectedStatusCode: http.StatusBadRequest,
		},
		{
			name: "Incorrect HMAC signature",
			request: &http.Request{
				ContentLength: 15,
				Method:        "POST",
				URL:           &url.URL{},
				Body:          io.NopCloser(bytes.NewBufferString(`{"id": "an-id"}`)),
				Header: map[string][]string{
					textproto.CanonicalMIMEHeaderKey(signatureHeaderName): {"n5O6j3X1o1iYgFLcbjjMHBIYkFU="},
				},
			},
			logExpected:        false,
			consumerFailure:    false,
			expectedStatusCode: http.StatusBadRequest,
		},
		{
			name: "Invalid payload",
			request: &http.Request{
				ContentLength: 14,
				Method:        "POST",
				URL:           &url.URL{},
				Body:          io.NopCloser(bytes.NewBufferString(`{"id": "an-id"`)),
				Header: map[string][]string{
					textproto.CanonicalMIMEHeaderKey(signatureHeaderName): {"P8OMdoVqNNE9iTFccTuwTY/J7vQ="},
				},
			},
			logExpected:        false,
			consumerFailure:    false,
			expectedStatusCode: http.StatusBadRequest,
		},
		{
			name: "Consumer fails",
			request: &http.Request{
				ContentLength: 15,
				Method:        "POST",
				URL:           &url.URL{},
				Body:          io.NopCloser(bytes.NewBufferString(`{"id": "an-id"}`)),
				Header: map[string][]string{
					textproto.CanonicalMIMEHeaderKey(signatureHeaderName): {"BGKR1L/1GfvINp8+rr9ISwm7VhQ="},
				},
			},
			logExpected:        false,
			consumerFailure:    true,
			expectedStatusCode: http.StatusInternalServerError,
		},
		{
			name: "Request succeeds",
			request: &http.Request{
				ContentLength: 15,
				Method:        "POST",
				URL:           &url.URL{},
				Body:          io.NopCloser(bytes.NewBufferString(`{"id": "an-id"}`)),
				Header: map[string][]string{
					textproto.CanonicalMIMEHeaderKey(signatureHeaderName): {"BGKR1L/1GfvINp8+rr9ISwm7VhQ="},
				},
			},
			logExpected:        true,
			consumerFailure:    false,
			expectedStatusCode: http.StatusOK,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var consumer consumer.Logs
			if tc.consumerFailure {
				consumer = consumertest.NewErr(errors.New("consumer failed"))
			} else {
				consumer = &consumertest.LogsSink{}
			}

			set := receivertest.NewNopCreateSettings()
			set.Logger = zaptest.NewLogger(t)
			ar, err := newAlertsReceiver(set, &Config{Alerts: AlertConfig{Secret: "some_secret"}}, consumer)
			require.NoError(t, err, "Failed to create alerts receiver")

			rec := httptest.NewRecorder()
			ar.handleRequest(rec, tc.request)

			assert.Equal(t, tc.expectedStatusCode, rec.Code, "Status codes are not equal")

			if !tc.consumerFailure {
				if tc.logExpected {
					assert.Equal(t, 1, consumer.(*consumertest.LogsSink).LogRecordCount(), "Did not receive log record")
				} else {
					assert.Equal(t, 0, consumer.(*consumertest.LogsSink).LogRecordCount(), "Received log record when it should have been dropped")
				}
			}
		})
	}
}

const (
	testAlertID         = "633335c99998645b1803c60b"
	testGroupID         = "5bc762b579358e3332046e6a"
	testAlertConfigID   = "REDACTED-alert"
	testOrgID           = "test-org-id"
	testProjectID       = "test-project-id"
	testProjectName     = "test-project"
	testMetricName      = "metric-name"
	testTypeName        = "OUTSIDE_METRIC_THRESHOLD"
	testHostNameAndPort = "127.0.0.1:27017"
	testClusterName     = "Cluster1"
)

func TestAlertsRetrieval(t *testing.T) {
	cases := []struct {
		name            string
		config          func() *Config
		client          func() alertsClient
		validateEntries func(*testing.T, plog.Logs)
	}{
		{
			name: "default",
			config: func() *Config {
				return &Config{
					ScraperControllerSettings: scraperhelper.NewDefaultScraperControllerSettings(metadata.Type),
					Granularity:               defaultGranularity,
					RetrySettings:             exporterhelper.NewDefaultRetrySettings(),
					Alerts: AlertConfig{
						Mode: alertModePoll,
						Projects: []*ProjectConfig{
							{
								Name: testProjectName,
							},
						},
						PageSize:     defaultAlertsPageSize,
						MaxPages:     defaultAlertsMaxPages,
						PollInterval: 1 * time.Second,
					},
				}
			},
			client: func() alertsClient {
				return testClient()
			},
			validateEntries: func(t *testing.T, logs plog.Logs) {
				expectedStringAttributes := map[string]string{
					"id":           testAlertID,
					"status":       "TRACKING",
					"event.domain": "mongodbatlas",
					"metric.name":  testMetricName,
					"type_name":    testTypeName,
				}
				validateAttributes(t, expectedStringAttributes, logs)
				expectedResourceAttributes := map[string]string{
					"mongodbatlas.group.id":        testGroupID,
					"mongodbatlas.alert.config.id": testAlertConfigID,
					"mongodbatlas.project.name":    testProjectName,
					"mongodbatlas.org.id":          testOrgID,
				}
				ra := logs.ResourceLogs().At(0).Resource().Attributes()
				for k, v := range expectedResourceAttributes {
					value, ok := ra.Get(k)
					require.True(t, ok)
					require.Equal(t, v, value.AsString())
				}
			},
		},
		{
			name: "project cluster inclusions",
			config: func() *Config {
				return &Config{
					ScraperControllerSettings: scraperhelper.NewDefaultScraperControllerSettings(metadata.Type),
					Granularity:               defaultGranularity,
					RetrySettings:             exporterhelper.NewDefaultRetrySettings(),
					Alerts: AlertConfig{
						Mode: alertModePoll,
						Projects: []*ProjectConfig{
							{
								Name:            testProjectName,
								IncludeClusters: []string{testClusterName},
							},
						},
						PageSize:     defaultAlertsPageSize,
						MaxPages:     defaultAlertsMaxPages,
						PollInterval: 1 * time.Second,
					},
				}
			},
			client: func() alertsClient {
				return testClient()
			},
			validateEntries: func(t *testing.T, logs plog.Logs) {
				require.Equal(t, logs.LogRecordCount(), 1)
			},
		},
		{
			name: "hostname and port missing",
			config: func() *Config {
				return &Config{
					ScraperControllerSettings: scraperhelper.NewDefaultScraperControllerSettings(metadata.Type),
					Granularity:               defaultGranularity,
					RetrySettings:             exporterhelper.NewDefaultRetrySettings(),
					Alerts: AlertConfig{
						Mode: alertModePoll,
						Projects: []*ProjectConfig{
							{
								Name: testProjectName,
							},
						},
						PageSize:     defaultAlertsPageSize,
						MaxPages:     defaultAlertsMaxPages,
						PollInterval: 1 * time.Second,
					},
				}
			},
			client: func() alertsClient {
				tc := &mockAlertsClient{}
				tc.On("GetProject", mock.Anything, mock.Anything).Return(&mongodbatlas.Project{
					ID:    testProjectID,
					OrgID: testOrgID,
					Name:  testProjectName,
					Links: []*mongodbatlas.Link{},
				}, nil)
				tc.On("GetAlerts", mock.Anything, testProjectID, mock.Anything).Return(
					[]mongodbatlas.Alert{
						{
							ID:            testAlertID,
							GroupID:       testGroupID,
							AlertConfigID: testAlertConfigID,
							EventTypeName: testTypeName,
							Created:       time.Now().Format(time.RFC3339),
							Updated:       time.Now().Format(time.RFC3339),
							Enabled:       new(bool),
							Status:        "TRACKING",
							MetricName:    testMetricName,
							CurrentValue: &mongodbatlas.CurrentValue{
								Number: new(float64),
								Units:  "By",
							},
							ClusterName:     testClusterName,
							HostnameAndPort: "",
							Matchers:        []mongodbatlas.Matcher{},
							MetricThreshold: &mongodbatlas.MetricThreshold{},
							Notifications:   []mongodbatlas.Notification{},
						},
					}, false, nil)
				return tc
			},
			validateEntries: func(t *testing.T, l plog.Logs) {
				require.Equal(t, l.LogRecordCount(), 1)
				rl := l.ResourceLogs().At(0)
				sl := rl.ScopeLogs().At(0)
				lr := sl.LogRecords().At(0)
				_, hasHostname := lr.Attributes().Get("net.peer.name")
				require.False(t, hasHostname)
				_, hasPort := lr.Attributes().Get("net.peer.port")
				require.False(t, hasPort)
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			logSink := &consumertest.LogsSink{}
			alertsRcvr, err := newAlertsReceiver(receivertest.NewNopCreateSettings(), tc.config(), logSink)
			require.NoError(t, err)
			alertsRcvr.client = tc.client()

			err = alertsRcvr.Start(context.Background(), componenttest.NewNopHost(), storage.NewNopClient())
			require.NoError(t, err)

			require.Eventually(t, func() bool {
				return logSink.LogRecordCount() > 0
			}, 10*time.Second, 10*time.Millisecond)

			require.NoError(t, alertsRcvr.Shutdown(context.Background()))
			logs := logSink.AllLogs()[0]

			tc.validateEntries(t, logs)
		})
	}
}

func TestAlertPollingExclusions(t *testing.T) {
	logSink := &consumertest.LogsSink{}
	alertsRcvr, err := newAlertsReceiver(receivertest.NewNopCreateSettings(), &Config{
		Alerts: AlertConfig{
			Enabled: true,
			Mode:    alertModePoll,
			Projects: []*ProjectConfig{
				{
					Name:            testProjectName,
					ExcludeClusters: []string{testClusterName},
				},
			},
			PageSize:     defaultAlertsPageSize,
			MaxPages:     defaultAlertsMaxPages,
			PollInterval: 1 * time.Second,
		},
	}, logSink)
	require.NoError(t, err)
	alertsRcvr.client = testClient()

	err = alertsRcvr.Start(context.Background(), componenttest.NewNopHost(), storage.NewNopClient())
	require.NoError(t, err)

	require.Never(t, func() bool {
		return logSink.LogRecordCount() > 0
	}, 3*time.Second, 10*time.Millisecond)

	require.NoError(t, alertsRcvr.Shutdown(context.Background()))
}

func testClient() *mockAlertsClient {
	ac := &mockAlertsClient{}
	ac.On("GetProject", mock.Anything, mock.Anything).Return(&mongodbatlas.Project{
		ID:    testProjectID,
		OrgID: testOrgID,
		Name:  testProjectName,
		Links: []*mongodbatlas.Link{},
	}, nil)
	ac.On("GetAlerts", mock.Anything, testProjectID, mock.Anything).Return(
		[]mongodbatlas.Alert{
			testAlert(),
		},
		false, nil)
	return ac
}

func testAlert() mongodbatlas.Alert {
	return mongodbatlas.Alert{
		ID:            testAlertID,
		GroupID:       testGroupID,
		AlertConfigID: testAlertConfigID,
		EventTypeName: testTypeName,
		Created:       time.Now().Format(time.RFC3339),
		Updated:       time.Now().Format(time.RFC3339),
		Enabled:       new(bool),
		Status:        "TRACKING",
		MetricName:    testMetricName,
		CurrentValue: &mongodbatlas.CurrentValue{
			Number: new(float64),
			Units:  "By",
		},
		ReplicaSetName:  "",
		ClusterName:     testClusterName,
		HostnameAndPort: testHostNameAndPort,
		Matchers:        []mongodbatlas.Matcher{},
		MetricThreshold: &mongodbatlas.MetricThreshold{},
		Notifications:   []mongodbatlas.Notification{},
	}
}

func validateAttributes(t *testing.T, expectedStringAttributes map[string]string, logs plog.Logs) {
	for i := 0; i < logs.ResourceLogs().Len(); i++ {
		rl := logs.ResourceLogs().At(0)
		for j := 0; j < rl.ScopeLogs().Len(); j++ {
			sl := rl.ScopeLogs().At(j)
			for k := 0; k < sl.LogRecords().Len(); k++ {
				lr := sl.LogRecords().At(k)
				for k, v := range expectedStringAttributes {
					val, ok := lr.Attributes().Get(k)
					require.True(t, ok)
					require.Equal(t, v, val.AsString())
				}
			}
		}
	}
}

type mockAlertsClient struct {
	mock.Mock
}

func (mac *mockAlertsClient) GetProject(ctx context.Context, pID string) (*mongodbatlas.Project, error) {
	args := mac.Called(ctx, pID)
	return args.Get(0).(*mongodbatlas.Project), args.Error(1)
}

func (mac *mockAlertsClient) GetAlerts(ctx context.Context, pID string, opts *internal.AlertPollOptions) ([]mongodbatlas.Alert, bool, error) {
	args := mac.Called(ctx, pID, opts)
	return args.Get(0).([]mongodbatlas.Alert), args.Bool(1), args.Error(2)
}
