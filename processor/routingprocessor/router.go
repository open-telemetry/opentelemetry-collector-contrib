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

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/telemetryquerylanguage/contexts/tqllogs"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/telemetryquerylanguage/tql"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/routingprocessor/internal/common"
)

var errExporterNotFound = errors.New("exporter not found")

// router registers exporters and default exporters for an exporter. router can
// be instantiated with component.TracesExporter, component.MetricsExporter, and
// component.LogsExporter type arguments.
type router[E component.Exporter] struct {
	logger *zap.Logger
	parser tql.Parser

	defaultExporterIDs []string
	table              []RoutingTableItem

	defaultExporters []E
	routes           map[string]routingItem[E]
}

// newRouter creates a new router instance with its type parameter constrained
// to component.Exporter.
func newRouter[E component.Exporter](
	table []RoutingTableItem,
	defaultExporterIDs []string,
	logger *zap.Logger,
) router[E] {
	return router[E]{
		logger: logger,
		parser: tql.NewParser(
			common.Functions(),
			tqllogs.ParsePath,
			tqllogs.ParseEnum,
			common.NewOTTLLogger(logger),
		),

		table:              table,
		defaultExporterIDs: defaultExporterIDs,

		routes: make(map[string]routingItem[E]),
	}
}

type routingItem[E component.Exporter] struct {
	exporters  []E
	expression tql.Query
}

func (r *router[E]) registerExporters(available map[config.ComponentID]component.Exporter) error {
	// register default exporters
	err := r.registerDefaultExporters(available)
	if err != nil {
		return err
	}

	// register exporters for each route
	err = r.registerRouteExporters(available)
	if err != nil {
		return err
	}

	return nil
}

// registerDefaultExporters registers the configured default exporters
// using the provided available exporters map.
func (r *router[E]) registerDefaultExporters(available map[config.ComponentID]component.Exporter) error {
	for _, name := range r.defaultExporterIDs {
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
func (r *router[E]) registerRouteExporters(available map[config.ComponentID]component.Exporter) error {
	for _, item := range r.table {
		e, err := r.routingExpression(item)
		if err != nil {
			return err
		}

		route, ok := r.routes[key(item)]
		if !ok {
			route.expression = e
		}

		for _, name := range item.Exporters {
			e, err := r.extractExporter(name, available)
			if errors.Is(err, errExporterNotFound) {
				continue
			}
			if err != nil {
				return err
			}
			route.exporters = append(route.exporters, e)
		}
		r.routes[key(item)] = route
	}
	return nil
}

// routingExpression builds a routing OTTL expressions from provided
// routing table entry configuration. If routing table entry configuration
// does not contain a OTTL expressions then nil is returned.
func (r *router[E]) routingExpression(item RoutingTableItem) (tql.Query, error) {
	var e tql.Query
	if item.Expression != "" {
		queries, err := r.parser.ParseQueries([]string{item.Expression})
		if err != nil {
			return e, err
		}
		if len(queries) != 1 {
			return e, errors.New("more than one expression specified")
		}
		e = queries[0]
	}
	return e, nil
}

func key(entry RoutingTableItem) string {
	if entry.Value != "" {
		return entry.Value
	}
	return entry.Expression
}

// extractExporter returns an exporter for the given name (type/name) and type
// argument if it exists in the list of available exporters.
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

func (r *router[E]) getExporters(key string) []E {
	e, ok := r.routes[key]
	if !ok {
		return r.defaultExporters
	}
	return e.exporters
}
