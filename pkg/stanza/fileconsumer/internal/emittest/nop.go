// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package emittest // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/emittest"

import (
	"context"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/emit"
)

func Nop(_ context.Context, _ []emit.Token) error {
	return nil
}
