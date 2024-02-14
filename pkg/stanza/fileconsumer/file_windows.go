// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package fileconsumer // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer"

import (
	"context"
)

func (m *Manager) preConsume(ctx context.Context) {
}

// On windows, we close files immediately after reading because they cannot be moved while open.
func (m *Manager) postConsume() {
	// m.currentPollFiles -> m.previousPollFiles
	m.previousPollFiles = m.currentPollFiles
	m.closePreviousFiles()
}
