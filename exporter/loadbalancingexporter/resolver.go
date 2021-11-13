// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package loadbalancingexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter"

import "context"

// resolver determines the contract for sources of backend endpoint information
type resolver interface {
	// resolve returns the current list of endpoints.
	// returns either a non-nil error and a nil list of endpoints, or a non-nil list of endpoints and nil error.
	resolve(context.Context) ([]string, error)

	// start signals the resolver to start its work
	start(context.Context) error

	// shutdown signals the resolver to finish its work. This should block until the current resolutions are finished.
	// Once this is invoked, callbacks will not be triggered anymore and will need to be registered again in case the consumer
	// decides to restart the resolver.
	shutdown(context.Context) error

	// onChange registers a function to call back whenever the list of backends is updated.
	// Make sure to register the callbacks before starting the exporter.
	onChange(func([]string))
}
