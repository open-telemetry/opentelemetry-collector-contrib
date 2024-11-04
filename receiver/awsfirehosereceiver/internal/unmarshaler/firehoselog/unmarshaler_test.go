// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package firehoselog

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestUnmarshal(t *testing.T) {
	body, err := os.ReadFile("testdata/firehose_log.json")
	require.NoError(t, err)

	unmarshaler := NewUnmarshaler(zap.NewNop())
	var records [][]byte
	records = append(records, body)
	got, err := unmarshaler.Unmarshal(records, nil, "arn:aws:firehose:us-east-1:123456789:deliverystream/testStream", 1646096348000)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, 1, got.ResourceLogs().Len())
	require.Equal(t, 1, got.ResourceLogs().At(0).ScopeLogs().Len())
}
