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

const (
	operatorType = "syslog_parser"

	RFC3164 = "rfc3164"
	RFC5424 = "rfc5424"

	NULTrailer = "NUL"
	LFTrailer  = "LF"
)

func init() {
	operator.Register(operatorType, func() operator.Builder { return NewConfig() })
}

// NewConfig creates a new syslog parser config with default values
func NewConfig() *Config {
	return NewConfigWithID(operatorType)
}

// NewConfigWithID creates a new syslog parser config with default values
func NewConfigWithID(operatorID string) *Config {
	return &Config{
		ParserConfig: helper.NewParserConfig(operatorID, operatorType),
	}
}

// Config is the configuration of a syslog parser operator.
type Config struct {
	helper.ParserConfig `mapstructure:",squash"`
	BaseConfig          `mapstructure:",squash"`
}

// BaseConfig is the detailed configuration of a syslog parser.
type BaseConfig struct {
	Protocol                     string  `mapstructure:"protocol,omitempty"`
	Location                     string  `mapstructure:"location,omitempty"`
	EnableOctetCounting          bool    `mapstructure:"enable_octet_counting,omitempty"`
	AllowSkipPriHeader           bool    `mapstructure:"allow_skip_pri_header,omitempty"`
	NonTransparentFramingTrailer *string `mapstructure:"non_transparent_framing_trailer,omitempty"`
	MaxOctets                    int     `mapstructure:"max_octets,omitempty"`
}

// Build will build a JSON parser operator.
func (c Config) Build(set component.TelemetrySettings) (operator.Operator, error) {
	if c.TimeParser == nil {
		parseFromField := entry.NewAttributeField("timestamp")
		c.TimeParser = &helper.TimeParser{
			ParseFrom:  &parseFromField,
			LayoutType: helper.NativeKey,
		}
	}

	parserOperator, err := c.ParserConfig.Build(set)
	if err != nil {
		return nil, err
	}

	proto := strings.ToLower(c.Protocol)

	switch {
	case proto == "":
		return nil, errors.New("missing field 'protocol'")
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
		maxOctets:                    c.MaxOctets,
	}, nil
}
