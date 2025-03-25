// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awslogsencodingextension

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/extension/extensiontest"
)

func TestNew_CloudWatchLogsSubscriptionFilter(t *testing.T) {
	e, err := newExtension(&Config{Format: formatCloudWatchLogsSubscriptionFilter}, extensiontest.NewNopSettings(extensiontest.NopType))
	require.NoError(t, err)
	require.NotNil(t, e)

	_, err = e.UnmarshalLogs([]byte("invalid"))
	require.ErrorContains(t, err, `failed to unmarshal logs as "cloudwatch_logs_subscription_filter" format`)
}

func TestNew_Unimplemented(t *testing.T) {
	e, err := newExtension(&Config{Format: "invalid"}, extensiontest.NewNopSettings(extensiontest.NopType))
	require.Error(t, err)
	require.Nil(t, e)
	assert.EqualError(t, err, `unimplemented format "invalid"`)
}
