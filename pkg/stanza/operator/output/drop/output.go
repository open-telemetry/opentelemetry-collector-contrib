// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package drop // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/output/drop"

import (
	"context"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

// Output is an operator that consumes and ignores incoming entries.
type Output struct {
	helper.OutputOperator
}

func (o *Output) ProcessBatch(_ context.Context, _ []*entry.Entry) error {
	return nil
}

// Process will drop the incoming entry.
func (o *Output) Process(_ context.Context, _ *entry.Entry) error {
	return nil
}
