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

var errExporterNotFound = errors.New("exporter not found")

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

func (r *router[E]) registerExporters(available map[config.ComponentID]component.Exporter) error {
	// register default exporters
	err := r.registerDefaultExporters(available)
	if err != nil {
		return err
	}

	// register exporters for each route
	for _, entry := range r.config.Table {
		err := r.registerRouteExporters(entry.Value, entry.Exporters, available)
		if err != nil {
			return err
		}
	}

	return nil
}

// registerDefaultExporters registers the configured default exporters
// using the provided available exporters map.
func (r *router[E]) registerDefaultExporters(available map[config.ComponentID]component.Exporter) error {
	for _, name := range r.config.DefaultExporters {
		e, err := r.extractExporter(name, available)
		if errors.Is(err, errExporterNotFound) {
			continue
		}
		if err != nil {
			return err
		}
		r.defaultExporters = append(r.defaultExporters, e)
	}

	return nil
}

// registerRouteExporters registers route exporters using the provided
// available exporters map to check if they were available.
func (r *router[E]) registerRouteExporters(
	route string,
	exporters []string,
	available map[config.ComponentID]component.Exporter,
) error {
	for _, name := range exporters {
		e, err := r.extractExporter(name, available)
		if errors.Is(err, errExporterNotFound) {
			continue
		}
		if err != nil {
			return err
		}
		r.exporters[route] = append(r.exporters[route], e)
	}

	return nil
}

func (r *router[E]) extractExporter(name string, available map[config.ComponentID]component.Exporter) (E, error) {
	var exporter E

	id, err := config.NewComponentIDFromString(name)
	if err != nil {
		return exporter, err
	}
	v, ok := available[id]
	if !ok {
		r.logger.Warn(
			"Can't find the exporter for the routing processor for this pipeline type."+
				" This is OK if you did not specify this processor for that pipeline type",
			zap.Any("pipeline_type", new(E)),
			zap.Error(
				fmt.Errorf(
					"error registering exporter %q",
					name,
				),
			),
		)
		return exporter, errExporterNotFound
	}
	exporter, ok = v.(E)
	if !ok {
		return exporter,
			fmt.Errorf("the exporter %q isn't a %T exporter", id.String(), new(E))
	}
	return exporter, nil
}
