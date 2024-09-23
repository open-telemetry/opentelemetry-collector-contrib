// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package githubreceiver

import (
	"context"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
)

type mockHTTPClient struct {
	mockDo func(req *http.Request) (*http.Response, error)
}

func (m *mockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	return m.mockDo(req)
}
func (m *mockHTTPClient) CloseIdleConnections() {}

func TestStartShutdown(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	recv, err := newGitHubLogsReceiver(cfg, zap.NewNop(), consumertest.NewNop())
	require.NoError(t, err)

	err = recv.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)

	err = recv.Shutdown(context.Background())
	require.NoError(t, err)
}

func TestShutdownNoServer(t *testing.T) {
	// test that shutdown without a start does not error or panic
	recv := newReceiver(t, createDefaultConfig().(*Config), consumertest.NewNop())
	require.NoError(t, recv.Shutdown(context.Background()))
}

/* Enterprise Log Tests */

func TestPollBasicEnterprise(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	mockAccessToken := configopaque.String("AccessToken")
	cfg.AccessToken = mockAccessToken
	cfg.LogType = "enterprise"
	cfg.Name = "justin-voss-observiq"
	sink := &consumertest.LogsSink{}
	recv, err := newGitHubLogsReceiver(cfg, zap.NewNop(), sink)
	require.NoError(t, err)
	recv.client = &mockHTTPClient{
		mockDo: func(req *http.Request) (*http.Response, error) {
			require.Contains(t, req.URL.String(), "per_page="+strconv.Itoa(gitHubMaxLimit))
			return mockAPIResponseOKBasicEnterprise(t), nil
		},
	}

	err = recv.poll(context.Background())
	require.NoError(t, err)
	logs := sink.AllLogs()
	log := logs[0]

	// golden.WriteLogs(t, "testdata/logsTestData/plog.yaml", log)
	expected, err := golden.ReadLogs("testdata/logsTestData/plog.yaml")
	require.NoError(t, err)
	require.NoError(t, plogtest.CompareLogs(expected, log, plogtest.IgnoreObservedTimestamp()))

	require.Equal(t, 20, log.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().Len())
	require.Equal(t, mockNextURLEnterprise, recv.nextURL)
	err = recv.Shutdown(context.Background())
	require.NoError(t, err)
}

func TestPollEmptyResponseEnterprise(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	mockAccessToken := configopaque.String("AccessToken")
	cfg.AccessToken = mockAccessToken
	cfg.Name = "justin-voss-observiq"
	sink := &consumertest.LogsSink{}
	recv, err := newGitHubLogsReceiver(cfg, zap.NewNop(), sink)
	require.NoError(t, err)
	recv.client = &mockHTTPClient{
		mockDo: func(req *http.Request) (*http.Response, error) {
			require.Contains(t, req.URL.String(), "per_page="+strconv.Itoa(gitHubMaxLimit))
			return mockAPIResponseEmpty(t), nil
		},
	}
	err = recv.poll(context.Background())
	require.NoError(t, err)
	logs := sink.AllLogs()
	require.Empty(t, logs)

	err = recv.Shutdown(context.Background())
	require.NoError(t, err)
}
func TestPollTooManyRequestsEnterprise(t *testing.T) {
	cfg := createDefaultConfig().(*Config)

	sink := &consumertest.LogsSink{}
	recv, err := newGitHubLogsReceiver(cfg, zap.NewNop(), sink)
	require.NoError(t, err)

	recv.client = &mockHTTPClient{
		mockDo: func(req *http.Request) (*http.Response, error) {

			require.Contains(t, req.URL.String(), "per_page="+strconv.Itoa(gitHubMaxLimit))
			if strings.Contains(req.URL.String(), "after=") {
				return mockAPIResponseTooManyRequests(), nil
			}
			return mockAPIResponseOK100LogsEnterprise(t), nil
		},
	}

	err = recv.poll(context.Background())
	require.NoError(t, err)

	logs := sink.AllLogs()

	require.Equal(t, 100, logs[0].ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().Len())
	require.Equal(t, mockNextURLEnterprise, recv.nextURL)

	err = recv.Shutdown(context.Background())
	require.NoError(t, err)
}
func TestPollOverflowEnterprise(t *testing.T) {
	cfg := createDefaultConfig().(*Config)

	sink := &consumertest.LogsSink{}
	recv, err := newGitHubLogsReceiver(cfg, zap.NewNop(), sink)
	require.NoError(t, err)

	recv.client = &mockHTTPClient{
		mockDo: func(req *http.Request) (*http.Response, error) {
			require.Contains(t, req.URL.String(), "per_page="+strconv.Itoa(gitHubMaxLimit))
			if strings.Contains(req.URL.String(), "after=") {
				return mockAPIResponseOKBasicEnterprise(t), nil
			}
			return mockAPIResponseOK100LogsEnterprise(t), nil
		},
	}

	err = recv.poll(context.Background())
	require.NoError(t, err)

	logs := sink.AllLogs()

	require.Equal(t, 120, logs[0].ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().Len())
	require.Equal(t, mockNextURLEnterprise, recv.nextURL)

	err = recv.Shutdown(context.Background())
	require.NoError(t, err)
}

