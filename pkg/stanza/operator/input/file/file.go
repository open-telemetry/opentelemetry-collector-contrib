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

	"golang.org/x/text/encoding"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/internal/fileconsumer"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

// Input is an operator that monitors files for entries
type Input struct {
	helper.InputOperator

	fileConsumer *fileconsumer.Input

	FilePathField         entry.Field
	FileNameField         entry.Field
	FilePathResolvedField entry.Field
	FileNameResolvedField entry.Field
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
	// Skip the entry if it's empty
	if len(token) == 0 {
		return
	}
	var body interface{} = token
	var err error
	if f.fileConsumer.Encoding.Encoding != encoding.Nop {
		// TODO push decode logic down to reader, so it can reuse decoder
		body, err = f.fileConsumer.Encoding.Decode(token)
		if err != nil {
			f.Errorf("decode: %w", err)
			return
		}
	}

	ent, err := f.NewEntry(body)
	if err != nil {
		f.Errorf("create entry: %w", err)
		return
	}

	// TODO turn these into options
	if err := ent.Set(f.FilePathField, attrs.Path); err != nil {
		f.Errorf("set attribute: %w", err)
		return
	}
	if err := ent.Set(f.FileNameField, attrs.Name); err != nil {
		f.Errorf("set attribute: %w", err)
		return
	}
	if err := ent.Set(f.FilePathResolvedField, attrs.ResolvedPath); err != nil {
		f.Errorf("set attribute: %w", err)
		return
	}
	if err := ent.Set(f.FileNameResolvedField, attrs.ResolvedName); err != nil {
		f.Errorf("set attribute: %w", err)
		return
	}

	f.Write(ctx, ent)
}
