// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package routingconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/routingconnector"

import (
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/pipeline"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

var (
	errNoConditionOrStatement = errors.New("invalid route: no condition or statement provided")
	errConditionAndStatement  = errors.New("invalid route: both condition and statement provided")
	errNoPipelines            = errors.New("invalid route: no pipelines defined")
	errUnexpectedConsumer     = errors.New("expected consumer to be a connector router")
	errNoTableItems           = errors.New("invalid routing table: the routing table is empty")
)

// Config defines configuration for the Routing processor.
type Config struct {
	// ErrorMode determines how the processor reacts to errors that occur while processing an OTTL
	// condition.
	// Valid values are `ignore` and `propagate`.
	// `ignore` means the processor ignores errors returned by conditions and continues on to the
	// next condition. This is the recommended mode. If `ignore` is used and a statement's
	// condition has an error then the payload will be routed to the default exporter. `propagate`
	// means the processor returns the error up the pipeline.  This will result in the payload being
	// dropped from the collector.
	// The default value is `propagate`.
	ErrorMode ottl.ErrorMode `mapstructure:"error_mode"`
	// DefaultPipelines contains the list of pipelines to use when a more specific record can't be
	// found in the routing table.
	// Optional.
	DefaultPipelines []pipeline.ID `mapstructure:"default_pipelines"`
	// Table contains the routing table for this processor.
	// Required.
	Table []RoutingTableItem `mapstructure:"table"`
	// prevent unkeyed literal initialization
	_ struct{}
}

// Validate checks if the processor configuration is valid.
func (c *Config) Validate() error {
	// validate that there's at least one item in the table
	if len(c.Table) == 0 {
		return errNoTableItems
	}

	// validate that every route has a value for the routing attribute and has
	// at least one pipeline
	for _, item := range c.Table {
		if item.Statement == "" && item.Condition == "" {
			return errNoConditionOrStatement
		}
		if item.Statement != "" && item.Condition != "" {
			return errConditionAndStatement
		}
		if len(item.Pipelines) == 0 {
			return errNoPipelines
		}

		switch item.Context {
		case "", "resource", "span", "metric", "datapoint", "log": // ok
		case "request":
			if item.Statement != "" || item.Condition == "" {
				return fmt.Errorf("%q context requires a 'condition'", item.Context)
			}
			if _, err := parseRequestCondition(item.Condition); err != nil {
				return err
			}
		default:
			return errors.New("invalid context: " + item.Context)
		}
	}
	return nil
}

// RoutingTableItem specifies how data should be routed to the different pipelines
type RoutingTableItem struct {
	// One of "request", "resource", "log" (other OTTL contexts will be added in the future)
	// Optional. Default "resource".
	Context string `mapstructure:"context"`

	// Statement is a OTTL statement used for making a routing decision.
	// One of 'Statement' or 'Condition' must be provided.
	Statement string `mapstructure:"statement"`

	// Condition is an OTTL condition used for making a routing decision.
	// One of 'Statement' or 'Condition' must be provided.
	Condition string `mapstructure:"condition"`

	// Pipelines contains the list of pipelines to use when the value from the FromAttribute field
	// matches this table item. When no pipelines are specified, the ones specified under
	// DefaultPipelines are used, if any.
	// The routing processor will fail upon the first failure from these pipelines.
	// Optional.
	Pipelines []pipeline.ID `mapstructure:"pipelines"`
	// prevent unkeyed literal initialization
	_ struct{}
}
