// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package file // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/file"

import (
	"context"
	"errors"
	"fmt"

	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/attrs"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

type toBodyFunc func([]byte) any

// Input is an operator that monitors files for entries
type Input struct {
	helper.InputOperator

	fileConsumer *fileconsumer.Manager

	toBody                  toBodyFunc
	includeFileRecordNumber bool
}

// Start will start the file monitoring process
func (i *Input) Start(persister operator.Persister) error {
	return i.fileConsumer.Start(persister)
}

// Stop will stop the file monitoring process
func (i *Input) Stop() error {
	return i.fileConsumer.Stop()
}

func (i *Input) emitBatch(ctx context.Context, tokens [][]byte, attributes map[string]any, lastRecordNumber int64) error {
	entries, conversionError := i.convertTokens(tokens, attributes, lastRecordNumber)
	if conversionError != nil {
		conversionError = fmt.Errorf("convert tokens: %w", conversionError)
	}

	consumeError := i.WriteBatch(ctx, entries)
	if consumeError != nil {
		consumeError = fmt.Errorf("consume entries: %w", consumeError)
	}

	return errors.Join(conversionError, consumeError)
}

func (i *Input) convertTokens(tokens [][]byte, attributes map[string]any, lastRecordNumber int64) ([]*entry.Entry, error) {
	entries := make([]*entry.Entry, 0, len(tokens))
	var errs []error

	for tokenIndex, token := range tokens {
		if len(token) == 0 {
			continue
		}

		ent, err := i.NewEntry(i.toBody(token))
		if err != nil {
			errs = append(errs, fmt.Errorf("create entry: %w", err))
			continue
		}

		for k, v := range attributes {
			if err := ent.Set(entry.NewAttributeField(k), v); err != nil {
				i.Logger().Error("set attribute", zap.Error(err))
			}
		}

		if i.includeFileRecordNumber {
			if err := ent.Set(entry.NewAttributeField(attrs.LogFileRecordNumber), lastRecordNumber-int64(len(tokens))+int64(tokenIndex)+1); err != nil {
				i.Logger().Error("set record number attribute", zap.Error(err))
			}
		}

		entries = append(entries, ent)
	}
	return entries, errors.Join(errs...)
}