func mockAPIResponseOKBasicEnterprise(t *testing.T) *http.Response {
	mockRes := &http.Response{}
	mockRes.StatusCode = http.StatusOK
	mockRes.Header = http.Header{}
	mockRes.Header.Add("Link", mockLinkHeaderNextEnterprise)
	mockRes.Header.Add("Link", mockLinkHeaderLastEnterprise)
	mockRes.Body = io.NopCloser(strings.NewReader(jsonFileAsString(t, "testdata/logsTestData/gitHubResponseBasicEnterprise.json")))
	return mockRes
}

func mockAPIResponseOK100LogsEnterprise(t *testing.T) *http.Response {
	mockRes := &http.Response{}
	mockRes.StatusCode = http.StatusOK
	mockRes.Header = http.Header{}
	mockRes.Header.Add("Link", mockLinkHeaderNextEnterprise)
	mockRes.Header.Add("Link", mockLinkHeaderLastEnterprise)
	mockRes.Body = io.NopCloser(strings.NewReader(jsonFileAsString(t, "testdata/logsTestData/gitHubResponse100LogsEnterprise.json")))
	return mockRes
}

/* Organization Log Tests */

func TestPollBasicOrganization(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	mockAccessToken := configopaque.String("AccessToken")
	cfg.AccessToken = mockAccessToken
	cfg.LogType = "organization"
	cfg.Name = "Justin-Organization-observIQ"
	sink := &consumertest.LogsSink{}
	recv, err := newGitHubLogsReceiver(cfg, zap.NewNop(), sink)
	require.NoError(t, err)
	recv.client = &mockHTTPClient{
		mockDo: func(req *http.Request) (*http.Response, error) {
			require.Contains(t, req.URL.String(), "per_page="+strconv.Itoa(gitHubMaxLimit))
			return mockAPIResponseOKBasicOrganization(t), nil
		},
	}

	err = recv.poll(context.Background())
	require.NoError(t, err)
	logs := sink.AllLogs()
	log := logs[0]

	// golden.WriteLogs(t, "testdata/logsTestData/plog_org.yaml", log)
	expected, err := golden.ReadLogs("testdata/logsTestData/plog_org.yaml")
	require.NoError(t, err)
	require.NoError(t, plogtest.CompareLogs(expected, log, plogtest.IgnoreObservedTimestamp()))

	require.Equal(t, 1, log.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().Len())
	require.Equal(t, mockNextURLOrganization, recv.nextURL)
	err = recv.Shutdown(context.Background())
	require.NoError(t, err)
}

func TestPollEmptyResponseOrganization(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	mockAccessToken := configopaque.String("AccessToken")
	cfg.AccessToken = mockAccessToken
	cfg.LogType = "organization"
	cfg.Name = "Justin-Organization-observIQ"
	sink := &consumertest.LogsSink{}
	recv, err := newGitHubLogsReceiver(cfg, zap.NewNop(), sink)
	require.NoError(t, err)
	recv.client = &mockHTTPClient{
		mockDo: func(req *http.Request) (*http.Response, error) {
			require.Contains(t, req.URL.String(), "per_page="+strconv.Itoa(gitHubMaxLimit))
			return mockAPIResponseEmpty(t), nil
		},
	}
	err = recv.poll(context.Background())
	require.NoError(t, err)
	logs := sink.AllLogs()
	require.Empty(t, logs)

	err = recv.Shutdown(context.Background())
	require.NoError(t, err)
}

