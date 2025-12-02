// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package routingconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/routingconnector"

import (
	"errors"
	"fmt"
	"strings"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pipeline"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottldatapoint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlresource"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspan"
)

var errPipelineNotFound = errors.New("pipeline not found")

// consumerProvider is a function with a type parameter C (expected to be one
// of consumer.Traces, consumer.Metrics, or Consumer.Logs). returns a
// consumer for the given component ID(s).
type consumerProvider[C any] func(...pipeline.ID) (C, error)

// router registers consumers and default consumers for a pipeline. the type
// parameter C is expected to be one of: consumer.Traces, consumer.Metrics, or
// consumer.Logs.
type router[C any] struct {
	parserCollection *ottl.ParserCollection[any]
	defaultConsumer  C
	logger           *zap.Logger
	routes           map[string]routingItem[C]
	consumerProvider consumerProvider[C]
	table            []RoutingTableItem
	routeSlice       []routingItem[C]
}

// newRouter creates a new router instance with based on type parameters C and K.
// see router struct definition for the allowed types.
func newRouter[C any](
	table []RoutingTableItem,
	defaultPipelineIDs []pipeline.ID,
	provider consumerProvider[C],
	settings component.TelemetrySettings,
) (*router[C], error) {
	r := &router[C]{
		logger:           settings.Logger,
		table:            table,
		routes:           make(map[string]routingItem[C]),
		consumerProvider: provider,
	}

	if err := r.buildParsers(table, settings); err != nil {
		return nil, err
	}

	if err := r.registerConsumers(defaultPipelineIDs); err != nil {
		return nil, err
	}

	return r, nil
}

type routingItem[C any] struct {
	consumer           C
	requestCondition   *requestCondition
	resourceStatement  *ottl.Statement[ottlresource.TransformContext]
	spanStatement      *ottl.Statement[ottlspan.TransformContext]
	metricStatement    *ottl.Statement[ottlmetric.TransformContext]
	dataPointStatement *ottl.Statement[ottldatapoint.TransformContext]
	logStatement       *ottl.Statement[ottllog.TransformContext]
	statementContext   string
}

func (r *router[C]) buildParsers(table []RoutingTableItem, settings component.TelemetrySettings) error {
	// We need to build parsers for all contexts to support inference, regardless of what is explicitly used in the config.
	// Prioritize resource context to maintain backward compatibility for ambiguous paths (e.g. 'attributes').
	priorities := []string{
		"resource",
		"span",
		"spanevent",
		"metric",
		"datapoint",
		"log",
		"scope",
		"instrumentation_scope",
	}

	resourceParser, err := ottlresource.NewParser(
		standardFunctions[ottlresource.TransformContext](),
		settings,
		ottlresource.EnablePathContextNames(),
	)
	if err != nil {
		return err
	}
	spanParser, err := ottlspan.NewParser(
		spanFunctions(),
		settings,
		ottlspan.EnablePathContextNames(),
	)
	if err != nil {
		return err
	}
	metricParser, err := ottlmetric.NewParser(
		standardFunctions[ottlmetric.TransformContext](),
		settings,
		ottlmetric.EnablePathContextNames(),
	)
	if err != nil {
		return err
	}
	dataPointParser, err := ottldatapoint.NewParser(
		standardFunctions[ottldatapoint.TransformContext](),
		settings,
		ottldatapoint.EnablePathContextNames(),
	)
	if err != nil {
		return err
	}
	logParser, err := ottllog.NewParser(
		standardFunctions[ottllog.TransformContext](),
		settings,
		ottllog.EnablePathContextNames(),
	)
	if err != nil {
		return err
	}

	r.parserCollection, err = ottl.NewParserCollection[any](
		settings,
		ottl.WithContextInferrerPriorities[any](priorities),
		ottl.WithParserCollectionContext(
			ottlresource.ContextName,
			&resourceParser,
			ottl.WithStatementConverter(createStatementConverter[ottlresource.TransformContext]()),
		),
		ottl.WithParserCollectionContext(
			ottlspan.ContextName,
			&spanParser,
			ottl.WithStatementConverter(createStatementConverter[ottlspan.TransformContext]()),
		),
		ottl.WithParserCollectionContext(
			ottlmetric.ContextName,
			&metricParser,
			ottl.WithStatementConverter(createStatementConverter[ottlmetric.TransformContext]()),
		),
		ottl.WithParserCollectionContext(
			ottldatapoint.ContextName,
			&dataPointParser,
			ottl.WithStatementConverter(createStatementConverter[ottldatapoint.TransformContext]()),
		),
		ottl.WithParserCollectionContext(
			ottllog.ContextName,
			&logParser,
			ottl.WithStatementConverter(createStatementConverter[ottllog.TransformContext]()),
		),
	)
	return err
}

