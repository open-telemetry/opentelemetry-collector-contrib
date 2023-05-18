// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package source // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/headerssetterextension/internal/source"

import "context"

type Source interface {
	Get(context.Context) (string, error)
}
