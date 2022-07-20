// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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

	fileConsumer *fileconsumer.Input

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
