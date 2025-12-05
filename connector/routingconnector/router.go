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
	resourceStatement  *ottl.Statement[ottlresource.TransformContext]
	spanStatement      *ottl.Statement[*ottlspan.TransformContext]
	metricStatement    *ottl.Statement[ottlmetric.TransformContext]
	dataPointStatement *ottl.Statement[ottldatapoint.TransformContext]
	logStatement       *ottl.Statement[ottllog.TransformContext]
	statementContext   string
}

func (r *router[C]) buildParsers(_ []RoutingTableItem, settings component.TelemetrySettings) error {
	// Context inference priority list: when a condition uses an ambiguous path (one that exists
	// in multiple contexts), the inferrer tries each context in order until one can parse it.
	//
	// "resource" is first for backward compatibility: before context inference existed, the
	// routing connector only supported resource context. Existing configs with conditions like
	// `attributes["env"] == "prod"` must continue to resolve to resource.attributes.
	//
	// The remaining order matters less in practice because most ambiguous paths (like "attributes")
	// exist in resource anyway, and non-ambiguous paths (like "body", "severity_text", "name")
	// only exist in one context regardless of priority. That said, the order is sorted based on which
	// events are most common in practice, hence 'span' is first.
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

	// Create all parsers upfront. This follows the pattern used by other OTTL-using components
	// like the transform processor. The OTTL context inferrer needs access to all context
	// parsers to properly determine which context to use based on paths, functions, and enums.
	// The one-time initialization cost is minimal compared to the complexity and fragility
	// of trying to pre-determine which contexts are needed via statement inspection.
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

	r.parserCollection, err = ottl.NewParserCollection(
		settings,
		ottl.WithContextInferrerPriorities[any](priorities),
		ottl.WithParserCollectionContext(
			ottlresource.ContextName,
			&resourceParser,
			ottl.WithStatementConverter(singleStatementConverter[ottlresource.TransformContext]()),
		),
		ottl.WithParserCollectionContext(
			ottlspan.ContextName,
			&spanParser,
			ottl.WithStatementConverter(singleStatementConverter[*ottlspan.TransformContext]()),
		),
		ottl.WithParserCollectionContext(
			ottlmetric.ContextName,
			&metricParser,
			ottl.WithStatementConverter(singleStatementConverter[ottlmetric.TransformContext]()),
		),
		ottl.WithParserCollectionContext(
			ottldatapoint.ContextName,
			&dataPointParser,
			ottl.WithStatementConverter(singleStatementConverter[ottldatapoint.TransformContext]()),
		),
		ottl.WithParserCollectionContext(
			ottllog.ContextName,
			&logParser,
			ottl.WithStatementConverter(singleStatementConverter[ottllog.TransformContext]()),
		),
	)
	return err
}

// singleStatementConverter extracts a single parsed statement from the parser output.
// Unlike the transform processor which works with statement sequences, the routing connector
// evaluates one statement per route to determine where data should be routed.
//
// The length check is technically redundant since registerRouteConsumers always passes exactly
// one statement to the parser, and the OTTL parser produces one parsed statement per input.
// However, it serves as defense-in-depth against future bugs in either this code or the OTTL library.
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
		route, dupeFound := r.routes[key(item)]
		if dupeFound {
			var pipelineNames []string
			for _, pipeline := range item.Pipelines {
				pipelineNames = append(pipelineNames, pipeline.String())
			}
			exporters := strings.Join(pipelineNames, ", ")
			r.logger.Warn(fmt.Sprintf(`Statement %q already exists in the routing table, the route with target pipeline(s) %q will be ignored.`, item.Statement, exporters))
			// Without this continue, the duplicate's pipelines would overwrite the original
			// route's consumer, contradicting the warning message above.
			continue
		}

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
				result, err = r.parserCollection.ParseStatementsWithContext(item.Context, statementsGetter)
			}

			if err != nil {
				return err
			}

			// singleStatementConverter returns the single parsed *ottl.Statement[K]
			switch s := result.(type) {
			case *ottl.Statement[ottlresource.TransformContext]:
				route.resourceStatement = s
				route.statementContext = "resource"
			case *ottl.Statement[*ottlspan.TransformContext]:
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

		consumer, err := r.consumerProvider(item.Pipelines...)
		if err != nil {
			return fmt.Errorf("%w: %s", errPipelineNotFound, err.Error())
		}
		route.consumer = consumer
		if !dupeFound {
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
	default:
		return "[" + entry.Context + "] " + entry.Statement
	}
}
