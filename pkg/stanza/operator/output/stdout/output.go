// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package stdout // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/output/stdout"

import (
	"context"
	"encoding/json"
	"sync"

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
