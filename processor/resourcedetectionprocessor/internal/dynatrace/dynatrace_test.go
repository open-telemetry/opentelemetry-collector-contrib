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
	"go.opentelemetry.io/collector/processor"
)

const testPropertiesFile = `
dt.entity.host=my-host-from-properties
host.name=my-host-from-properties
dt.entity.host_group=my-host-group-from-properties
dt.foo=bar
`

const testJSONFile = `{
  "dt.entity.host": "my-host-from-json",
  "host.name": "my-host-from-json",
  "dt.entity.host_group": "my-host-group-from-json",
  "dt.bar": "foo"
}`

const testJSONFileWithObjects = `{
  "dt.entity.host": {"name":"my-host-from-json"},
  "host.name": "my-host-from-json"
}`

const testJSONFileInvalid = `invalid`

const testPropertiesFileWithMalformedEntries = `
dt.entity.host=my-host-from-properties
dt.entity.host_group
host.name=bar=

test = attr
`

func TestDetectorNewDetector(t *testing.T) {
	d, err := NewDetector(processor.Settings{}, nil)

	require.NoError(t, err)
	require.NotNil(t, d)

	if runtime.GOOS == "windows" {
		t.Setenv("ProgramData", "C:\\ProgramData")
		require.Equal(t, "C:\\ProgramData//dynatrace/enrichment", d.(*Detector).enrichmentDirectory)
	} else {
		require.Equal(t, "/var/lib/dynatrace/enrichment", d.(*Detector).enrichmentDirectory)
	}
}

func TestDetector_DetectFromProperties(t *testing.T) {
	d, err := NewDetector(processor.Settings{}, nil)

	require.Nil(t, err)

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

	get, ok = resource.Attributes().Get("dt.entity.host_group")
	require.False(t, ok)
}

func TestDetector_DetectFromJSON(t *testing.T) {
	d, err := NewDetector(processor.Settings{}, nil)

	require.Nil(t, err)

	tempDir := t.TempDir()

	require.NoError(t, createTestFile(tempDir, dtHostMetadataJSON, testJSONFile))
	d.(*Detector).enrichmentDirectory = tempDir

	resource, _, err := d.Detect(context.Background())

	require.NoError(t, err)
	require.NotNil(t, resource)
	require.Equal(t, 2, resource.Attributes().Len())

	get, ok := resource.Attributes().Get("dt.entity.host")
	require.True(t, ok)
	require.Equal(t, "my-host-from-json", get.Str())

	get, ok = resource.Attributes().Get("host.name")
	require.True(t, ok)
	require.Equal(t, "my-host-from-json", get.Str())

	get, ok = resource.Attributes().Get("dt.entity.host_group")
	require.False(t, ok)
}

func TestDetector_DetectFromBothPropertiesTakesPrecedence(t *testing.T) {
	d, err := NewDetector(processor.Settings{}, nil)

	require.Nil(t, err)

	tempDir := t.TempDir()

	require.NoError(t, createTestFile(tempDir, dtHostMetadataJSON, testJSONFile))
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

	_, ok = resource.Attributes().Get("dt.entity.host_group")
	require.False(t, ok)

	_, ok = resource.Attributes().Get("dt.foo")
	require.False(t, ok)

	_, ok = resource.Attributes().Get("dt.bar")
	require.False(t, ok)
}

func TestDetector_DetectNoFilesAvailable(t *testing.T) {
	d, err := NewDetector(processor.Settings{}, nil)

	require.Nil(t, err)

	tempDir := t.TempDir()

	d.(*Detector).enrichmentDirectory = tempDir

	resource, _, err := d.Detect(context.Background())

	require.NoError(t, err)
	require.NotNil(t, resource)
	require.Equal(t, 0, resource.Attributes().Len())
}

func TestDetector_DetectFromPropertiesWithMalformedEntries(t *testing.T) {
	d, err := NewDetector(processor.Settings{}, nil)

	require.Nil(t, err)

	tempDir := t.TempDir()

	require.NoError(t, createTestFile(tempDir, dtHostMetadataProperties, testPropertiesFileWithMalformedEntries))
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
	require.Equal(t, "bar=", get.Str())

	_, ok = resource.Attributes().Get("test")
	require.False(t, ok)
}

func TestDetector_DetectFromJSONWithObjects(t *testing.T) {
	d, err := NewDetector(processor.Settings{}, nil)

	require.Nil(t, err)

	tempDir := t.TempDir()

	require.NoError(t, createTestFile(tempDir, dtHostMetadataJSON, testJSONFileWithObjects))
	d.(*Detector).enrichmentDirectory = tempDir

	resource, _, err := d.Detect(context.Background())

	require.NoError(t, err)
	require.NotNil(t, resource)

	// expect only 1 attribute, as only string values are supported
	require.Equal(t, 1, resource.Attributes().Len())

	get, ok := resource.Attributes().Get("host.name")
	require.True(t, ok)
	require.Equal(t, "my-host-from-json", get.Str())

	_, ok = resource.Attributes().Get("dt.entity.host")
	require.False(t, ok)
}

func TestDetector_DetectFromJSONInvalid(t *testing.T) {
	d, err := NewDetector(processor.Settings{}, nil)

	require.Nil(t, err)

	tempDir := t.TempDir()

	require.NoError(t, createTestFile(tempDir, dtHostMetadataJSON, testJSONFileInvalid))
	d.(*Detector).enrichmentDirectory = tempDir

	_, _, err = d.Detect(context.Background())

	require.Error(t, err)
}

func createTestFile(directory, name, content string) error {
	return os.WriteFile(filepath.Join(directory, name), []byte(content), os.ModePerm)
}
