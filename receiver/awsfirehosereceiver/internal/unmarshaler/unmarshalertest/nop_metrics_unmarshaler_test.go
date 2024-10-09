// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package unmarshalertest

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func TestNewNopMetrics(t *testing.T) {
	unmarshaler := NewNopMetrics()
	got, err := unmarshaler.Unmarshal(nil)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, typeStr, unmarshaler.Type())
}

func TestNewWithMetrics(t *testing.T) {
	metrics := pmetric.NewMetrics()
	metrics.ResourceMetrics().AppendEmpty()
	unmarshaler := NewWithMetrics(metrics)
	got, err := unmarshaler.Unmarshal(nil)
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, metrics, got)
	require.Equal(t, typeStr, unmarshaler.Type())
}

func TestNewErrMetrics(t *testing.T) {
	wantErr := fmt.Errorf("test error")
	unmarshaler := NewErrMetrics(wantErr)
	got, err := unmarshaler.Unmarshal(nil)
	require.Error(t, err)
	require.Equal(t, wantErr, err)
	require.NotNil(t, got)
	require.Equal(t, typeStr, unmarshaler.Type())
}
