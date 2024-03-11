// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package namedpipe // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/namedpipe"

import (
	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

var operatorType = component.MustNewType("namedpipe")

func init() {
	operator.RegisterFactory(NewFactory())
}

type factory struct{}

// NewFactory creates a new factory.
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
		InputConfig: helper.NewInputConfig(operatorID, operatorType.String()),
	}
}
