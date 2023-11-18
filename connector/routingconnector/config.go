// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package routingconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/routingconnector"

import (
	"errors"

	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

var (
	errEmptyRoute         = errors.New("invalid route: no statement provided")
	errNoPipelines        = errors.New("invalid route: no pipelines defined")
	errUnexpectedConsumer = errors.New("expected consumer to be a connector router")
	errNoTableItems       = errors.New("invalid routing table: the routing table is empty")
)

// Config defines configuration for the Routing processor.
type Config struct {
	// DefaultPipelines contains the list of pipelines to use when a more specific record can't be
	// found in the routing table.
	// Optional.
	DefaultPipelines []component.ID `mapstructure:"default_pipelines"`

	// ErrorMode determines how the processor reacts to errors that occur while processing an OTTL
	// condition.
	// Valid values are `ignore` and `propagate`.
	// `ignore` means the processor ignores errors returned by conditions and continues on to the
	// next condition. This is the recommended mode. If `ignored` is used and a statement's
	// condition has an error then the payload will be routed to the default exporter. `propagate`
	// means the processor returns the error up the pipeline.  This will result in the payload being
	// dropped from the collector.
	// The default value is `propagate`.
	ErrorMode ottl.ErrorMode `mapstructure:"error_mode"`

	// Table contains the routing table for this processor.
	// Required.
	Table []RoutingTableItem `mapstructure:"table"`

	// MatchOnce determines whether the connector matches multiple statements.
	// Optional.
	MatchOnce bool `mapstructure:"match_once"`
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
		if len(item.Statement) == 0 {
			return errEmptyRoute
		}

		if len(item.Pipelines) == 0 {
			return errNoPipelines
		}
	}

	return nil
}

// RoutingTableItem specifies how data should be routed to the different pipelines
type RoutingTableItem struct {
	// Statement is a OTTL statement used for making a routing decision.
	// Required when 'Value' isn't provided.
	Statement string `mapstructure:"statement"`

	// Pipelines contains the list of pipelines to use when the value from the FromAttribute field
	// matches this table item. When no pipelines are specified, the ones specified under
	// DefaultPipelines are used, if any.
	// The routing processor will fail upon the first failure from these pipelines.
	// Optional.
	Pipelines []component.ID `mapstructure:"pipelines"`
}
