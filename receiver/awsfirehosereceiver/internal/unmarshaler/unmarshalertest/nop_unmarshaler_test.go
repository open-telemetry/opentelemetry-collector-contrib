// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
