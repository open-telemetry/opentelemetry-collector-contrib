// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package file // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/file"

import (
	"context"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

type toBodyFunc func([]byte) interface{}

// Input is an operator that monitors files for entries
type Input struct {
	helper.InputOperator

	fileConsumer *fileconsumer.Manager

	toBody         toBodyFunc
	preEmitOptions []preEmitOption
}

// Start will start the file monitoring process
func (f *Input) Start(persister operator.Persister) error {
	return f.fileConsumer.Start(persister)
}

// Stop will stop the file monitoring process
func (f *Input) Stop() error {
	return f.fileConsumer.Stop()
}

func (f *Input) emit(ctx context.Context, attrs *fileconsumer.FileAttributes, token []byte) {
	if len(token) == 0 {
		return
	}

	ent, err := f.NewEntry(f.toBody(token))
	if err != nil {
		f.Errorf("create entry: %w", err)
		return
	}

	for _, option := range f.preEmitOptions {
		if err := option(attrs, ent); err != nil {
			f.Errorf("preemit: %w", err)
		}
	}

	f.Write(ctx, ent)
}

type preEmitOption func(*fileconsumer.FileAttributes, *entry.Entry) error

func setHeaderMetadata(attrs *fileconsumer.FileAttributes, ent *entry.Entry) error {
	return ent.Set(entry.NewAttributeField(), attrs.HeaderAttributesCopy())
}

func setFileName(attrs *fileconsumer.FileAttributes, ent *entry.Entry) error {
	return ent.Set(entry.NewAttributeField("log.file.name"), attrs.Name)
}

func setFilePath(attrs *fileconsumer.FileAttributes, ent *entry.Entry) error {
	return ent.Set(entry.NewAttributeField("log.file.path"), attrs.Path)
}

func setFileNameResolved(attrs *fileconsumer.FileAttributes, ent *entry.Entry) error {
	return ent.Set(entry.NewAttributeField("log.file.name_resolved"), attrs.NameResolved)
}

func setFilePathResolved(attrs *fileconsumer.FileAttributes, ent *entry.Entry) error {
	return ent.Set(entry.NewAttributeField("log.file.path_resolved"), attrs.PathResolved)
}
