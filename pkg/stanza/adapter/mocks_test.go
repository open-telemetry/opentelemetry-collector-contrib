// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package adapter

import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/transformer/noop"
)

var unstartableType = component.MustNewType("unstartable_operator")

// This file implements some useful testing components
func init() {
	operator.RegisterFactory(operator.NewFactory(unstartableType, newUnstartableConfig, createUnstartableOperator))
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

func newUnstartableConfig(operatorID string) component.Config {
	return &UnstartableConfig{
		OutputConfig: helper.NewOutputConfig(operatorID, "unstartable_operator"),
	}
}

func createUnstartableOperator(set component.TelemetrySettings, cfg component.Config) (operator.Operator, error) {
	op, err := helper.NewOutputOperator(set, cfg.(*UnstartableConfig).OutputConfig)
	if err != nil {
		return nil, err
	}
	return &UnstartableOperator{OutputOperator: op}, nil
}

// Start will return an error
func (o *UnstartableOperator) Start(_ operator.Persister) error {
	return errors.New("something very unusual happened")
}

// Process will return nil
func (o *UnstartableOperator) Process(_ context.Context, _ *entry.Entry) error {
	return nil
}

const testTypeStr = "test"

var testType = component.MustNewType(testTypeStr)

type TestConfig struct {
	BaseConfig `mapstructure:",squash"`
	Input      operator.Identifiable `mapstructure:",squash"`
}
type TestReceiverType struct{}

func (f TestReceiverType) Type() component.Type {
	return testType
}

func (f TestReceiverType) CreateDefaultConfig() component.Config {
	return &TestConfig{
		BaseConfig: BaseConfig{
			Operators: []operator.Identifiable{},
		},
		Input: noop.NewFactory().NewDefaultConfig("").(operator.Identifiable),
	}
}

func (f TestReceiverType) BaseConfig(cfg component.Config) BaseConfig {
	return cfg.(*TestConfig).BaseConfig
}

func (f TestReceiverType) InputConfig(cfg component.Config) operator.Identifiable {
	return cfg.(*TestConfig).Input
}
