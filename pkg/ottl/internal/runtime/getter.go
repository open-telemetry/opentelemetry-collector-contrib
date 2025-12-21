// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package runtime // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/internal/runtime"

import (
	"context"
)

type Getter[K any, T any] interface {
	Get(ctx context.Context, tCtx K) (T, error)
}
