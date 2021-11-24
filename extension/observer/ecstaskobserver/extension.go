// Copyright  The OpenTelemetry Authors
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

package ecstaskobserver // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/ecstaskobserver"

import (
	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
)

var _ component.Extension = (*ecsTaskObserver)(nil)
var _ observer.EndpointsLister = (*ecsTaskObserver)(nil)

type ecsTaskObserver struct {
	component.Extension
	config           *Config
	endpointsWatcher *observer.EndpointsWatcher
	telemetry        component.TelemetrySettings
}

// ListEndpoints is invoked by an observer.EndpointsWatcher helper to report task container endpoints.
// It's required to implement observer.EndpointsLister
func (e *ecsTaskObserver) ListEndpoints() []observer.Endpoint { return nil }
