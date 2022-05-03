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

	"go.opentelemetry.io/collector/config"
)

// Config defines configuration for the Routing processor.
type Config struct {
	config.ProcessorSettings `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct

	// DefaultExporters contains the list of exporters to use when a more specific record can't be found in the routing table.
	// Optional.
	DefaultExporters []string `mapstructure:"default_exporters"`

	// AttributeSource defines where the attribute defined in `from_attribute` is searched for.
	// The allowed values are:
	// - "context" - the attribute must exist in the incoming context
	// - "resource" - the attribute must exist in resource attributes
	// The default value is "context".
	// Optional.
	AttributeSource AttributeSource `mapstructure:"attribute_source"`

	// FromAttribute contains the attribute name to look up the route value. This attribute should be part of the context propagated
	// down from the previous receivers and/or processors. If all the receivers and processors are propagating the entire context correctly,
	// this could be the HTTP/gRPC header from the original request/RPC. Typically, aggregation processors (batch, groupbytrace)
	// will create a new context, so, those should be avoided when using this processor.Although the HTTP spec allows headers to be repeated,
	// this processor will only use the first value.
	// Required.
	FromAttribute string `mapstructure:"from_attribute"`

	// DropRoutingResourceAttribute controls whether to remove the resource attribute used for routing.
	// This is only relevant if AttributeSource is set to resource.
	// Optional.
	DropRoutingResourceAttribute bool `mapstructure:"drop_resource_routing_attribute"`

	// Table contains the routing table for this processor.
	// Required.
	Table []RoutingTableItem `mapstructure:"table"`
}

// Validate checks if the processor configuration is valid.
func (c *Config) Validate() error {
	// validate that every route has a value for the routing attribute and has
	// at least one exporter
	for _, item := range c.Table {
		if len(item.Value) == 0 {
			return fmt.Errorf("invalid (empty) route : %w", errEmptyRoute)
		}

		if len(item.Exporters) == 0 {
			return fmt.Errorf("invalid route %s: %w", item.Value, errNoExporters)
		}
	}

	// validate that there's at least one item in the table
	if len(c.Table) == 0 {
		return fmt.Errorf("invalid routing table: %w", errNoTableItems)
	}

	// we also need a "FromAttribute" value
	if len(c.FromAttribute) == 0 {
		return fmt.Errorf(
			"invalid attribute to read the route's value from: %w",
			errNoMissingFromAttribute,
		)
	}

	if c.AttributeSource != resourceAttributeSource && c.DropRoutingResourceAttribute {
		return errors.New("using a different attribute source than 'attribute' and drop_resource_routing_attribute is set to true")
	}

	return nil
}

type AttributeSource string

const (
	contextAttributeSource  = AttributeSource("context")
	resourceAttributeSource = AttributeSource("resource")

	defaultAttributeSource = contextAttributeSource
)

// RoutingTableItem specifies how data should be routed to the different exporters
type RoutingTableItem struct {
	// Value represents a possible value for the field specified under FromAttribute. Required.
	Value string `mapstructure:"value"`

	// Exporters contains the list of exporters to use when the value from the FromAttribute field matches this table item.
	// When no exporters are specified, the ones specified under DefaultExporters are used, if any.
	// The routing processor will fail upon the first failure from these exporters.
	// Optional.
	Exporters []string `mapstructure:"exporters"`
}
