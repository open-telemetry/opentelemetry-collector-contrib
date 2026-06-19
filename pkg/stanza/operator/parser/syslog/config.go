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
	None    = "none"

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
		BaseConfig: BaseConfig{
			EnableLenientDay: true,
			CiscoIOS: &CiscoIOSConfig{
				MessageCounter:  true,
				SequenceNumber:  true,
				Hostname:        true,
				SecondFractions: true,
			},
		},
	}
}

// Config is the configuration of a syslog parser operator.
type Config struct {
	helper.ParserConfig `mapstructure:",squash"`
	BaseConfig          `mapstructure:",squash"`
}

// CiscoIOSConfig defines the configuration for Cisco IOS syslog extensions parser.
type CiscoIOSConfig struct {
	Enable          bool `mapstructure:"enable,omitempty"`
	MessageCounter  bool `mapstructure:"message_counter,omitempty"`
	SequenceNumber  bool `mapstructure:"sequence_number,omitempty"`
	Hostname        bool `mapstructure:"hostname,omitempty"`
	SecondFractions bool `mapstructure:"second_fractions,omitempty"`
}

// BaseConfig is the detailed configuration of a syslog syslog parser.
type BaseConfig struct {
	Protocol                     string          `mapstructure:"protocol,omitempty"`
	Location                     string          `mapstructure:"location,omitempty"`
	EnableOctetCounting          bool            `mapstructure:"enable_octet_counting,omitempty"`
	AllowSkipPriHeader           bool            `mapstructure:"allow_skip_pri_header,omitempty"`
	NonTransparentFramingTrailer *string         `mapstructure:"non_transparent_framing_trailer,omitempty"`
	MaxOctets                    int             `mapstructure:"max_octets,omitempty"`
	EnableCompliantMsg           bool            `mapstructure:"enable_compliant_msg,omitempty"`
	EnableRFC3339                bool            `mapstructure:"enable_rfc3339,omitempty"`
	EnableSecondFractions        bool            `mapstructure:"enable_second_fractions,omitempty"`
	EnableLenientDay             bool            `mapstructure:"enable_lenient_day,omitempty"`
	EnableEmbeddedNewlines       bool            `mapstructure:"enable_embedded_newlines,omitempty"`
	EnableCiscoHostname          bool            `mapstructure:"enable_cisco_hostname,omitempty"`
	CiscoIOS                     *CiscoIOSConfig `mapstructure:"cisco_ios,omitempty"`
	Year                         *int            `mapstructure:"year,omitempty"`
	Timezone                     string          `mapstructure:"timezone,omitempty"`
}

// Build will build a JSON parser operator.
func (c Config) Build(set component.TelemetrySettings) (operator.Operator, error) {
	proto := strings.ToLower(c.Protocol)

	// For RFC protocols, set up the default time parser if not provided.
	// For 'none' protocol, skip automatic time parser setup since timestamp may not exist.
	if c.TimeParser == nil && proto != None {
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

	switch {
	case proto == "":
		return nil, errors.New("missing field 'protocol'")
	case proto != RFC5424 && proto != RFC3164 && proto != None:
		return nil, fmt.Errorf("unsupported protocol version: %s", proto)
	case proto == None && c.NonTransparentFramingTrailer != nil:
		return nil, errors.New("non_transparent_framing is not compatible with protocol none")
	case proto != RFC5424 && proto != None && (c.NonTransparentFramingTrailer != nil || c.EnableOctetCounting):
		return nil, errors.New("octet_counting and non_transparent_framing are only compatible with protocol rfc5424")
	case proto == RFC5424 && (c.NonTransparentFramingTrailer != nil && c.EnableOctetCounting):
		return nil, errors.New("only one of octet_counting or non_transparent_framing can be enabled")
	case proto == RFC5424 && c.NonTransparentFramingTrailer != nil:
		if *c.NonTransparentFramingTrailer != NULTrailer && *c.NonTransparentFramingTrailer != LFTrailer {
			return nil, fmt.Errorf("invalid non_transparent_framing_trailer '%s'. Must be either 'LF' or 'NUL'", *c.NonTransparentFramingTrailer)
		}
	}

	if proto != RFC5424 && c.EnableCompliantMsg {
		return nil, errors.New("enable_compliant_msg is only compatible with protocol rfc5424")
	}

	if proto != RFC3164 && (c.EnableRFC3339 || c.EnableSecondFractions || c.EnableEmbeddedNewlines || c.EnableCiscoHostname ||
		(c.CiscoIOS != nil && (c.CiscoIOS.Enable || !c.CiscoIOS.MessageCounter || !c.CiscoIOS.SequenceNumber || !c.CiscoIOS.Hostname || !c.CiscoIOS.SecondFractions)) ||
		c.Year != nil || c.Timezone != "") {
		return nil, errors.New("rfc3164 options are only compatible with protocol rfc3164")
	}

	if c.Location == "" {
		c.Location = "UTC"
	}

	location, err := time.LoadLocation(c.Location)
	if err != nil {
		return nil, fmt.Errorf("failed to load location %s: %w", c.Location, err)
	}

	var timezone *time.Location
	if c.Timezone != "" {
		timezone, err = time.LoadLocation(c.Timezone)
		if err != nil {
			return nil, fmt.Errorf("failed to load timezone %s: %w", c.Timezone, err)
		}
	}

	return &Parser{
		ParserOperator:               parserOperator,
		protocol:                     proto,
		location:                     location,
		enableOctetCounting:          c.EnableOctetCounting,
		allowSkipPriHeader:           c.AllowSkipPriHeader,
		nonTransparentFramingTrailer: c.NonTransparentFramingTrailer,
		maxOctets:                    c.MaxOctets,
		enableCompliantMsg:           c.EnableCompliantMsg,
		enableRFC3339:                c.EnableRFC3339,
		enableSecondFractions:        c.EnableSecondFractions,
		enableLenientDay:             c.EnableLenientDay,
		enableEmbeddedNewlines:       c.EnableEmbeddedNewlines,
		enableCiscoHostname:          c.EnableCiscoHostname,
		ciscoIOS:                     c.CiscoIOS,
		year:                         c.Year,
		timezone:                     timezone,
	}, nil
}
