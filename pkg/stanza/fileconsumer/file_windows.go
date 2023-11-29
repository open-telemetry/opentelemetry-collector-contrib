// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows
// +build windows

package fileconsumer // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer"

import (
	"context"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/reader"
)

func (m *Manager) preConsume(ctx context.Context, newReaders []*reader.Reader) {
	return
}

// On windows, we close files immediately after reading becauase they cannot be moved while open.
func (m *Manager) postConsume(readers []*reader.Reader) {
	m.previousPollFiles = readers
	m.closePreviousFiles()
}
