// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package blobuploadprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/blobuploadprocessor"

import (
	"go.opentelemetry.io/collector/processor"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/blobuploadprocessor/internal/metadata"
)

// NewFactoryWithDeps instantiates the factory with dependency injection, allowing
// for a more testable interface that allows global functions/objects to be swapped out.
func newFactoryWithDeps(d deps) processor.Factory {
	return processor.NewFactory(
		metadata.Type,
		createDefaultConfig,
		processor.WithTraces(
			createTracesProcessor(d),
			metadata.TracesStability),
		processor.WithLogs(
			createLogsProcessor(d),
			metadata.LogsStability))
}

// NewFactory is used by the OTel collector to instantiate this component.
func NewFactory() processor.Factory {
	return newFactoryWithDeps(newDeps())
}
