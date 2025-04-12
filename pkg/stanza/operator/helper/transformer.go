// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package helper // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"

import (
	"context"
	"errors"
	"fmt"

	"github.com/expr-lang/expr/vm"
	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	stanza_errors "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/errors"
)

// NewTransformerConfig creates a new transformer config with default values
func NewTransformerConfig(operatorID, operatorType string) TransformerConfig {
	return TransformerConfig{
		WriterConfig: NewWriterConfig(operatorID, operatorType),
		OnError:      SendOnError,
	}
}

// TransformerConfig provides a basic implementation of a transformer config.
type TransformerConfig struct {
	WriterConfig `mapstructure:",squash"`
	OnError      string `mapstructure:"on_error"`
	IfExpr       string `mapstructure:"if"`
}

// Build will build a transformer operator.
func (c TransformerConfig) Build(set component.TelemetrySettings) (TransformerOperator, error) {
	writerOperator, err := c.WriterConfig.Build(set)
	if err != nil {
		return TransformerOperator{}, stanza_errors.WithDetails(err, "operator_id", c.ID())
	}

	switch c.OnError {
	case SendOnError, SendOnErrorQuiet, DropOnError, DropOnErrorQuiet:
	default:
		return TransformerOperator{}, stanza_errors.NewError(
			"operator config has an invalid `on_error` field.",
			"ensure that the `on_error` field is set to one of `send`, `send_quiet`, `drop`, `drop_quiet`.",
			"on_error", c.OnError,
		)
	}

	transformerOperator := TransformerOperator{
		WriterOperator: writerOperator,
		OnError:        c.OnError,
	}

	if c.IfExpr != "" {
		compiled, err := ExprCompileBool(c.IfExpr)
		if err != nil {
			return TransformerOperator{}, fmt.Errorf("failed to compile expression '%s': %w", c.IfExpr, err)
		}
		transformerOperator.IfExpr = compiled
	}

	return transformerOperator, nil
}

// TransformerOperator provides a basic implementation of a transformer operator.
type TransformerOperator struct {
	WriterOperator
	OnError string
	IfExpr  *vm.Program
}

// CanProcess will always return true for a transformer operator.
func (t *TransformerOperator) CanProcess() bool {
	return true
}

func (t *TransformerOperator) ProcessBatchWith(ctx context.Context, entries []*entry.Entry, process ProcessFunction) error {
	var errs []error
	for i := range entries {
		errs = append(errs, process(ctx, entries[i]))
	}
	return errors.Join(errs...)
}

// ProcessWith will process an entry with a transform function.
func (t *TransformerOperator) ProcessWith(ctx context.Context, entry *entry.Entry, transform TransformFunction) error {
	// Short circuit if the "if" condition does not match
	skip, err := t.Skip(ctx, entry)
	if err != nil {
		return t.HandleEntryError(ctx, entry, err)
	}
	if skip {
		return t.Write(ctx, entry)
	}

	if err := transform(entry); err != nil {
		return t.HandleEntryError(ctx, entry, err)
	}
	return t.Write(ctx, entry)
}

// HandleEntryError will handle an entry error using the on_error strategy.
func (t *TransformerOperator) HandleEntryError(ctx context.Context, entry *entry.Entry, err error) error {
	if entry == nil {
		return errors.New("got a nil entry, this should not happen and is potentially a bug")
	}

	if t.OnError == SendOnErrorQuiet || t.OnError == DropOnErrorQuiet {
		// No need to construct the zap attributes if logging not enabled at debug level.
		if t.Logger().Core().Enabled(zapcore.DebugLevel) {
			t.Logger().Debug("Failed to process entry", zapAttributes(entry, t.OnError, err)...)
		}
	} else {
		t.Logger().Error("Failed to process entry", zapAttributes(entry, t.OnError, err)...)
	}
	if t.OnError == SendOnError || t.OnError == SendOnErrorQuiet {
		if writeErr := t.Write(ctx, entry); writeErr != nil {
			err = fmt.Errorf("failed to send entry after error: %w", writeErr)
		}
	}

	return err
}

func (t *TransformerOperator) Skip(_ context.Context, entry *entry.Entry) (bool, error) {
	if t.IfExpr == nil {
		return false, nil
	}

	env := GetExprEnv(entry)
	defer PutExprEnv(env)

	matches, err := vm.Run(t.IfExpr, env)
	if err != nil {
		return false, fmt.Errorf("running if expr: %w", err)
	}

	return !matches.(bool), nil
}

func zapAttributes(entry *entry.Entry, action string, err error) []zap.Field {
	logFields := make([]zap.Field, 0, 3+len(entry.Attributes))
	logFields = append(logFields, zap.Error(err))
	logFields = append(logFields, zap.String("action", action))
	logFields = append(logFields, zap.Time("entry.timestamp", entry.Timestamp))
	for attrName, attrValue := range entry.Attributes {
		logFields = append(logFields, zap.Any(attrName, attrValue))
	}
	return logFields
}

// ProcessFunction is a function that processes an entry.
type ProcessFunction = func(context.Context, *entry.Entry) error

// TransformFunction is function that transforms an entry.
type TransformFunction = func(*entry.Entry) error

// SendOnError specifies an on_error mode for sending entries after an error.
const SendOnError = "send"

// SendOnErrorQuiet specifies an on_error mode for sending entries after an error but without logging on error level
const SendOnErrorQuiet = "send_quiet"

// DropOnError specifies an on_error mode for dropping entries after an error.
const DropOnError = "drop"

// DropOnErrorQuiet specifies an on_error mode for dropping entries after an error but without logging on error level
const DropOnErrorQuiet = "drop_quiet"
