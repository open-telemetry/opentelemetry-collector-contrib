// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package syslog // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/parser/syslog"

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

var operatorType = component.MustNewType("syslog_parser")

func init() {
	operator.RegisterFactory(NewFactory())
}

type factory struct{}

// NewFactory creates a syslog parser factory.
func NewFactory() operator.Factory {
	return &factory{}
}

// Type gets the type of the operator.
func (f *factory) Type() component.Type {
	return operatorType
}

// NewDefaultConfig creates a new default configuration.
func (f *factory) NewDefaultConfig(operatorID string) component.Config {
	return &Config{
		ParserConfig: helper.NewParserConfig(operatorID, operatorType.String()),
	}
}

// CreateOperator creates a new syslog parser.
func (f *factory) CreateOperator(cfg component.Config, set component.TelemetrySettings) (operator.Operator, error) {
	c := cfg.(*Config)
	if c.ParserConfig.TimeParser == nil {
		parseFromField := entry.NewAttributeField("timestamp")
		c.ParserConfig.TimeParser = &helper.TimeParser{
			ParseFrom:  &parseFromField,
			LayoutType: helper.NativeKey,
		}
	}

	parserOperator, err := helper.NewParser(c.ParserConfig, set)
	if err != nil {
		return nil, err
	}

	proto := strings.ToLower(c.Protocol)

	switch {
	case proto == "":
		return nil, fmt.Errorf("missing field 'protocol'")
	case proto != RFC5424 && (c.NonTransparentFramingTrailer != nil || c.EnableOctetCounting):
		return nil, errors.New("octet_counting and non_transparent_framing are only compatible with protocol rfc5424")
	case proto == RFC5424 && (c.NonTransparentFramingTrailer != nil && c.EnableOctetCounting):
		return nil, errors.New("only one of octet_counting or non_transparent_framing can be enabled")
	case proto == RFC5424 && c.NonTransparentFramingTrailer != nil:
		if *c.NonTransparentFramingTrailer != NULTrailer && *c.NonTransparentFramingTrailer != LFTrailer {
			return nil, fmt.Errorf("invalid non_transparent_framing_trailer '%s'. Must be either 'LF' or 'NUL'", *c.NonTransparentFramingTrailer)
		}
	case proto != RFC5424 && proto != RFC3164:
		return nil, fmt.Errorf("unsupported protocol version: %s", proto)
	}

	if c.Location == "" {
		c.Location = "UTC"
	}

	location, err := time.LoadLocation(c.Location)
	if err != nil {
		return nil, fmt.Errorf("failed to load location %s: %w", c.Location, err)
	}

	return &Parser{
		ParserOperator:               parserOperator,
		protocol:                     proto,
		location:                     location,
		enableOctetCounting:          c.EnableOctetCounting,
		allowSkipPriHeader:           c.AllowSkipPriHeader,
		nonTransparentFramingTrailer: c.NonTransparentFramingTrailer,
	}, nil
}