func createStatementConverter[K any]() ottl.ParsedStatementsConverter[K, any] {
	return func(pc *ottl.ParserCollection[any], statements ottl.StatementsGetter, parsedStatements []*ottl.Statement[K]) (any, error) {
		if len(parsedStatements) != 1 {
			return nil, fmt.Errorf("expected exactly one statement, got %d", len(parsedStatements))
		}
		return parsedStatements[0], nil
	}
}

func (r *router[C]) registerConsumers(defaultPipelineIDs []pipeline.ID) error {
	// register default pipelines
	err := r.registerDefaultConsumer(defaultPipelineIDs)
	if err != nil {
		return err
	}

	r.normalizeConditions()

	// register pipelines for each route
	err = r.registerRouteConsumers()
	if err != nil {
		return err
	}

	return nil
}

// registerDefaultConsumer registers a consumer for the default pipelines configured
func (r *router[C]) registerDefaultConsumer(pipelineIDs []pipeline.ID) error {
	if len(pipelineIDs) == 0 {
		return nil
	}

	consumer, err := r.consumerProvider(pipelineIDs...)
	if err != nil {
		return fmt.Errorf("%w: %s", errPipelineNotFound, err.Error())
	}

	r.defaultConsumer = consumer

	return nil
}

// convert conditions to statements
func (r *router[C]) normalizeConditions() {
	for i := range r.table {
		item := &r.table[i]
		if item.Condition != "" {
			item.Statement = fmt.Sprintf("route() where %s", item.Condition)
		}
	}
}

// registerRouteConsumers registers a consumer for the pipelines configured for each route
func (r *router[C]) registerRouteConsumers() (err error) {
	for _, item := range r.table {
		route, ok := r.routes[key(item)]
		if !ok {
			route.statementContext = item.Context
			if item.Context == "request" {
				route.requestCondition, err = parseRequestCondition(item.Condition)
				if err != nil {
					return err
				}
			} else {
				statementsGetter := ottl.NewStatementsGetter([]string{item.Statement})
				var result any
				if item.Context == "" {
					// Context is empty, try to infer it
					// Default to resource context if inference fails or ambiguous (though priorities handle ambiguity)
					result, err = r.parserCollection.ParseStatements(statementsGetter, ottl.WithDefaultContext(ottlresource.ContextName))
				} else {
					// Context is explicit
					result, err = r.parserCollection.ParseStatementsWithContext(item.Context, statementsGetter, true)
				}

				if err != nil {
					return err
				}

				// Type switch to assign the correct statement
				switch s := result.(type) {
				case *ottl.Statement[ottlresource.TransformContext]:
					route.resourceStatement = s
					route.statementContext = "resource"
				case *ottl.Statement[ottlspan.TransformContext]:
					route.spanStatement = s
					route.statementContext = "span"
				case *ottl.Statement[ottlmetric.TransformContext]:
					route.metricStatement = s
					route.statementContext = "metric"
				case *ottl.Statement[ottldatapoint.TransformContext]:
					route.dataPointStatement = s
					route.statementContext = "datapoint"
				case *ottl.Statement[ottllog.TransformContext]:
					route.logStatement = s
					route.statementContext = "log"
				default:
					return fmt.Errorf("unexpected statement type: %T", result)
				}
			}
		} else {
			var pipelineNames []string
			for _, pipeline := range item.Pipelines {
				pipelineNames = append(pipelineNames, pipeline.String())
			}
			exporters := strings.Join(pipelineNames, ", ")
			r.logger.Warn(fmt.Sprintf(`Statement %q already exists in the routing table, the route with target pipeline(s) %q will be ignored.`, item.Statement, exporters))
		}

		consumer, err := r.consumerProvider(item.Pipelines...)
		if err != nil {
			return fmt.Errorf("%w: %s", errPipelineNotFound, err.Error())
		}
		route.consumer = consumer
		if !ok {
			r.routeSlice = append(r.routeSlice, route)
		}

		r.routes[key(item)] = route
	}
	return nil
}

func key(entry RoutingTableItem) string {
	switch entry.Context {
	case "", "resource":
		return entry.Statement
	case "request":
		return "[request] " + entry.Condition
	}
	if entry.Context == "" || entry.Context == "resource" {
		return entry.Statement
	}
	return "[" + entry.Context + "] " + entry.Statement
}
