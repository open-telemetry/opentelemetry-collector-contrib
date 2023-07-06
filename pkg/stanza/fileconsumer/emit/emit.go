// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package emit // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/emit"

import (
	"context"
)

type Callback func(ctx context.Context, token []byte, attrs map[string]any)
