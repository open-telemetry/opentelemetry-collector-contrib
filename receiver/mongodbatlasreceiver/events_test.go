// Copyright The OpenTelemetry Authors
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

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/golden"
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

	expected, err := golden.ReadLogs(filepath.Join("testdata", "events", "golden", "events.json"))
	require.NoError(t, err)

	logs := sink.AllLogs()[0]
	require.NoError(t, plogtest.CompareLogs(expected, logs, plogtest.IgnoreObservedTimestamp()))
}

func TestProjectGetFailure(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Events = &EventsConfig{
		Projects: []*ProjectConfig{
			{
				Name: "fake-project",
			},
		},
		PollInterval: time.Second,
	}

	sink := &consumertest.LogsSink{}
	r := newEventsReceiver(receivertest.NewNopCreateSettings(), cfg, sink)
	mClient := &mockEventsClient{}
	mClient.On("GetProject", mock.Anything, "fake-project").Return(nil, fmt.Errorf("unable to get project: %d", http.StatusUnauthorized))

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
	mec.On("GetEvents", mock.Anything, mock.Anything, mock.Anything).Return(mec.loadTestEvents(t), false, nil)
}

func (mec *mockEventsClient) setupGetProject() {
	mec.On("GetProject", mock.Anything, mock.Anything).Return(&mongodbatlas.Project{
		ID:    testProjectID,
		OrgID: testOrgID,
		Name:  testProjectName,
		Links: []*mongodbatlas.Link{},
	}, nil)
}

func (mec *mockEventsClient) loadTestEvents(t *testing.T) []*mongodbatlas.Event {
	testEvents := filepath.Join("testdata", "events", "sample-payloads", "events.json")
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

func (mec *mockEventsClient) GetEvents(ctx context.Context, pID string, opts *internal.GetEventsOptions) ([]*mongodbatlas.Event, bool, error) {
	args := mec.Called(ctx, pID, opts)
	return args.Get(0).([]*mongodbatlas.Event), args.Bool(1), args.Error(2)
}
