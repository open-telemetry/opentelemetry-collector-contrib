// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package routingconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/routingconnector"

import (
	"errors"
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

var (
	errEmptyRoute      = errors.New("no statement provided")
	errNoPipelines     = errors.New("no pipelines defined for the route")
	errTooFewPipelines = errors.New(
		"routingconnector requires at least two pipelines to route between",
	)
	errNoTableItems = errors.New("the routing table is empty")
)

// Config defines configuration for the Routing processor.
type Config struct {
	// DefaultPipelines contains the list of pipelines to use when a more specific record can't be
	// found in the routing table.
	// Optional.
	DefaultPipelines []string `mapstructure:"default_pipelines"`

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
}

// Validate checks if the processor configuration is valid.
func (c *Config) Validate() error {
	// validate that there's at least one item in the table
	if len(c.Table) == 0 {
		return fmt.Errorf("invalid routing table: %w", errNoTableItems)
	}

	// validate that every route has a value for the routing attribute and has
	// at least one pipeline
	for _, item := range c.Table {
		if len(item.Statement) == 0 {
			return fmt.Errorf("invalid (empty) route : %w", errEmptyRoute)
		}

		if len(item.Pipelines) == 0 {
			return fmt.Errorf("invalid route %s: %w", item.Value, errNoPipelines)
		}
	}

	return nil
}

// RoutingTableItem specifies how data should be routed to the different pipelines
type RoutingTableItem struct {
	// Value represents a possible value for the field specified under FromAttribute.
	// Required when Statement isn't provided.
	Value string `mapstructure:"value"`

	// Statement is a OTTL statement used for making a routing decision.
	// Required when 'Value' isn't provided.
	Statement string `mapstructure:"statement"`

	// Pipelines contains the list of pipelines to use when the value from the FromAttribute field
	// matches this table item. When no pipelines are specified, the ones specified under
	// DefaultPipelines are used, if any.
	// The routing processor will fail upon the first failure from these pipelines.
	// Optional.
	Pipelines []string `mapstructure:"pipelines"`
}
