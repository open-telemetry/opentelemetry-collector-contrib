// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dynatrace

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/processor"
	"go.uber.org/zap"
)

const testPropertiesFile = `
dt.entity.host=my-host-from-properties
host.name=my-host-from-properties
dt.entity.host_group=my-host-group-from-properties
dt.foo=bar
invalid-entry
`

func TestDetectorNewDetector(t *testing.T) {
	d, err := NewDetector(processor.Settings{}, nil)

	require.NoError(t, err)
	require.NotNil(t, d)

	if runtime.GOOS == "windows" {
		t.Setenv("ProgramData", "C:\\ProgramData")
		require.Equal(t, "C:\\ProgramData\\dynatrace\\enrichment", d.(*Detector).enrichmentDirectory)
	} else {
		require.Equal(t, "/var/lib/dynatrace/enrichment", d.(*Detector).enrichmentDirectory)
	}
}

func TestDetector_DetectFromProperties(t *testing.T) {
	d, err := NewDetector(processor.Settings{
		TelemetrySettings: component.TelemetrySettings{
			Logger: zap.NewNop(),
		},
	}, nil)

	require.NoError(t, err)

	tempDir := t.TempDir()

	require.NoError(t, createTestFile(tempDir, dtHostMetadataProperties, testPropertiesFile))
	d.(*Detector).enrichmentDirectory = tempDir

	resource, _, err := d.Detect(context.Background())

	require.NoError(t, err)
	require.NotNil(t, resource)
	require.Equal(t, 2, resource.Attributes().Len())

	get, ok := resource.Attributes().Get("dt.entity.host")
	require.True(t, ok)
	require.Equal(t, "my-host-from-properties", get.Str())

	get, ok = resource.Attributes().Get("host.name")
	require.True(t, ok)
	require.Equal(t, "my-host-from-properties", get.Str())

	// verify that we do not take any additional properties
	_, ok = resource.Attributes().Get("dt.entity.host_group")
	require.False(t, ok)
}

func TestDetector_DetectNoFileAvailable(t *testing.T) {
	d, err := NewDetector(processor.Settings{
		TelemetrySettings: component.TelemetrySettings{
			Logger: zap.NewNop(),
		},
	}, nil)

	require.NoError(t, err)

	tempDir := t.TempDir()

	d.(*Detector).enrichmentDirectory = tempDir

	resource, _, err := d.Detect(context.Background())

	require.NoError(t, err)
	require.NotNil(t, resource)
	require.Equal(t, 0, resource.Attributes().Len())
}

func createTestFile(directory, name, content string) error {
	return os.WriteFile(filepath.Join(directory, name), []byte(content), 0o600)
}
