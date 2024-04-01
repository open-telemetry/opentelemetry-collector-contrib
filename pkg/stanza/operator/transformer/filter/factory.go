// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filter // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/transformer/filter"

import (
	"fmt"
	"math/big"

	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

var operatorType = component.MustNewType("filter")

func init() {
	operator.RegisterFactory(NewFactory())
}

// NewFactory creates a new factory.
func NewFactory() operator.Factory {
	return operator.NewFactory(operatorType, newDefaultConfig, createOperator)
}

func newDefaultConfig(operatorID string) component.Config {
	return &Config{
		TransformerConfig: helper.NewTransformerConfig(operatorID, operatorType.String()),
		DropRatio:         1,
	}
}

func createOperator(set component.TelemetrySettings, cfg component.Config) (operator.Operator, error) {
	c := cfg.(*Config)
	transformer, err := helper.NewTransformer(set, c.TransformerConfig)
	if err != nil {
		return nil, err
	}

	compiledExpression, err := helper.ExprCompileBool(c.Expression)
	if err != nil {
		return nil, fmt.Errorf("failed to compile expression '%s': %w", c.Expression, err)
	}

	if c.DropRatio < 0.0 || c.DropRatio > 1.0 {
		return nil, fmt.Errorf("drop_ratio must be a number between 0 and 1")
	}

	return &Transformer{
		TransformerOperator: transformer,
		expression:          compiledExpression,
		dropCutoff:          big.NewInt(int64(c.DropRatio * 1000)),
	}, nil
}
