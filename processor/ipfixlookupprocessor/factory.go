// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ipfixlookupprocessor

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"
)

const (
	// this is the name used to refer to the processor in the config.yaml
	typeStr = "ipfix_lookup"
)

func NewFactory() processor.Factory {

	return processor.NewFactory(
		typeStr,
		createDefaultConfig,
		processor.WithTraces(createTracesToTracesProcessor, component.StabilityLevelAlpha))
}

func createDefaultConfig() component.Config {
	return &Config{
		QueryParameters: QueryParameters{
			BaseQuery: BaseQuery{
				FieldName:  "input.type",
				FieldValue: "netflow",
			},
			DeviceIdentifier: "fields.observer\\.ip.0",
			LookupFields: LookupFields{
				SourceIP:        "source.ip",
				SourcePort:      "source.port",
				DestinationIP:   "destination.ip",
				DestinationPort: "destination.port",
			},
		},
		SpanAttributeFields: []string{
			"@this",
			"fields.event\\.duration.0",
			"fields.observer\\.ip.0",
			"fields.source\\.ip.0",
			"fields.source\\.port.0",
			"fields.destination\\.ip.0",
			"fields.destination\\.port.0",
			"fields.netflow\\.ip_next_hop_ipv4_address",
		},

		Spans: Spans{
			SpanFields: SpanFields{
				SourceIPs: []string{
					"net.peer.ip",
					"src.ip",
				},
				SourcePorts: []string{
					"net.peer.port",
					"src.port",
				},
				DestinationIPandPort: []string{
					"http.host",
				},
				DestinationIPs: []string{
					"dst.ip",
				},
				DestinationPorts: []string{
					"dst.port",
				},
			},
		},
	}
}

func createTracesToTracesProcessor(_ context.Context, params processor.CreateSettings, cfg component.Config, nextConsumer consumer.Traces) (processor.Traces, error) {
	c := newIPFIXLookupProcessor(params.Logger, cfg)
	c.tracesConsumer = nextConsumer
	return c, nil
}