func TestPollTooManyRequestsOrganization(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.LogType = "organization"
	cfg.Name = "Justin-Organization-observIQ"
	sink := &consumertest.LogsSink{}
	recv, err := newGitHubLogsReceiver(cfg, zap.NewNop(), sink)
	require.NoError(t, err)

	recv.client = &mockHTTPClient{
		mockDo: func(req *http.Request) (*http.Response, error) {

			require.Contains(t, req.URL.String(), "per_page="+strconv.Itoa(gitHubMaxLimit))
			if strings.Contains(req.URL.String(), "after=") {
				return mockAPIResponseTooManyRequests(), nil
			}
			return mockAPIResponseOKBasicOrganization(t), nil
		},
	}

	err = recv.poll(context.Background())
	require.NoError(t, err)

	logs := sink.AllLogs()

	require.Equal(t, 1, logs[0].ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().Len())
	require.Equal(t, mockNextURLOrganization, recv.nextURL)

	err = recv.Shutdown(context.Background())
	require.NoError(t, err)
}

func TestPollOverflowOrganization(t *testing.T) {
	cfg := createDefaultConfig().(*Config)

	sink := &consumertest.LogsSink{}
	recv, err := newGitHubLogsReceiver(cfg, zap.NewNop(), sink)
	require.NoError(t, err)

	recv.client = &mockHTTPClient{
		mockDo: func(req *http.Request) (*http.Response, error) {
			require.Contains(t, req.URL.String(), "per_page="+strconv.Itoa(gitHubMaxLimit))
			if strings.Contains(req.URL.String(), "after=") {
				return mockAPIResponseOKBasicOrganization(t), nil
			}
			return mockAPIResponseOK24LogsOrganization(t), nil
		},
	}

	err = recv.poll(context.Background())
	require.NoError(t, err)

	logs := sink.AllLogs()

	require.Equal(t, 24, logs[0].ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().Len())
	require.Equal(t, mockNextURLOrganization, recv.nextURL)

	err = recv.Shutdown(context.Background())
	require.NoError(t, err)
}

func mockAPIResponseOKBasicOrganization(t *testing.T) *http.Response {
	mockRes := &http.Response{}
	mockRes.StatusCode = http.StatusOK
	mockRes.Header = http.Header{}
	mockRes.Header.Add("Link", mockLinkHeaderNextOrganization)
	mockRes.Header.Add("Link", mockLinkHeaderLastOrganization)
	mockRes.Body = io.NopCloser(strings.NewReader(jsonFileAsString(t, "testdata/logsTestData/gitHubResponseBasicOrganization.json")))
	return mockRes
}

func mockAPIResponseOK24LogsOrganization(t *testing.T) *http.Response {
	mockRes := &http.Response{}
	mockRes.StatusCode = http.StatusOK
	mockRes.Header = http.Header{}
	mockRes.Header.Add("Link", mockLinkHeaderNextOrganization)
	mockRes.Header.Add("Link", mockLinkHeaderLastOrganization)
	mockRes.Body = io.NopCloser(strings.NewReader(jsonFileAsString(t, "testdata/logsTestData/gitHubResponse24LogsOrganization.json")))
	return mockRes
}

/* User Log Tests */

