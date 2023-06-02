// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package routingprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/routingprocessor"

import (
	"errors"
	"fmt"
	"strings"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

var (
	errEmptyRoute             = errors.New("empty routing attribute provided")
	errNoExporters            = errors.New("no exporters defined for the route")
	errNoTableItems           = errors.New("the routing table is empty")
	errNoMissingFromAttribute = errors.New("the FromAttribute property is empty")
)

// Config defines configuration for the Routing processor.
type Config struct {

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

	// ErrorMode determines how the processor reacts to errors that occur while processing an OTTL condition.
	// Valid values are `ignore` and `propagate`.
	// `ignore` means the processor ignores errors returned by conditions and continues on to the next condition. This is the recommended mode.
	// If `ignored` is used and a statement's condition has an error then the payload will be routed to the default exporter.
	// `propagate` means the processor returns the error up the pipeline.  This will result in the payload being dropped from the collector.
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

	ottlRoutingOnly := true
	// validate that every route has a value for the routing attribute and has
	// at least one exporter
	for _, item := range c.Table {
		if len(item.Value) == 0 && len(item.Statement) == 0 {
			return fmt.Errorf("invalid (empty) route : %w", errEmptyRoute)
		}

		if len(item.Value) != 0 && len(item.Statement) != 0 {
			return fmt.Errorf("invalid route: both statement (%s) and value (%s) provided", item.Statement, item.Value)
		}

		if len(item.Exporters) == 0 {
			return fmt.Errorf("invalid route %s: %w", item.Value, errNoExporters)
		}

		if item.Value != "" {
			ottlRoutingOnly = false
		}
	}

	if ottlRoutingOnly {
		c.AttributeSource = ""
		c.FromAttribute = ""
	} else if len(c.FromAttribute) == 0 {
		// we also need a "FromAttribute" value
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
	// Value represents a possible value for the field specified under FromAttribute.
	// Required when Statement isn't provided.
	Value string `mapstructure:"value"`

	// Statement is a OTTL statement used for making a routing decision.
	// Required when 'Value' isn't provided.
	Statement string `mapstructure:"statement"`

	// Exporters contains the list of exporters to use when the value from the FromAttribute field matches this table item.
	// When no exporters are specified, the ones specified under DefaultExporters are used, if any.
	// The routing processor will fail upon the first failure from these exporters.
	// Optional.
	Exporters []string `mapstructure:"exporters"`
}

// rewriteRoutingEntriesToOTTL translates the attributes-based routing into OTTL
func rewriteRoutingEntriesToOTTL(cfg *Config) *Config {
	if cfg.AttributeSource != resourceAttributeSource {
		return cfg
	}
	table := make([]RoutingTableItem, 0, len(cfg.Table))
	for _, e := range cfg.Table {
		if e.Statement != "" {
			table = append(table, e)
			continue
		}
		var s strings.Builder
		if cfg.DropRoutingResourceAttribute {
			s.WriteString(
				fmt.Sprintf(
					"delete_key(resource.attributes, \"%s\")",
					cfg.FromAttribute,
				),
			)
		} else {
			s.WriteString("route()")
		}
		s.WriteString(
			fmt.Sprintf(
				" where resource.attributes[\"%s\"] == \"%s\"",
				cfg.FromAttribute,
				e.Value,
			),
		)
		table = append(table, RoutingTableItem{
			Statement: s.String(),
			Exporters: e.Exporters,
		})
	}
	return &Config{
		DefaultExporters: cfg.DefaultExporters,
		Table:            table,
	}
}
