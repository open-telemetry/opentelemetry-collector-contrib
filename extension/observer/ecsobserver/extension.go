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
	"fmt"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"
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
	targets := e.sd.discoverTargets()

	// If there is a result file path configured, we output the scraped targets to a file
	// instead of notifying listeners.
	if filePath := e.sd.config.ResultFile; filePath != "" {
		err := writeTargetsToFile(targets, filePath)
		if err != nil {
			e.sd.config.logger.Error(
				"Failed to write targets to file",
				zap.String("file path", filePath),
				zap.String("error", err.Error()),
			)
		}
		return nil
	}

	// Otherwise, convert to endpoints
	endpoints := make([]observer.Endpoint, len(targets))
	idx := 0
	for _, target := range targets {
		endpoints[idx] = *target.toEndpoint()
		idx++
	}

	return endpoints
}

// writeTargetsToFile writes targets to the given file path.
func writeTargetsToFile(targets map[string]*Target, filePath string) error {
	tmpFilePath := filePath + "_temp"

	// Convert to serializable Prometheus targets
	promTargets := make([]*PrometheusTarget, len(targets))
	idx := 0
	for _, target := range targets {
		promTargets[idx] = target.toPrometheusTarget()
		idx++
	}

	m, err := yaml.Marshal(promTargets)
	if err != nil {
		return fmt.Errorf("Failed to marshal Prometheus targets. Error: %s", err.Error())
	}

	err = ioutil.WriteFile(tmpFilePath, m, 0644)
	if err != nil {
		return fmt.Errorf("Failed to marshal Prometheus targets into file: %s. Error: %s", tmpFilePath, err.Error())
	}

	err = os.Rename(tmpFilePath, filePath)
	if err != nil {
		os.Remove(tmpFilePath)
		return fmt.Errorf("Failed to rename tmp result file %s to %s. Error: %s", tmpFilePath, filePath, err.Error())
	}

	return nil
}
