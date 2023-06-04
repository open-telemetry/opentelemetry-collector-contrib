// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReadComments(t *testing.T) {
	config, err := readConfigDocs("../../exporter/splunkhecexporter")
	require.NoError(t, err)
	var field FieldDoc
	for _, f := range config {
		if f.Name == "LogDataEnabled" {
			field = f
			break
		}
	}

	require.Equal(t, "LogDataEnabled can be used to disable sending logs by the exporter.", field.Comment)
	require.Equal(t, "log_data_enabled", field.Tag)
}
