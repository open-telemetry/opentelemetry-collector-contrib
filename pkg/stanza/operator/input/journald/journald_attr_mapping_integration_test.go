// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration && linux

package journald

import (
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/semconvtest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/adapter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
)

func TestSemanticConventionCompliance(t *testing.T) {
	t.Setenv("TESTCONTAINERS_RYUK_DISABLED", "true")

	e := entry.New()
	mapJournalEntryAttributes(e, map[string]any{
		"MESSAGE":   "test message",
		"PRIORITY":  "6",
		"CODE_FILE": "/src/main.go",
		"CODE_FUNC": "main",
		"CODE_LINE": "42",
		"_HOSTNAME": "test-host",
		"_PID":      "1234",
		"_EXE":      "/usr/bin/test-app",
		"_CMDLINE":  "/usr/bin/test-app --config test.yaml",
	})

	semconvtest.TestLogs(t, adapter.ConvertEntries([]*entry.Entry{e}))
}
