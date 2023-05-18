// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Skip tests on Windows temporarily, see https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/11451
//go:build !windows
// +build !windows

package docsgen

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/configschema"
)

func TestTableTemplate(t *testing.T) {
	field := testDataField(t)
	tmpl, err := tableTemplate()
	require.NoError(t, err)
	bytes, err := renderTable(tmpl, field)
	require.NoError(t, err)
	require.NotNil(t, bytes)
}

func testDataField(t *testing.T) *configschema.Field {
	jsonBytes, err := os.ReadFile(filepath.Join("testdata", "otlp-receiver.json"))
	require.NoError(t, err)
	field := configschema.Field{}
	err = json.Unmarshal(jsonBytes, &field)
	require.NoError(t, err)
	return &field
}
