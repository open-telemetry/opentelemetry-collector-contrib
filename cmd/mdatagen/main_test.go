// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/receiver/receivertest"

	md "github.com/open-telemetry/opentelemetry-collector-contrib/cmd/mdatagen/internal/metadata"
)

func Test_runContents(t *testing.T) {
	tests := []struct {
		name    string
		yml     string
		wantErr bool
	}{
		{
			name: "valid metadata",
			yml: `
name: metricreceiver
metrics:
  metric:
    enabled: true
    description: Description.
    unit: s
    gauge:
      value_type: double`,
		},
		{
			name:    "invalid yaml",
			yml:     "invalid",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpdir := t.TempDir()

			metadataFile := filepath.Join(tmpdir, "metadata.yaml")
			require.NoError(t, os.WriteFile(metadataFile, []byte(tt.yml), 0600))

			err := run(metadataFile)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			require.FileExists(t, filepath.Join(tmpdir, "internal/metadata/generated_metrics.go"))
			require.FileExists(t, filepath.Join(tmpdir, "documentation.md"))
		})
	}
}

func Test_run(t *testing.T) {
	type args struct {
		ymlPath string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name:    "no argument",
			args:    args{""},
			wantErr: true,
		},
		{
			name:    "no such file",
			args:    args{"/no/such/file"},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := run(tt.args.ymlPath); (err != nil) != tt.wantErr {
				t.Errorf("run() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestGenerated verifies that the internal/metadata API is generated correctly.
func TestGenerated(t *testing.T) {
	mb := md.NewMetricsBuilder(md.DefaultMetricsBuilderConfig(), receivertest.NewNopCreateSettings())
	m := mb.Emit()
	require.Equal(t, 0, m.ResourceMetrics().Len())
}
