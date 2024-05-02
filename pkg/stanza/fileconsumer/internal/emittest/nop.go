// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package emittest // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/emittest"

import (
	"context"
)

func Nop(_ context.Context, _ []byte, _ map[string]any) error {
	return nil
}
