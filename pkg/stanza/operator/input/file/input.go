// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package file // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/file"

import (
	"context"
	"fmt"

	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

type toBodyFunc func([]byte) any

// Input is an operator that monitors files for entries
type Input struct {
	helper.InputOperator

	fileConsumer *fileconsumer.Manager

	toBody toBodyFunc
}

// Start will start the file monitoring process
func (i *Input) Start(persister operator.Persister) error {
	return i.fileConsumer.Start(persister)
}

// Stop will stop the file monitoring process
func (i *Input) Stop() error {
	return i.fileConsumer.Stop()
}

func (i *Input) emit(ctx context.Context, token []byte, attrs map[string]any) error {
	if len(token) == 0 {
		return nil
	}

	ent, err := i.NewEntry(i.toBody(token))
	if err != nil {
		return fmt.Errorf("create entry: %w", err)
	}

	for k, v := range attrs {
		if err := ent.Set(entry.NewAttributeField(k), v); err != nil {
			i.Logger().Error("set attribute", zap.Error(err))
		}
	}
	return i.Write(ctx, ent)
}
