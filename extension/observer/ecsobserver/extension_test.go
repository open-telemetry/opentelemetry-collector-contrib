// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ecsobserver

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
)

type mockNotifier struct {
	endpointsMap map[observer.EndpointID]observer.Endpoint
}

func (m mockNotifier) OnAdd(added []observer.Endpoint) {
	for _, e := range added {
		m.endpointsMap[e.ID] = e
	}
}

func (m mockNotifier) OnRemove(removed []observer.Endpoint) {
	for _, e := range removed {
		delete(m.endpointsMap, e.ID)
	}
}

func (m mockNotifier) OnChange(changed []observer.Endpoint) {
	for _, e := range changed {
		m.endpointsMap[e.ID] = e
	}
}

func TestStartAndStopObserver(t *testing.T) {
	config := createDefaultConfig()
	sd := &serviceDiscovery{config: config.(*Config)}
	sd.init()

	obs := &ecsObserver{
		EndpointsWatcher: observer.EndpointsWatcher{
			RefreshInterval: 10 * time.Second,
			Endpointslister: endpointsLister{
				sd:           sd,
				observerName: "test-observer",
			},
		},
	}

	notifier := mockNotifier{map[observer.EndpointID]observer.Endpoint{}}

	ctx := context.Background()
	require.NoError(t, obs.Start(ctx, componenttest.NewNopHost()))
	obs.ListAndWatch(notifier)

	time.Sleep(2 * time.Second) // Wait a bit to sync endpoints once.
	require.NoError(t, obs.Shutdown(ctx))
}
