// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awscloudwatchmetricstreamsencodingextension

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/extension/extensiontest"
)

func TestNew_JSON(t *testing.T) {
	e, err := newExtension(&Config{Format: formatJSON}, extensiontest.NewNopSettings(extensiontest.NopType))
	require.NoError(t, err)
	require.NotNil(t, e)

	expectedErrStr := "failed to unmarshal metrics as 'json' format: " +
		"error unmarshaling datum at index 0: readObjectStart: " +
		"expect { or n, but found a, error found in #1 byte of ...|a|..., " +
		"bigger context ...|a|..."
	_, err = e.UnmarshalMetrics([]byte("a"))
	assert.EqualError(t, err, expectedErrStr)
}

func TestNew_OpenTelemetry10(t *testing.T) {
	e, err := newExtension(&Config{Format: formatOpenTelemetry10}, extensiontest.NewNopSettings(extensiontest.NopType))
	require.NoError(t, err)
	require.NotNil(t, e)

	expectedErrStr := "failed to unmarshal metrics as 'opentelemetry1.0' format: " +
		"index out of bounds: length prefix exceeds available bytes in record"
	_, err = e.UnmarshalMetrics([]byte("a"))
	assert.EqualError(t, err, expectedErrStr)
}

func TestNew_Unimplemented(t *testing.T) {
	e, err := newExtension(&Config{Format: "invalid"}, extensiontest.NewNopSettings(extensiontest.NopType))
	require.Error(t, err)
	require.Nil(t, e)
	assert.EqualError(t, err, `unimplemented format "invalid"`)
}
