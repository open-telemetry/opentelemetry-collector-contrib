// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mongodbatlasreceiver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/atlas/mongodbatlas"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/extension/experimental/storage"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbatlasreceiver/internal"
)

func TestStartAndShutdown(t *testing.T) {
	cases := []struct {
		desc                string
		getConfig           func() *Config
		expectedStartErr    error
		expectedShutdownErr error
	}{
		{
			desc: "valid config",
			getConfig: func() *Config {
				cfg := createDefaultConfig().(*Config)
				cfg.Events = &EventsConfig{
					Projects: []*ProjectConfig{
						{
							Name: testProjectName,
						},
					},
					PollInterval: time.Minute,
				}
				return cfg
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			sink := &consumertest.LogsSink{}
			r := newEventsReceiver(receivertest.NewNopCreateSettings(), tc.getConfig(), sink)
			err := r.Start(context.Background(), componenttest.NewNopHost(), storage.NewNopClient())
			if tc.expectedStartErr != nil {
				require.ErrorContains(t, err, tc.expectedStartErr.Error())
			} else {
				require.NoError(t, err)
			}
			err = r.Shutdown(context.Background())
			if tc.expectedShutdownErr != nil {
				require.ErrorContains(t, err, tc.expectedShutdownErr.Error())
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestContextDone(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Events = &EventsConfig{
		Projects: []*ProjectConfig{
			{
				Name: testProjectName,
			},
		},
	}
	sink := &consumertest.LogsSink{}
	r := newEventsReceiver(receivertest.NewNopCreateSettings(), cfg, sink)
	r.pollInterval = 500 * time.Millisecond
	mClient := &mockEventsClient{}
	mClient.setupMock(t)
	r.client = mClient

	ctx, cancel := context.WithCancel(context.Background())
	err := r.Start(ctx, componenttest.NewNopHost(), storage.NewNopClient())
	require.NoError(t, err)
	cancel()

	require.Never(t, func() bool {
		return sink.LogRecordCount() > 0
	}, 2*time.Second, 500*time.Millisecond)

	err = r.Shutdown(context.Background())
	require.NoError(t, err)
}

func TestPoll(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Events = &EventsConfig{
		Projects: []*ProjectConfig{
			{
				Name: testProjectName,
			},
		},
		Organizations: []*OrgConfig{
			{
				ID: testOrgID,
			},
		},
		PollInterval: time.Second,
	}

	sink := &consumertest.LogsSink{}
	r := newEventsReceiver(receivertest.NewNopCreateSettings(), cfg, sink)
	mClient := &mockEventsClient{}
	mClient.setupMock(t)
	r.client = mClient

	err := r.Start(context.Background(), componenttest.NewNopHost(), storage.NewNopClient())
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return sink.LogRecordCount() > 0
	}, 5*time.Second, 1*time.Second)

	err = r.Shutdown(context.Background())
	require.NoError(t, err)

	expectedProjectLogs, err := golden.ReadLogs(filepath.Join("testdata", "events", "golden", "project-events.yaml"))
	require.NoError(t, err)

	expectedOrgLogs, err := golden.ReadLogs(filepath.Join("testdata", "events", "golden", "org-events.yaml"))
	require.NoError(t, err)

	projectLogs := sink.AllLogs()[0]
	require.NoError(t, plogtest.CompareLogs(expectedProjectLogs, projectLogs, plogtest.IgnoreObservedTimestamp()))

	orgLogs := sink.AllLogs()[1]
	require.NoError(t, plogtest.CompareLogs(expectedOrgLogs, orgLogs, plogtest.IgnoreObservedTimestamp()))
}

func TestProjectGetFailure(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Events = &EventsConfig{
		Projects: []*ProjectConfig{
			{
				Name: "fake-project",
			},
		},
		Organizations: []*OrgConfig{
			{
				ID: "fake-org",
			},
		},
		PollInterval: time.Second,
	}

	sink := &consumertest.LogsSink{}
	r := newEventsReceiver(receivertest.NewNopCreateSettings(), cfg, sink)
	mClient := &mockEventsClient{}
	mClient.On("GetProject", mock.Anything, "fake-project").Return(nil, fmt.Errorf("unable to get project: %d", http.StatusUnauthorized))
	mClient.On("GetOrganization", mock.Anything, "fake-org").Return(nil, fmt.Errorf("unable to get org: %d", http.StatusUnauthorized))

	err := r.Start(context.Background(), componenttest.NewNopHost(), storage.NewNopClient())
	require.NoError(t, err)

	require.Never(t, func() bool {
		return sink.LogRecordCount() > 0
	}, 2*time.Second, 500*time.Millisecond)

	err = r.Shutdown(context.Background())
	require.NoError(t, err)
}

type mockEventsClient struct {
	mock.Mock
}

func (mec *mockEventsClient) setupMock(t *testing.T) {
	mec.setupGetProject()
	mec.setupGetOrganization()
	mec.On("GetProjectEvents", mock.Anything, mock.Anything, mock.Anything).Return(mec.loadTestEvents(t, "project-events.json"), false, nil)
	mec.On("GetOrganizationEvents", mock.Anything, mock.Anything, mock.Anything).Return(mec.loadTestEvents(t, "org-events.json"), false, nil)
}

func (mec *mockEventsClient) setupGetProject() {
	mec.On("GetProject", mock.Anything, mock.Anything).Return(&mongodbatlas.Project{
		ID:    testProjectID,
		OrgID: testOrgID,
		Name:  testProjectName,
		Links: []*mongodbatlas.Link{},
	}, nil)
}

func (mec *mockEventsClient) setupGetOrganization() {
	mec.On("GetOrganization", mock.Anything, mock.Anything).Return(&mongodbatlas.Organization{
		ID:    testOrgID,
		Links: []*mongodbatlas.Link{},
	}, nil)
}

func (mec *mockEventsClient) loadTestEvents(t *testing.T, filename string) []*mongodbatlas.Event {
	testEvents := filepath.Join("testdata", "events", "sample-payloads", filename)
	eventBytes, err := os.ReadFile(testEvents)
	require.NoError(t, err)

	var events []*mongodbatlas.Event
	err = json.Unmarshal(eventBytes, &events)
	require.NoError(t, err)
	return events
}

func (mec *mockEventsClient) GetProject(ctx context.Context, pID string) (*mongodbatlas.Project, error) {
	args := mec.Called(ctx, pID)
	return args.Get(0).(*mongodbatlas.Project), args.Error(1)
}

func (mec *mockEventsClient) GetProjectEvents(ctx context.Context, pID string, opts *internal.GetEventsOptions) ([]*mongodbatlas.Event, bool, error) {
	args := mec.Called(ctx, pID, opts)
	return args.Get(0).([]*mongodbatlas.Event), args.Bool(1), args.Error(2)
}

func (mec *mockEventsClient) GetOrganization(ctx context.Context, oID string) (*mongodbatlas.Organization, error) {
	args := mec.Called(ctx, oID)
	return args.Get(0).(*mongodbatlas.Organization), args.Error(1)
}

func (mec *mockEventsClient) GetOrganizationEvents(ctx context.Context, oID string, opts *internal.GetEventsOptions) ([]*mongodbatlas.Event, bool, error) {
	args := mec.Called(ctx, oID, opts)
	return args.Get(0).([]*mongodbatlas.Event), args.Bool(1), args.Error(2)
}
