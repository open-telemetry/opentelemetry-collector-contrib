package k8sleaderelectortest

import (
	"context"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/k8sleaderelector"
	"github.com/stretchr/testify/mock"
	"go.opentelemetry.io/collector/component"
)

type MockHost struct {
	mock.Mock
}

func (m *MockHost) GetExtensions() map[component.ID]component.Component {
	args := m.Called()
	return args.Get(0).(map[component.ID]component.Component)
}

// MockLeaderElection mocks the k8sleaderelector.LeaderElection interface.
type MockLeaderElection struct {
	mock.Mock
}

func (m *MockLeaderElection) SetCallBackFuncs(onStartLeading k8sleaderelector.StartCallback, onStopLeading k8sleaderelector.StopCallback) {
	m.Called(onStartLeading, onStopLeading)
}

func (m *MockLeaderElection) Start(ctx context.Context, host component.Host) error {
	args := m.Called(ctx, host)
	return args.Error(0)
}

func (m *MockLeaderElection) Shutdown(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}
