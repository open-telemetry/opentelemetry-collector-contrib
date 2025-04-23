// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package file // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/output/file"

import (
	"context"
	"encoding/json"
	"os"
	"sync"
	"text/template"

	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

// Output is an operator that writes logs to a file.
type Output struct {
	helper.OutputOperator

	path    string
	tmpl    *template.Template
	encoder *json.Encoder
	file    *os.File
	mux     sync.Mutex
}

// Start will open the output file.
func (o *Output) Start(_ operator.Persister) error {
	var err error
	o.file, err = os.OpenFile(o.path, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0o600)
	if err != nil {
		return err
	}

	o.encoder = json.NewEncoder(o.file)
	o.encoder.SetEscapeHTML(false)

	return nil
}

// Stop will close the output file.
func (o *Output) Stop() error {
	if o.file != nil {
		if err := o.file.Close(); err != nil {
			o.Logger().Error("close", zap.Error(err))
		}
	}
	return nil
}

func (o *Output) ProcessBatch(ctx context.Context, entries []*entry.Entry) error {
	var errs error
	for i := range entries {
		errs = multierr.Append(errs, o.Process(ctx, entries[i]))
	}
	return errs
}

// Process will write an entry to the output file.
func (o *Output) Process(_ context.Context, entry *entry.Entry) error {
	o.mux.Lock()
	defer o.mux.Unlock()

	if o.tmpl != nil {
		err := o.tmpl.Execute(o.file, entry)
		if err != nil {
			return err
		}
	} else {
		err := o.encoder.Encode(entry)
		if err != nil {
			return err
		}
	}

	return nil
}
