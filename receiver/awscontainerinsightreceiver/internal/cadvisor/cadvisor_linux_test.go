// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package cadvisor

import (
	"errors"
	"net/http"
	"testing"

	"github.com/google/cadvisor/cache/memory"
	"github.com/google/cadvisor/container"
	info "github.com/google/cadvisor/info/v1"
	"github.com/google/cadvisor/manager"
	"github.com/google/cadvisor/utils/sysfs"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/cadvisor/testutils"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/stores"
)

type mockCadvisorManager struct {
	t *testing.T
}

// Start the manager. Calling other manager methods before this returns
// may produce undefined behavior.
func (*mockCadvisorManager) Start() error {
	return nil
}

// Get information about all subcontainers of the specified container (includes self).
func (m *mockCadvisorManager) SubcontainersInfo(_ string, _ *info.ContainerInfoRequest) ([]*info.ContainerInfo, error) {
	containerInfos := testutils.LoadContainerInfo(m.t, "./extractors/testdata/CurInfoContainer.json")
	return containerInfos, nil
}

type mockCadvisorManager2 struct{}

func (*mockCadvisorManager2) Start() error {
	return errors.New("new error")
}

func (*mockCadvisorManager2) SubcontainersInfo(string, *info.ContainerInfoRequest) ([]*info.ContainerInfo, error) {
	return nil, nil
}

func newMockCreateManager(t *testing.T) createCadvisorManager {
	return func(*memory.InMemoryCache, sysfs.SysFs, manager.HousekeepingConfig,
		container.MetricSet, *http.Client, []string,
		string,
	) (cadvisorManager, error) {
		return &mockCadvisorManager{t: t}, nil
	}
}

var mockCreateManager2 = func(*memory.InMemoryCache, sysfs.SysFs, manager.HousekeepingConfig,
	container.MetricSet, *http.Client, []string,
	string,
) (cadvisorManager, error) {
	return &mockCadvisorManager2{}, nil
}

var mockCreateManagerWithError = func(*memory.InMemoryCache, sysfs.SysFs, manager.HousekeepingConfig,
	container.MetricSet, *http.Client, []string,
	string,
) (cadvisorManager, error) {
	return nil, errors.New("error")
}

type MockDecorator struct{}

func (*MockDecorator) Decorate(metric stores.CIMetric) stores.CIMetric {
	return metric
}

func (*MockDecorator) Shutdown() error {
	return nil
}

func TestGetMetrics(t *testing.T) {
	hostInfo := testutils.MockHostInfo{ClusterName: "cluster"}
	decoratorOption := WithDecorator(&MockDecorator{})

	c, err := New("eks", hostInfo, zap.NewNop(), cadvisorManagerCreator(newMockCreateManager(t)), decoratorOption)
	assert.NotNil(t, c)
	assert.NoError(t, err)
	assert.NotNil(t, c.GetMetrics())
	assert.NoError(t, c.Shutdown())
}

func TestGetMetricsNoClusterName(t *testing.T) {
	hostInfo := testutils.MockHostInfo{}
	decoratorOption := WithDecorator(&MockDecorator{})

	c, err := New("eks", hostInfo, zap.NewNop(), cadvisorManagerCreator(newMockCreateManager(t)), decoratorOption)
	assert.NotNil(t, c)
	assert.NoError(t, err)
	assert.Nil(t, c.GetMetrics())
	assert.NoError(t, c.Shutdown())
}

func TestGetMetricsErrorWhenCreatingManager(t *testing.T) {
	hostInfo := testutils.MockHostInfo{ClusterName: "cluster"}
	decoratorOption := WithDecorator(&MockDecorator{})

	c, err := New("eks", hostInfo, zap.NewNop(), cadvisorManagerCreator(mockCreateManagerWithError), decoratorOption)
	assert.Nil(t, c)
	assert.Error(t, err)
}

func TestGetMetricsErrorWhenCallingManagerStart(t *testing.T) {
	hostInfo := testutils.MockHostInfo{ClusterName: "cluster"}
	decoratorOption := WithDecorator(&MockDecorator{})

	c, err := New("eks", hostInfo, zap.NewNop(), cadvisorManagerCreator(mockCreateManager2), decoratorOption)
	assert.Nil(t, c)
	assert.Error(t, err)
}
