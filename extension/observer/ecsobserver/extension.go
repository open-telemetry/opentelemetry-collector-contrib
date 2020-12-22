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

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
	"go.opentelemetry.io/collector/component"
)

type ecsObserver struct {
	observer.EndpointsWatcher
}

type endpointsLister struct {
	sd           *serviceDiscovery
	observerName string
}

var _ component.ServiceExtension = (*ecsObserver)(nil)

func newObserver(config *Config) (component.ServiceExtension, error) {
	sd := &serviceDiscovery{config: config}
	sd.init()
	h := &ecsObserver{
		EndpointsWatcher: observer.EndpointsWatcher{
			RefreshInterval: config.RefreshInterval,
			Endpointslister: endpointsLister{
				sd:           sd,
				observerName: config.Name(),
			},
		},
	}

	return h, nil
}

func (h *ecsObserver) Start(context.Context, component.Host) error {
	return nil
}

func (h *ecsObserver) Shutdown(context.Context) error {
	h.StopListAndWatch()
	return nil
}

func (e endpointsLister) ListEndpoints() []observer.Endpoint {
	// TODO: Implement this
	return nil
}
