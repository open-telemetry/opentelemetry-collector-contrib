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

	_, err = e.UnmarshalMetrics([]byte{})
	assert.EqualError(t, err, errEmptyRecord.Error())
}

func TestNew_OpenTelemetry10(t *testing.T) {
	e, err := newExtension(&Config{Format: formatOpenTelemetry10}, extensiontest.NewNopSettings(extensiontest.NopType))
	require.NoError(t, err)
	require.NotNil(t, e)

	_, err = e.UnmarshalMetrics([]byte{})
	assert.EqualError(t, err, `UnmarshalMetrics unimplemented for format "opentelemetry1.0"`)
}

func TestNew_Unimplemented(t *testing.T) {
	e, err := newExtension(&Config{Format: "invalid"}, extensiontest.NewNopSettings(extensiontest.NopType))
	require.Error(t, err)
	require.Nil(t, e)
	assert.EqualError(t, err, `unimplemented format "invalid"`)
}
