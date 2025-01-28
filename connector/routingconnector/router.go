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

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/routingconnector/internal/common"
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
	resourceParser   ottl.Parser[ottlresource.TransformContext]
	spanParser       ottl.Parser[ottlspan.TransformContext]
	metricParser     ottl.Parser[ottlmetric.TransformContext]
	dataPointParser  ottl.Parser[ottldatapoint.TransformContext]
	logParser        ottl.Parser[ottllog.TransformContext]
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
	var buildResource, buildSpan, buildMetric, buildDataPoint, buildLog bool
	for _, item := range table {
		switch item.Context {
		case "", "resource":
			buildResource = true
		case "span":
			buildSpan = true
		case "metric":
			buildMetric = true
		case "datapoint":
			buildDataPoint = true
		case "log":
			buildLog = true
		}
	}

	var errs error
	if buildResource {
		parser, err := ottlresource.NewParser(
			common.Functions[ottlresource.TransformContext](),
			settings,
		)
		if err == nil {
			r.resourceParser = parser
		} else {
			errs = errors.Join(errs, err)
		}
	}
	if buildSpan {
		parser, err := ottlspan.NewParser(
			common.Functions[ottlspan.TransformContext](),
			settings,
		)
		if err == nil {
			r.spanParser = parser
		} else {
			errs = errors.Join(errs, err)
		}
	}
	if buildMetric {
		parser, err := ottlmetric.NewParser(
			common.Functions[ottlmetric.TransformContext](),
			settings,
		)
		if err == nil {
			r.metricParser = parser
		} else {
			errs = errors.Join(errs, err)
		}
	}
	if buildDataPoint {
		parser, err := ottldatapoint.NewParser(
			common.Functions[ottldatapoint.TransformContext](),
			settings,
		)
		if err == nil {
			r.dataPointParser = parser
		} else {
			errs = errors.Join(errs, err)
		}
	}
	if buildLog {
		parser, err := ottllog.NewParser(
			common.Functions[ottllog.TransformContext](),
			settings,
		)
		if err == nil {
			r.logParser = parser
		} else {
			errs = errors.Join(errs, err)
		}
	}
	return errs
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
			switch item.Context {
			case "request":
				route.requestCondition, err = parseRequestCondition(item.Condition)
				if err != nil {
					return err
				}
			case "", "resource":
				statement, err := r.resourceParser.ParseStatement(item.Statement)
				if err != nil {
					return err
				}
				route.resourceStatement = statement
			case "span":
				statement, err := r.spanParser.ParseStatement(item.Statement)
				if err != nil {
					return err
				}
				route.spanStatement = statement
			case "metric":
				statement, err := r.metricParser.ParseStatement(item.Statement)
				if err != nil {
					return err
				}
				route.metricStatement = statement
			case "datapoint":
				statement, err := r.dataPointParser.ParseStatement(item.Statement)
				if err != nil {
					return err
				}
				route.dataPointStatement = statement
			case "log":
				statement, err := r.logParser.ParseStatement(item.Statement)
				if err != nil {
					return err
				}
				route.logStatement = statement
			}
		} else {
			pipelineNames := []string{}
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
