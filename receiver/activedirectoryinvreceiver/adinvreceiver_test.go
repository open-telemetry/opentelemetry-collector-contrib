// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package activedirectoryinvreceiver

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.uber.org/zap"
)

type mockRuntime struct {
	mock.Mock
}

func (mr *mockRuntime) SupportedOS() bool {
	args := mr.Called()
	return args.Bool(0)
}

type mockClient struct {
	mock.Mock
}

func (mc *mockClient) Open(path string) (Container, error) {
	args := mc.Called(path)
	return args.Get(0).(Container), args.Error(1)
}

type mockContainer struct {
	mock.Mock
}

func (mc *mockContainer) ToObject() (Object, error) {
	args := mc.Called()
	return args.Get(0).(Object), args.Error(1)
}

func (mc *mockContainer) Close() {
	mc.Called()
}

func (mc *mockContainer) Children() (ObjectIter, error) {
	args := mc.Called()
	return args.Get(0).(ObjectIter), args.Error(1)
}

type mockObject struct {
	mock.Mock
}

func (mo *mockObject) Attrs(key string) ([]any, error) {
	args := mo.Called(key)
	return args.Get(0).([]any), args.Error(1)
}

func (mo *mockObject) ToContainer() (Container, error) {
	args := mo.Called()
	return args.Get(0).(Container), args.Error(1)
}

type mockObjectIter struct {
	mock.Mock
}

func (mo *mockObjectIter) Next() (Object, error) {
	args := mo.Called()
	return args.Get(0).(Object), args.Error(1)
}

func (mo *mockObjectIter) Close() {
	mo.Called()
}

func TestStart(t *testing.T) {
	cfg := createDefaultConfig().(*ADConfig)
	cfg.BaseDN = "CN=Guest,CN=Users,DC=exampledomain,DC=com"

	sink := &consumertest.LogsSink{}
	mc := defaultmockClient()
	mockRuntime := &mockRuntime{}
	mockRuntime.On("SupportedOS").Return(true)
	logsRcvr := newLogsReceiver(cfg, zap.NewNop(), mc, mockRuntime, sink)
	// Start the receiver
	err := logsRcvr.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	// Shutdown the receiver
	err = logsRcvr.Shutdown(t.Context())
	require.NoError(t, err)
}

func TestStartUnsupportedOS(t *testing.T) {
	cfg := createDefaultConfig().(*ADConfig)
	sink := &consumertest.LogsSink{}
	mockClient := &mockClient{}
	mockRuntime := &mockRuntime{}
	mockRuntime.On("SupportedOS").Return(false)
	logsRcvr := newLogsReceiver(cfg, zap.NewNop(), mockClient, mockRuntime, sink)
	// Start the receiver
	err := logsRcvr.Start(t.Context(), componenttest.NewNopHost())
	require.Error(t, err)
	require.Contains(t, err.Error(), "active_directory_inv is only supported on Windows")
}

func TestLogRecord(t *testing.T) {
	expectedBody := `{"name":"test","mail":"test","department":"test","manager":"test","memberOf":"test"}`
	var expectedResult, actualResult map[string]any
	cfg := createDefaultConfig().(*ADConfig)
	cfg.PollInterval = 1 * time.Second // Set poll interval to 1s to speed up test
	sink := &consumertest.LogsSink{}
	mockClient := defaultmockClient()
	mockRuntime := &mockRuntime{}
	mockRuntime.On("SupportedOS").Return(true)
	logsRcvr := newLogsReceiver(cfg, zap.NewNop(), mockClient, mockRuntime, sink)
	// Start the receiver
	err := logsRcvr.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		return sink.LogRecordCount() > 0
	}, 2*time.Second, 10*time.Millisecond)
	result := sink.AllLogs()[0].ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body().AsRaw()
	err = json.Unmarshal([]byte(expectedBody), &expectedResult)
	require.NoError(t, err)
	err = json.Unmarshal([]byte(result.(string)), &actualResult)
	require.NoError(t, err)
	// Shutdown the receiver
	err = logsRcvr.Shutdown(t.Context())
	require.NoError(t, err)
	assert.Equal(t, expectedResult, actualResult)
}

func defaultmockClient() Client {
	mockClient := &mockClient{}
	mockContainer := &mockContainer{}
	mockObject := &mockObject{}
	mockObjectIter := &mockObjectIter{}
	attrs := []any{"test"}
	mockContainer.On("ToObject").Return(mockObject, nil)
	mockContainer.On("Children").Return(mockObjectIter, errors.New("no children"))
	mockContainer.On("Close").Return(nil)
	mockObject.On("Attrs", mock.Anything).Return(attrs, nil)
	mockObject.On("ToContainer").Return(mockContainer, nil)
	mockClient.On("Open", mock.Anything).Return(mockContainer, nil)
	return mockClient
}
