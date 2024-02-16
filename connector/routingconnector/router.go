// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package routingconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/routingconnector"

import (
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/routingconnector/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlresource"
)

var errPipelineNotFound = errors.New("pipeline not found")

// consumerProvider is a function with a type parameter C (expected to be one
// of consumer.Traces, consumer.Metrics, or Consumer.Logs). returns a
// consumer for the given component ID(s).
type consumerProvider[C any] func(...component.ID) (C, error)

// router registers consumers and default consumers for a pipeline. the type
// parameter C is expected to be one of: consumer.Traces, consumer.Metrics, or
// consumer.Logs.
type router[C any] struct {
	logger *zap.Logger
	parser ottl.Parser[ottlresource.TransformContext]

	table      []RoutingTableItem
	routes     map[string]routingItem[C]
	routeSlice []routingItem[C]

	defaultConsumer  C
	consumerProvider consumerProvider[C]
}

// newRouter creates a new router instance with based on type parameters C and K.
// see router struct definition for the allowed types.
func newRouter[C any](
	table []RoutingTableItem,
	defaultPipelineIDs []component.ID,
	provider consumerProvider[C],
	settings component.TelemetrySettings,
) (*router[C], error) {
	parser, err := ottlresource.NewParser(
		common.Functions[ottlresource.TransformContext](),
		settings,
	)

	if err != nil {
		return nil, err
	}

	r := &router[C]{
		logger:           settings.Logger,
		parser:           parser,
		table:            table,
		routes:           make(map[string]routingItem[C]),
		consumerProvider: provider,
	}

	if err := r.registerConsumers(defaultPipelineIDs); err != nil {
		return nil, err
	}

	return r, nil
}

type routingItem[C any] struct {
	consumer  C
	statement *ottl.Statement[ottlresource.TransformContext]
}

func (r *router[C]) registerConsumers(defaultPipelineIDs []component.ID) error {
	// register default pipelines
	err := r.registerDefaultConsumer(defaultPipelineIDs)
	if err != nil {
		return err
	}

	// register pipelines for each route
	err = r.registerRouteConsumers()
	if err != nil {
		return err
	}

	return nil
}

// registerDefaultConsumer registers a consumer for the default
// pipelines configured
func (r *router[C]) registerDefaultConsumer(pipelineIDs []component.ID) error {
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

// registerRouteConsumers registers a consumer for the pipelines configured
// for each route
func (r *router[C]) registerRouteConsumers() error {
	for _, item := range r.table {
		statement, err := r.getStatementFrom(item)
		if err != nil {
			return err
		}

		route, ok := r.routes[key(item)]
		if !ok {
			route.statement = statement
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

// getStatementFrom builds a routing OTTL statement from the provided
// routing table entry configuration. If the routing table entry configuration
// does not contain a valid OTTL statement then nil is returned.
func (r *router[C]) getStatementFrom(item RoutingTableItem) (*ottl.Statement[ottlresource.TransformContext], error) {
	var statement *ottl.Statement[ottlresource.TransformContext]
	if item.Statement != "" {
		var err error
		statement, err = r.parser.ParseStatement(item.Statement)
		if err != nil {
			return statement, err
		}
	}
	return statement, nil
}

func key(entry RoutingTableItem) string {
	return entry.Statement
}
