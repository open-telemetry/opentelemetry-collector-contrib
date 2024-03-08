// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package journald // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/journald"

import (
	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

var operatorType = component.MustNewType("journald_input")

func init() {
	operator.RegisterFactory(NewFactory())
}

type factory struct{}

// NewFactory creates a factory.
func NewFactory() operator.Factory {
	return &factory{}
}

// Type gets the type of the operator.
func (f *factory) Type() component.Type {
	return operatorType
}

// NewDefaultConfig creates a default configuration.
func (f *factory) NewDefaultConfig(operatorID string) component.Config {
	return &Config{
		InputConfig: helper.NewInputConfig(operatorID, operatorType.String()),
		StartAt:     "end",
		Priority:    "info",
	}
}
