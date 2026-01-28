// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"flag"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestReadConfig(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	cfg, err := readConfigForTest(t, dir)
	require.NoError(t, err)

	require.Equal(t, Package, cfg.Mode)
	require.Equal(t, dir, cfg.DirPath)
	require.Equal(t, dir, cfg.OutputFolder)
	require.Empty(t, cfg.ConfigType)
}

func TestReadConfig_Errors(t *testing.T) {
	t.Run("missing path", func(t *testing.T) {
		t.Chdir(t.TempDir())
		missing := filepath.Join(t.TempDir(), "missing")
		_, err := readConfigForTest(t, missing)
		require.Error(t, err)
	})

	t.Run("unknown file type", func(t *testing.T) {
		dir := t.TempDir()
		t.Chdir(dir)
		file := createConfigFile(t, dir, "config.go")

		_, err := readConfigForTest(t, "-t", "xml", file)
		require.Error(t, err)
	})
}

func TestReadConfig_RespectsRootTypeFlag(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	target := createConfigFile(t, dir, "component.go")

	cfg, err := readConfigForTest(t, "-r", "ExplicitType", target)
	require.NoError(t, err)

	require.Equal(t, "ExplicitType", cfg.ConfigType)
}

func TestReadConfig_ReadsSettingsFile(t *testing.T) {
	projectDir := t.TempDir()
	settings := Settings{
		Mappings: Mappings{
			"pkg": PackagesMapping{
				"Thing": {
					SchemaType: SchemaTypeString,
					Format:     "uuid",
				},
			},
		},
	}
	data, err := yaml.Marshal(settings)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(projectDir, SettingsFileName), data, 0o600))

	workDir := filepath.Join(projectDir, "workdir")
	require.NoError(t, os.Mkdir(workDir, 0o700))
	t.Chdir(workDir)

	target := createConfigFile(t, workDir, "component.go")

	cfg, err := readConfigForTest(t, target)
	require.NoError(t, err)

	expectedOutput := filepath.Join(projectDir, "workdir")
	require.Equal(t, evalPath(t, expectedOutput), evalPath(t, cfg.OutputFolder))
	require.Equal(t, Mappings{
		"pkg": PackagesMapping{
			"Thing": {SchemaType: SchemaTypeString, Format: "uuid"},
		},
	}, cfg.Mappings)
}

func createConfigFile(t *testing.T, dir, name string) string {
	t.Helper()
	target := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(target, []byte("package test"), 0o600))
	return target
}

func readConfigForTest(t *testing.T, args ...string) (*Config, error) {
	t.Helper()

	origArgs := os.Args
	origCommandLine := flag.CommandLine
	origRootType := configType
	origOutputFolder := outputFolder
	origFileType := fileType

	flag.CommandLine = flag.NewFlagSet(origArgs[0], flag.ContinueOnError)
	flag.CommandLine.SetOutput(io.Discard)

	configType = flag.String("r", "", "Root type name (default is derived from file name)")
	outputFolder = flag.String("o", "", "Output schema folder")
	fileType = flag.String("t", "yaml", "Output file type (yaml or json)")

	os.Args = append([]string{origArgs[0]}, args...)
	t.Cleanup(func() {
		os.Args = origArgs
		flag.CommandLine = origCommandLine
		configType = origRootType
		outputFolder = origOutputFolder
		fileType = origFileType
	})

	return ReadConfig()
}

func evalPath(t *testing.T, path string) string {
	t.Helper()
	dir := filepath.Dir(path)
	resolved, err := filepath.EvalSymlinks(dir)
	require.NoError(t, err)
	return filepath.Join(resolved, filepath.Base(path))
}
