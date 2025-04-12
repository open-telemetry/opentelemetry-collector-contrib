// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package namedpipe // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/namedpipe"

import (
	"fmt"

	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/textutils"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
)

func init() {
	operator.Register(operatorType, func() operator.Builder { return NewConfig() })
}

// Build will build a namedpipe input operator.
func (c *Config) Build(set component.TelemetrySettings) (operator.Operator, error) {
	inputOperator, err := c.InputConfig.Build(set)
	if err != nil {
		return nil, err
	}

	enc, err := textutils.LookupEncoding(c.Encoding)
	if err != nil {
		return nil, fmt.Errorf("failed to lookup encoding %q: %w", c.Encoding, err)
	}

	splitFunc, err := c.SplitConfig.Func(enc, true, DefaultMaxLogSize)
	if err != nil {
		return nil, fmt.Errorf("failed to create split function: %w", err)
	}

	maxLogSize := c.MaxLogSize
	if maxLogSize == 0 {
		maxLogSize = DefaultMaxLogSize
	}

	return &Input{
		InputOperator: inputOperator,

		buffer:      make([]byte, maxLogSize),
		path:        c.Path,
		permissions: c.Permissions,
		splitFunc:   splitFunc,
		trimFunc:    c.TrimConfig.Func(),
	}, nil
}
