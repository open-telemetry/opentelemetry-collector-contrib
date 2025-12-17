// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReadConfig_FileInput(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "service.go")
	require.NoError(t, os.WriteFile(file, []byte("package test"), 0o600))

	cfg, err := readConfigForTest(t, file)
	require.NoError(t, err)

	require.Equal(t, file, cfg.FilePath)
	require.Equal(t, dir, cfg.DirPath)
	expectedSchema := filepath.Join(dir, DefaultSchemaFileName+".yaml")
	require.Equal(t, expectedSchema, cfg.SchemaPath)
}

func TestReadConfig_DirectoryInput(t *testing.T) {
	dir := t.TempDir()

	cfg, err := readConfigForTest(t, dir)
	require.NoError(t, err)

	expectedFile := filepath.Join(dir, DefaultConfigGoFileName)
	require.Equal(t, expectedFile, cfg.FilePath)
	require.Equal(t, dir, cfg.DirPath)
	expectedSchema := filepath.Join(dir, DefaultSchemaFileName+".yaml")
	require.Equal(t, expectedSchema, cfg.SchemaPath)
}

func TestReadConfig_Errors(t *testing.T) {
	t.Run("missing args", func(t *testing.T) {
		_, err := readConfigForTest(t)
		require.Error(t, err)
	})

	t.Run("missing path", func(t *testing.T) {
		missing := filepath.Join(t.TempDir(), "missing")
		_, err := readConfigForTest(t, missing)
		require.Error(t, err)
	})
}

func TestReadConfig_DefaultRootTypeDerivedFromPath(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "my_config.go")
	require.NoError(t, os.WriteFile(target, []byte("package test"), 0o600))

	cfg, err := readConfigForTest(t, target)
	require.NoError(t, err)

	require.Equal(t, "MyConfig", cfg.RootTypeName)
}

func TestReadConfig_RespectsRootTypeFlag(t *testing.T) {
	dir := t.TempDir()

	cfg, err := readConfigForTest(t, "-r", "ExplicitType", dir)
	require.NoError(t, err)

	require.Equal(t, "ExplicitType", cfg.RootTypeName)
}

func TestReadConfig_CustomSchemaIdPrefix(t *testing.T) {
	dir := t.TempDir()

	cfg, err := readConfigForTest(t, "-p", "https://example.com/prefix", dir)
	require.NoError(t, err)

	expected := "https://example.com/prefix"
	require.Equal(t, expected, cfg.SchemaIDPrefix)
}

func readConfigForTest(t *testing.T, args ...string) (*Config, error) {
	t.Helper()

	origArgs := os.Args
	flag.CommandLine = flag.NewFlagSet(origArgs[0], flag.ContinueOnError)
	id = flag.String("p", "", "Schema ID prefix")
	rootType = flag.String("r", "", "Root type name (default is derived from file name)")
	output = flag.String("o", DefaultSchemaFileName, "Output schema file name (without extension)")
	fileType = flag.String("t", "yaml", "Output file type (yaml or json)")

	os.Args = append([]string{origArgs[0]}, args...)
	t.Cleanup(func() {
		os.Args = origArgs
	})

	return ReadConfig()
}
