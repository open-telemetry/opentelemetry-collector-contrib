// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package attrs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/attrs"

import fca "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/attrs"

const (
	// Deprecated: [v0.93.0] Use pkg/stanza/fileconsumer/attrs.LogFileName instead.
	// Will be removed in v0.94.0.
	LogFileName = fca.LogFileName

	// Deprecated: [v0.92.0] Use pkg/stanza/fileconsumer/attrs.LogFilePath instead.
	// Will be removed in v0.94.0.
	LogFilePath = fca.LogFilePath

	// Deprecated: [v0.92.0] Use pkg/stanza/fileconsumer/attrs.LogFileNameResolved instead.
	// Will be removed in v0.94.0.
	LogFileNameResolved = fca.LogFileNameResolved

	// Deprecated: [v0.92.0] Use pkg/stanza/fileconsumer/attrs.LogFilePathResolved instead.
	// Will be removed in v0.94.0.
	LogFilePathResolved = fca.LogFilePathResolved
)
