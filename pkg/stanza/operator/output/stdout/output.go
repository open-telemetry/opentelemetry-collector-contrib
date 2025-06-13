// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package stdout // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/output/stdout"

import (
	"context"
	"encoding/json"
	"sync"

	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

// Output is an operator that logs entries using stdout.
type Output struct {
	helper.OutputOperator
	encoder *json.Encoder
	mux     sync.Mutex
}

func (o *Output) ProcessBatch(ctx context.Context, entries []*entry.Entry) error {
	var errs error
	for i := range entries {
		errs = multierr.Append(errs, o.Process(ctx, entries[i]))
	}
	return errs
}

// Process will log entries received.
func (o *Output) Process(_ context.Context, entry *entry.Entry) error {
	o.mux.Lock()
	err := o.encoder.Encode(entry)
	if err != nil {
		o.mux.Unlock()
		o.Logger().Error("Failed to process entry", zap.Error(err))
		return err
	}
	o.mux.Unlock()
	return nil
}
