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

	"github.com/aws/aws-sdk-go/aws/session"
	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/awsutil"
	awsxray "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/xray"
)

var globalRegistry = NewRegistry()

// GlobalRegistry returns the global Registry.
func GlobalRegistry() *Registry {
	return globalRegistry
}

// Registry maintains a map of all registered Recorders.
type Registry struct {
	recorders sync.Map
}

// NewRegistry returns a new empty Registry.
func NewRegistry() *Registry {
	return &Registry{}
}

// Set if not already set. Otherwise, return the existing.
func (r *Registry) Set(id component.ID, recorder Recorder) {
	r.recorders.LoadOrStore(id, recorder)
}

// Get gets the associated recorder for the ID.
func (r *Registry) Get(id component.ID) Recorder {
	recorder, ok := r.recorders.Load(id)
	if ok {
		return recorder.(Recorder)
	}
	return nil
}

// Register configures and registers a new Recorder for the ID. If one
// already exists for the ID, then returns that one instead.
func (r *Registry) Register(
	id component.ID,
	client awsxray.XRayClient,
	sess *session.Session,
	cfg *Config,
	settings *awsutil.AWSSessionSettings,
) Recorder {
	if recorder, ok := r.recorders.Load(id); ok {
		return recorder.(Recorder)
	}
	recorder := newTelemetryRecorder(client, sess, cfg, settings)
	r.recorders.Store(id, recorder)
	for _, contributor := range cfg.Contributors {
		r.Set(contributor, recorder)
	}
	return recorder
}
