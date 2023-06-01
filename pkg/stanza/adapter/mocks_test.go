// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package adapter

import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/transformer/noop"
)

// This file implements some useful testing components
func init() {
	operator.Register("unstartable_operator", func() operator.Builder { return NewUnstartableConfig() })
}

// UnstartableConfig is the configuration of an unstartable mock operator
type UnstartableConfig struct {
	helper.OutputConfig `mapstructure:",squash"`
}

// UnstartableOperator is an operator that will build but not start
// While this is not expected behavior, it is possible that build-time
// validation could be invalidated before Start() is called
type UnstartableOperator struct {
	helper.OutputOperator
}

// newUnstartableConfig creates new output config
func NewUnstartableConfig() operator.Config {
	return operator.NewConfig(&UnstartableConfig{
		OutputConfig: helper.NewOutputConfig("unstartable_operator", "unstartable_operator"),
	})
}

// Build will build an unstartable operator
func (c *UnstartableConfig) Build(logger *zap.SugaredLogger) (operator.Operator, error) {
	o, _ := c.OutputConfig.Build(logger)
	return &UnstartableOperator{OutputOperator: o}, nil
}

// Start will return an error
func (o *UnstartableOperator) Start(_ operator.Persister) error {
	return errors.New("something very unusual happened")
}

// Process will return nil
func (o *UnstartableOperator) Process(_ context.Context, _ *entry.Entry) error {
	return nil
}

const testType = "test"

type TestConfig struct {
	BaseConfig `mapstructure:",squash"`
	Input      operator.Config `mapstructure:",squash"`
}
type TestReceiverType struct{}

func (f TestReceiverType) Type() component.Type {
	return testType
}

func (f TestReceiverType) CreateDefaultConfig() component.Config {
	return &TestConfig{
		BaseConfig: BaseConfig{
			Operators: []operator.Config{},
		},
		Input: operator.NewConfig(noop.NewConfig()),
	}
}

func (f TestReceiverType) BaseConfig(cfg component.Config) BaseConfig {
	return cfg.(*TestConfig).BaseConfig
}

func (f TestReceiverType) InputConfig(cfg component.Config) operator.Config {
	return cfg.(*TestConfig).Input
}
