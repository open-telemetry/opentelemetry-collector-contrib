// Copyright  OpenTelemetry Authors
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

package ecsobserver

import (
	"context"
)

// MockFecther allows running ServiceDiscovery without using actual TaskFetcher.
type MockFetcher struct {
	// use a factory instead of static tasks because filter and exporter
	// will udpate tasks in place. This avoid introducing a deep copy library for unt test.
	factory func() ([]*Task, error)
}

func newMockFetcher(tasksFactory func() ([]*Task, error)) *MockFetcher {
	return &MockFetcher{factory: tasksFactory}
}

// FetchAndDecorate calls factory to create a new list of task everytime.
func (m *MockFetcher) FetchAndDecorate(_ context.Context) ([]*Task, error) {
	return m.factory()
}
