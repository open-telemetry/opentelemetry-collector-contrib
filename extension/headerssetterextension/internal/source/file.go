// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package source // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/headerssetterextension/internal/source"

import (
	"context"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/internal/credentialsfile"
)

var _ Source = (*FileSource)(nil)

type FileSource struct {
	Resolver credentialsfile.ValueResolver
}

func (fs *FileSource) Get(_ context.Context) (string, error) {
	return fs.Resolver.Value(), nil
}