func TestPollBasicUser(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	mockAccessToken := configopaque.String("AccessToken")
	cfg.AccessToken = mockAccessToken
	cfg.LogType = "user"
	cfg.Name = "justinianvoss22"
	sink := &consumertest.LogsSink{}
	recv, err := newGitHubLogsReceiver(cfg, zap.NewNop(), sink)
	require.NoError(t, err)
	recv.client = &mockHTTPClient{
		mockDo: func(req *http.Request) (*http.Response, error) {
			require.Contains(t, req.URL.String(), "per_page="+strconv.Itoa(gitHubMaxLimit))
			return mockAPIResponseOKBasicUser(t), nil
		},
	}

	err = recv.poll(context.Background())
	require.NoError(t, err)
	logs := sink.AllLogs()
	log := logs[0]

	// golden.WriteLogs(t, "testdata/logsTestData/plog_user.yaml", log)
	expected, err := golden.ReadLogs("testdata/logsTestData/plog_user.yaml")
	require.NoError(t, err)
	require.NoError(t, plogtest.CompareLogs(expected, log, plogtest.IgnoreObservedTimestamp()))

	require.Equal(t, 1, log.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().Len())
	require.Equal(t, mockNextURLUser, recv.nextURL)
	err = recv.Shutdown(context.Background())
	require.NoError(t, err)
}

func TestPollEmptyResponseUser(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	mockAccessToken := configopaque.String("AccessToken")
	cfg.AccessToken = mockAccessToken
	cfg.LogType = "user"
	cfg.Name = "justinianvoss22"
	sink := &consumertest.LogsSink{}
	recv, err := newGitHubLogsReceiver(cfg, zap.NewNop(), sink)
	require.NoError(t, err)
	recv.client = &mockHTTPClient{
		mockDo: func(req *http.Request) (*http.Response, error) {
			require.Contains(t, req.URL.String(), "per_page="+strconv.Itoa(gitHubMaxLimit))
			return mockAPIResponseEmpty(t), nil
		},
	}
	err = recv.poll(context.Background())
	require.NoError(t, err)
	logs := sink.AllLogs()
	require.Empty(t, logs)

	err = recv.Shutdown(context.Background())
	require.NoError(t, err)
}

func TestPollTooManyRequestsUser(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.LogType = "user"
	cfg.Name = "justinianvoss22"
	sink := &consumertest.LogsSink{}
	recv, err := newGitHubLogsReceiver(cfg, zap.NewNop(), sink)
	require.NoError(t, err)

	recv.client = &mockHTTPClient{
		mockDo: func(req *http.Request) (*http.Response, error) {

			require.Contains(t, req.URL.String(), "per_page="+strconv.Itoa(gitHubMaxLimit))
			if strings.Contains(req.URL.String(), "after=") {
				return mockAPIResponseTooManyRequests(), nil
			}
			return mockAPIResponseOK30LogsUser(t), nil
		},
	}

	err = recv.poll(context.Background())
	require.NoError(t, err)

	logs := sink.AllLogs()

	require.Equal(t, 30, logs[0].ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().Len())
	require.Equal(t, mockNextURLUser, recv.nextURL)

	err = recv.Shutdown(context.Background())
	require.NoError(t, err)
}

func TestPollOverflowUser(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.LogType = "user"
	cfg.Name = "justinianvoss22"
	sink := &consumertest.LogsSink{}
	recv, err := newGitHubLogsReceiver(cfg, zap.NewNop(), sink)
	require.NoError(t, err)

	recv.client = &mockHTTPClient{
		mockDo: func(req *http.Request) (*http.Response, error) {
			require.Contains(t, req.URL.String(), "per_page="+strconv.Itoa(gitHubMaxLimit))
			if strings.Contains(req.URL.String(), "after=") {
				return mockAPIResponseOKBasicUser(t), nil
			}
			return mockAPIResponseOK30LogsUser(t), nil
		},
	}

	err = recv.poll(context.Background())
	require.NoError(t, err)

	logs := sink.AllLogs()

	require.Equal(t, 30, logs[0].ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().Len())
	require.Equal(t, mockNextURLUser, recv.nextURL)

	err = recv.Shutdown(context.Background())
	require.NoError(t, err)
}

func mockAPIResponseOKBasicUser(t *testing.T) *http.Response {
	mockRes := &http.Response{}
	mockRes.StatusCode = http.StatusOK
	mockRes.Header = http.Header{}
	mockRes.Header.Add("Link", mockLinkHeaderNextUser)
	mockRes.Header.Add("Link", mockLinkHeaderLastUser)
	mockRes.Body = io.NopCloser(strings.NewReader(jsonFileAsString(t, "testdata/logsTestData/gitHubResponseBasicUser.json")))
	return mockRes
}

