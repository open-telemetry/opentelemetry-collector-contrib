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
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlotelcol"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlresource"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspan"
)

var (
	errPipelineNotFound       = errors.New("pipeline not found")
	errStatementCountMismatch = errors.New("expected exactly one statement")
)

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
	otelcolStatement   *ottl.Statement[*ottlotelcol.TransformContext]
	resourceStatement  *ottl.Statement[*ottlresource.TransformContext]
	spanStatement      *ottl.Statement[*ottlspan.TransformContext]
	metricStatement    *ottl.Statement[*ottlmetric.TransformContext]
	dataPointStatement *ottl.Statement[*ottldatapoint.TransformContext]
	logStatement       *ottl.Statement[*ottllog.TransformContext]
	statementText      string
	statementContext   string
	action             Action
}

func (r *router[C]) buildParsers(_ []RoutingTableItem, settings component.TelemetrySettings) error {
	otelcolParser, err := ottlotelcol.NewParser(
		standardFunctions[*ottlotelcol.TransformContext](),
		settings,
		ottlotelcol.EnablePathContextNames(),
	)
	if err != nil {
		return err
	}
	resourceParser, err := ottlresource.NewParser(
		standardFunctions[*ottlresource.TransformContext](),
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
		standardFunctions[*ottlmetric.TransformContext](),
		settings,
		ottlmetric.EnablePathContextNames(),
	)
	if err != nil {
		return err
	}
	dataPointParser, err := ottldatapoint.NewParser(
		standardFunctions[*ottldatapoint.TransformContext](),
		settings,
		ottldatapoint.EnablePathContextNames(),
	)
	if err != nil {
		return err
	}
	logParser, err := ottllog.NewParser(
		standardFunctions[*ottllog.TransformContext](),
		settings,
		ottllog.EnablePathContextNames(),
	)
	if err != nil {
		return err
	}

	r.parserCollection, err = ottl.NewParserCollection(
		settings,
		ottl.EnableParserCollectionModifiedPathsLogging[any](true),
		ottl.WithParserCollectionContext(
			ottlotelcol.ContextName,
			&otelcolParser,
			ottl.WithStatementConverter(singleStatementConverter[*ottlotelcol.TransformContext]()),
		),
		ottl.WithParserCollectionContext(
			ottlresource.ContextName,
			&resourceParser,
			ottl.WithStatementConverter(singleStatementConverter[*ottlresource.TransformContext]()),
		),
		ottl.WithParserCollectionContext(
			ottlspan.ContextName,
			&spanParser,
			ottl.WithStatementConverter(singleStatementConverter[*ottlspan.TransformContext]()),
		),
		ottl.WithParserCollectionContext(
			ottlmetric.ContextName,
			&metricParser,
			ottl.WithStatementConverter(singleStatementConverter[*ottlmetric.TransformContext]()),
		),
		ottl.WithParserCollectionContext(
			ottldatapoint.ContextName,
			&dataPointParser,
			ottl.WithStatementConverter(singleStatementConverter[*ottldatapoint.TransformContext]()),
		),
		ottl.WithParserCollectionContext(
			ottllog.ContextName,
			&logParser,
			ottl.WithStatementConverter(singleStatementConverter[*ottllog.TransformContext]()),
		),
	)
	return err
}

// singleStatementConverter extracts a single parsed statement from the parser output.
func singleStatementConverter[K any]() ottl.ParsedStatementsConverter[K, any] {
	return func(_ *ottl.ParserCollection[any], _ ottl.StatementsGetter, parsedStatements []*ottl.Statement[K]) (any, error) {
		if len(parsedStatements) != 1 {
			return nil, fmt.Errorf("%w: got %d", errStatementCountMismatch, len(parsedStatements))
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
		var route routingItem[C]

		// Parse first so statementContext is resolved before the duplicate check.
		if item.Context == "request" {
			r.logger.Warn("The 'request' context is deprecated. Use 'otelcol.client.metadata[\"key\"]' "+
				"(HTTP/client metadata) or 'otelcol.grpc.metadata[\"key\"]' (gRPC metadata) instead.",
				zap.String("condition", item.Condition))
			route.requestCondition, err = parseRequestCondition(item.Condition)
			if err != nil {
				return err
			}
			route.statementContext = "request"
			route.statementText = item.Condition
		} else {
			statementsGetter := ottl.NewStatementsGetter([]string{item.Statement})
			var result any
			if item.Context == "" {
				// Try context inference first. If that fails, fall back to "resource" for
				// backward compatibility: unqualified paths (e.g. attributes["x"]) previously
				// implied resource context. Emit a warning so users can migrate to explicit
				// context-qualified paths (e.g. resource.attributes["x"]).
				result, err = r.parserCollection.ParseStatements(statementsGetter)
				if err != nil {
					result, err = r.parserCollection.ParseStatementsWithContext(ottlresource.ContextName, statementsGetter, true)
				}
			} else {
				result, err = r.parserCollection.ParseStatementsWithContext(item.Context, statementsGetter, true)
			}

			if err != nil {
				return err
			}

			// singleStatementConverter returns the single parsed *ottl.Statement[K]
			switch s := result.(type) {
			case *ottl.Statement[*ottlotelcol.TransformContext]:
				route.otelcolStatement = s
				route.statementContext = ottlotelcol.ContextName
				route.statementText = s.String()
			case *ottl.Statement[*ottlresource.TransformContext]:
				route.resourceStatement = s
				route.statementContext = ottlresource.ContextName
				route.statementText = s.String()
			case *ottl.Statement[*ottlspan.TransformContext]:
				route.spanStatement = s
				route.statementContext = ottlspan.ContextName
				route.statementText = s.String()
			case *ottl.Statement[*ottlmetric.TransformContext]:
				route.metricStatement = s
				route.statementContext = ottlmetric.ContextName
				route.statementText = s.String()
			case *ottl.Statement[*ottldatapoint.TransformContext]:
				route.dataPointStatement = s
				route.statementContext = ottldatapoint.ContextName
				route.statementText = s.String()
			case *ottl.Statement[*ottllog.TransformContext]:
				route.logStatement = s
				route.statementContext = ottllog.ContextName
				route.statementText = s.String()
			default:
				return fmt.Errorf("unexpected statement type: %T", result)
			}
		}

		// Use the resolved context for the key so that an explicit context and its
		// inferred equivalent are treated as the same route.
		k := key(route.statementContext, route.statementText)
		if _, dupeFound := r.routes[k]; dupeFound {
			var pipelineNames []string
			for _, p := range item.Pipelines {
				pipelineNames = append(pipelineNames, p.String())
			}
			exporters := strings.Join(pipelineNames, ", ")
			r.logger.Warn(fmt.Sprintf(`Statement %q already exists in the routing table, the route with target pipeline(s) %q will be ignored.`, item.Statement, exporters))
			continue
		}

		route.action = item.Action

		consumer, err := r.consumerProvider(item.Pipelines...)
		if err != nil {
			return fmt.Errorf("%w: %s", errPipelineNotFound, err.Error())
		}
		route.consumer = consumer
		r.routeSlice = append(r.routeSlice, route)

		r.routes[k] = route
	}
	return nil
}

func key(resolvedContext, ottlText string) string {
	return "[" + resolvedContext + "] " + ottlText
}
