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

func TestNew_VPCFlowLog(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Format = formatVPCFlowLog
	e, err := newExtension(cfg, extensiontest.NewNopSettings(extensiontest.NopType))
	require.NoError(t, err)
	require.NotNil(t, e)

	_, err = e.UnmarshalLogs([]byte("invalid"))
	require.ErrorContains(t, err, `failed to unmarshal logs as "vpc_flow_log" format`)
}

func TestNew_S3AccessLog(t *testing.T) {
	e, err := newExtension(&Config{Format: formatS3AccessLog}, extensiontest.NewNopSettings(extensiontest.NopType))
	require.NoError(t, err)
	require.NotNil(t, e)

	_, err = e.UnmarshalLogs([]byte("invalid"))
	require.ErrorContains(t, err, `failed to unmarshal logs as "s3_access_log" format`)
}

func TestNew_WAFLog(t *testing.T) {
	e, err := newExtension(&Config{Format: formatWAFLog}, extensiontest.NewNopSettings(extensiontest.NopType))
	require.NoError(t, err)
	require.NotNil(t, e)

	_, err = e.UnmarshalLogs([]byte("invalid"))
	require.ErrorContains(t, err, `failed to unmarshal logs as "waf_log" format`)
}

func TestNew_Unimplemented(t *testing.T) {
	e, err := newExtension(&Config{Format: "invalid"}, extensiontest.NewNopSettings(extensiontest.NopType))
	require.Error(t, err)
	require.Nil(t, e)
	assert.EqualError(t, err, `unimplemented format "invalid"`)
}