func mockAPIResponseOK30LogsUser(t *testing.T) *http.Response {
	mockRes := &http.Response{}
	mockRes.StatusCode = http.StatusOK
	mockRes.Header = http.Header{}
	mockRes.Header.Add("Link", mockLinkHeaderNextUser)
	mockRes.Header.Add("Link", mockLinkHeaderLastUser)
	mockRes.Body = io.NopCloser(strings.NewReader(jsonFileAsString(t, "testdata/logsTestData/gitHubResponse30LogsUser.json")))
	return mockRes
}

func newReceiver(t *testing.T, cfg *Config, c consumer.Logs) *gitHubLogsReceiver {
	r, err := newGitHubLogsReceiver(cfg, zap.NewNop(), c)
	require.NoError(t, err)
	return r
}

func jsonFileAsString(t *testing.T, filePath string) string {
	jsonBytes, err := os.ReadFile(filePath)
	require.NoError(t, err)
	return string(jsonBytes)
}

func mockAPIResponseEmpty(t *testing.T) *http.Response {
	mockRes := &http.Response{}
	mockRes.StatusCode = http.StatusOK
	mockRes.Body = io.NopCloser(strings.NewReader(jsonFileAsString(t, "testdata/logsTestData/gitHubResponseEmpty.json")))
	return mockRes
}

func mockAPIResponseTooManyRequests() *http.Response {
	return &http.Response{
		StatusCode: http.StatusTooManyRequests,
		Body:       io.NopCloser(strings.NewReader("{}")),
	}
}

var (
	mockNextURLEnterprise        = `https://api.github.com/enterprises/justin-voss-observiq/audit-log?per_page=100&after=MTcyNTQ4MTg2NDI2MXxjZmxuckFiZ1lUT0lUdFdoUi1GUVl3&before=`
	mockLinkHeaderNextEnterprise = `<https://api.github.com/enterprises/justin-voss-observiq/audit-log?per_page=100&after=MTcyNTQ4MTg2NDI2MXxjZmxuckFiZ1lUT0lUdFdoUi1GUVl3&before=>; rel="next"`
	mockLinkHeaderLastEnterprise = `<https://api.github.com/enterprises/justin-voss-observiq/audit-log?per_page=100&after=MTcyNTQ4MTg2NDI2MXxjZmxuckFiZ1lUT0lUdFdoUi1GUVl3&before=>; rel="last"`

	mockNextURLOrganization        = `https://api.github.com/orgs/Justin-Organization-observIQ/audit-log?per_page=100&after=MTcyNTQ4MTg2NDI2MXxjZmxuckFiZ1lUT0lUdFdoUi1GUVl3&before=`
	mockLinkHeaderNextOrganization = `<https://api.github.com/orgs/Justin-Organization-observIQ/audit-log?per_page=100&after=MTcyNTQ4MTg2NDI2MXxjZmxuckFiZ1lUT0lUdFdoUi1GUVl3&before=>; rel="next"`
	mockLinkHeaderLastOrganization = `<https://api.github.com/orgs/Justin-Organization-observIQ/audit-log?per_page=100&after=MTcyNTQ4MTg2NDI2MXxjZmxuckFiZ1lUT0lUdFdoUi1GUVl3&before=>; rel="last"`

	mockNextURLUser        = `https://api.github.com/users/justinianvoss22/events/public?per_page=100&after=MTcyNTQ4MTg2NDI2MXxjZmxuckFiZ1lUT0lUdFdoUi1GUVl3&before=`
	mockLinkHeaderNextUser = `<https://api.github.com/users/justinianvoss22/events/public?per_page=100&after=MTcyNTQ4MTg2NDI2MXxjZmxuckFiZ1lUT0lUdFdoUi1GUVl3&before=>; rel="next"`
	mockLinkHeaderLastUser = `<https://api.github.com/users/justinianvoss22/events/public?per_page=100&after=MTcyNTQ4MTg2NDI2MXxjZmxuckFiZ1lUT0lUdFdoUi1GUVl3&before=>; rel="last"`
)
