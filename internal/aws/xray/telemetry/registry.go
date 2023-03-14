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

package telemetry // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/xray/telemetry"

import (
	"sync"

	"go.opentelemetry.io/collector/component"

	awsxray "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/xray"
)

type Registry interface {
	Set(id component.ID, recorder Recorder)
	Get(id component.ID) Recorder
	Register(id component.ID, cfg Config, client awsxray.XRayClient, opts ...RecorderOption) Recorder
}

var globalRegistry = NewRegistry()

// GlobalRegistry returns the global Registry.
func GlobalRegistry() Registry {
	return globalRegistry
}

// registry maintains a map of all registered recorders.
type registry struct {
	recorders sync.Map
}

// NewRegistry returns a new empty Registry.
func NewRegistry() Registry {
	return &registry{}
}

// Set if not already set. Otherwise, return the existing.
func (r *registry) Set(id component.ID, recorder Recorder) {
	r.recorders.LoadOrStore(id, recorder)
}

// Get gets the associated recorder for the ID.
func (r *registry) Get(id component.ID) Recorder {
	recorder, ok := r.recorders.Load(id)
	if ok {
		return recorder.(Recorder)
	}
	return nil
}

// Register configures and registers a new Recorder for the ID. If one
// already exists for the ID, then returns that one instead.
func (r *registry) Register(
	id component.ID,
	cfg Config,
	client awsxray.XRayClient,
	opts ...RecorderOption,
) Recorder {
	if recorder, ok := r.recorders.Load(id); ok {
		return recorder.(Recorder)
	}
	recorder := newTelemetryRecorder(client, opts...)
	r.recorders.Store(id, recorder)
	for _, contributor := range cfg.Contributors {
		r.Set(contributor, recorder)
	}
	return recorder
}
