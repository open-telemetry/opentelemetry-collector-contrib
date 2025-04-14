// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package windows // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/windows"

import (
	"errors"

	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
)

func init() {
	operator.Register(operatorType, func() operator.Builder { return NewConfig() })
}

// Build will build a windows event log operator.
func (c *Config) Build(set component.TelemetrySettings) (operator.Operator, error) {
	inputOperator, err := c.InputConfig.Build(set)
	if err != nil {
		return nil, err
	}

	if c.Channel == "" {
		return nil, errors.New("missing required `channel` field")
	}

	if c.MaxReads < 1 {
		return nil, errors.New("the `max_reads` field must be greater than zero")
	}

	if c.StartAt != "end" && c.StartAt != "beginning" {
		return nil, errors.New("the `start_at` field must be set to `beginning` or `end`")
	}

	if (c.Remote.Server != "" || c.Remote.Username != "" || c.Remote.Password != "") && // any not empty
		(c.Remote.Server == "" || c.Remote.Username == "" || c.Remote.Password == "") { // any empty
		return nil, errors.New("remote configuration must have non-empty `username` and `password`")
	}

	input := &Input{
		InputOperator:    inputOperator,
		buffer:           NewBuffer(),
		channel:          c.Channel,
		maxReads:         c.MaxReads,
		currentMaxReads:  c.MaxReads,
		startAt:          c.StartAt,
		pollInterval:     c.PollInterval,
		raw:              c.Raw,
		excludeProviders: excludeProvidersSet(c.ExcludeProviders),
		remote:           c.Remote,
	}
	input.startRemoteSession = input.defaultStartRemoteSession

	if c.SuppressRenderingInfo {
		input.processEvent = input.processEventWithoutRenderingInfo
	} else {
		input.processEvent = input.processEventWithRenderingInfo
	}

	return input, nil
}

func excludeProvidersSet(providers []string) map[string]struct{} {
	set := make(map[string]struct{}, len(providers))
	for _, provider := range providers {
		set[provider] = struct{}{}
	}
	return set
}
