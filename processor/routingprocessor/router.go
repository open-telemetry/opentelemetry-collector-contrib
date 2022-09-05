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

package routingprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/routingprocessor"

import (
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.uber.org/zap"
)

var (
	errDefaultExporterNotFound = errors.New("default exporter not found")
	errExporterNotFound        = errors.New("exporter not found")
)

// router registers exporters and default exporters for an exporter. router can
// be instantiated with component.TracesExporter, component.MetricsExporter, and
// component.LogsExporter type arguments.
type router[E component.Exporter] struct {
	config Config
	logger *zap.Logger

	defaultExporters []E
	exporters        map[string][]E
}

// newRouter creates a new router instance with its type parameter constrained
// to component.Exporter.
func newRouter[E component.Exporter](config Config, logger *zap.Logger) router[E] {
	return router[E]{
		logger: logger,
		config: config,

		exporters: make(map[string][]E),
	}
}

func (r *router[E]) registerExporters(exporters map[config.ComponentID]component.Exporter) error {
	available := make(map[string]component.Exporter)
	for id, exp := range exporters {
		exporter, ok := exp.(E)
		if !ok {
			return fmt.Errorf("the exporter %q isn't a %T exporter", id.String(), new(E))
		}
		available[id.String()] = exporter
	}

	// default exporters
	r.registerDefaultExporters(available)

	// exporters for each route
	for _, entry := range r.config.Table {
		r.registerRouteExporters(entry.Value, available, entry.Exporters)
	}

	return nil
}

// registerDefaultExporters registers the configured default exporters
// using the provided available exporters map.
func (r *router[E]) registerDefaultExporters(availableExporters map[string]component.Exporter) {
	for _, e := range r.config.DefaultExporters {
		v, ok := availableExporters[e]
		if !ok {
			r.logger.Warn(
				"Can't find the exporter for the routing processor for this pipeline type."+
					" This is OK if you did not specify this processor for that pipeline type",
				zap.Any("pipeline_type", new(E)),
				zap.Error(
					fmt.Errorf(
						"error registering default exporter %q: %w",
						e,
						errDefaultExporterNotFound,
					),
				),
			)
			continue
		}
		r.defaultExporters = append(r.defaultExporters, v.(E))
	}
}

// registerRouteExporters registers the requested exporters using the provided
// available exporters map to check if they were available.
func (r *router[E]) registerRouteExporters(
	route string,
	availableExporters map[string]component.Exporter,
	exporters []string,
) {
	r.logger.Debug("Registering exporter for route",
		zap.String("route", route),
		zap.Any("requested", exporters),
	)

	for _, e := range exporters {
		v, ok := availableExporters[e]
		if !ok {
			r.logger.Warn(
				"Can't find the exporter for the routing processor for this pipeline type."+
					" This is OK if you did not specify this processor for that pipeline type",
				zap.Any("pipeline_type", new(E)),
				zap.Error(
					fmt.Errorf(
						"error registering route %q for exporter %q: %w",
						route,
						e,
						errExporterNotFound,
					),
				),
			)
			continue
		}
		r.exporters[route] = append(r.exporters[route], v.(E))
	}
}
