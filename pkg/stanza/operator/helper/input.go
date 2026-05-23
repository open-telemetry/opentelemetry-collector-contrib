// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package helper // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/stanzaerrors"
)

// NewInputConfig creates a new input config with default values.
func NewInputConfig(operatorID, operatorType string) InputConfig {
	return InputConfig{
		AttributerConfig: NewAttributerConfig(),
		IdentifierConfig: NewIdentifierConfig(),
		WriterConfig:     NewWriterConfig(operatorID, operatorType),
	}
}

// InputConfig provides a basic implementation of an input operator config.
type InputConfig struct {
	AttributerConfig `mapstructure:",squash"`
	IdentifierConfig `mapstructure:",squash"`
	WriterConfig     `mapstructure:",squash"`
}

// Build will build a base producer.
func (c InputConfig) Build(set component.TelemetrySettings) (InputOperator, error) {
	writerOperator, err := c.WriterConfig.Build(set)
	if err != nil {
		return InputOperator{}, stanzaerrors.WithDetails(err, "operator_id", c.ID())
	}

	attributer, err := c.AttributerConfig.Build()
	if err != nil {
		return InputOperator{}, stanzaerrors.WithDetails(err, "operator_id", c.ID())
	}

	identifier, err := c.IdentifierConfig.Build()
	if err != nil {
		return InputOperator{}, stanzaerrors.WithDetails(err, "operator_id", c.ID())
	}

	inputOperator := InputOperator{
		Attributer:     attributer,
		Identifier:     identifier,
		WriterOperator: writerOperator,
	}

	return inputOperator, nil
}

// InputOperator provides a basic implementation of an input operator.
type InputOperator struct {
	Attributer
	Identifier
	WriterOperator
}

// NewEntry will create a new entry using the `attributes`, and `resource` configuration.
func (i *InputOperator) NewEntry(value any) (*entry.Entry, error) {
	entry := entry.New()
	entry.Body = value

	if err := i.Attribute(entry); err != nil {
		return nil, fmt.Errorf("add attributes to entry: %w", err)
	}

	if err := i.Identify(entry); err != nil {
		return nil, fmt.Errorf("add resource keys to entry: %w", err)
	}

	return entry, nil
}

// CanProcess will always return false for an input operator.
func (*InputOperator) CanProcess() bool {
	return false
}

// ProcessBatch will always return an error if called.
func (i *InputOperator) ProcessBatch(_ context.Context, _ []*entry.Entry) error {
	i.Logger().Error("Operator received a batch of entries, but can not process")
	return stanzaerrors.NewError(
		"Operator can not process logs.",
		"Ensure that operator is not configured to receive logs from other operators",
	)
}

// Process will always return an error if called.
func (i *InputOperator) Process(_ context.Context, _ *entry.Entry) error {
	i.Logger().Error("Operator received an entry, but can not process")
	return stanzaerrors.NewError(
		"Operator can not process logs.",
		"Ensure that operator is not configured to receive logs from other operators",
	)
}
