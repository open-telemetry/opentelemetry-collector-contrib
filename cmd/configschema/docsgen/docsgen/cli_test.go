// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Skip tests on Windows temporarily, see https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/11451
//go:build !windows
// +build !windows

package docsgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"text/template"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/otelcol"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/configschema"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/components"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/redisreceiver"
)

func TestWriteConfigDoc(t *testing.T) {
	cfg := redisreceiver.NewFactory().CreateDefaultConfig()
	dr := configschema.NewDirResolver(filepath.Join("..", "..", "..", ".."), configschema.DefaultModule)
	outputFilename := ""
	tmpl := testTemplate(t)
	writeConfigDoc(
		tmpl,
		dr,
		configschema.CfgInfo{
			Group:       "receiver",
			Type:        "redis",
			CfgInstance: cfg,
		},
		func(dir string, bytes []byte, perm os.FileMode) error {
			outputFilename = dir
			return nil
		},
	)
	expectedPath := filepath.Join("receiver", "redisreceiver", "config.md")
	assert.True(t, strings.HasSuffix(outputFilename, expectedPath))
}

func TestHandleCLI_NoArgs(t *testing.T) {
	wr := &fakeIOWriter{}
	handleCLI(
		defaultComponents(t),
		configschema.NewDefaultDirResolver(),
		testTemplate(t),
		func(filename string, data []byte, perm os.FileMode) error { return nil },
		wr,
	)
	assert.Equal(t, 3, len(wr.lines))
}

func TestHandleCLI_Single(t *testing.T) {
	args := []string{"", "receiver", "redis"}
	cs := defaultComponents(t)
	wr := &fakeFilesystemWriter{}

	testHandleCLI(t, cs, wr, args)

	assert.Equal(t, 1, len(wr.configFiles))
	assert.Equal(t, 1, len(wr.fileContents))
	assert.True(t, strings.Contains(wr.fileContents[0], `Redis Receiver Reference`))
}

func TestHandleCLI_All(t *testing.T) {
	t.Skip("this test takes > 5m when -race is used")
	args := []string{"", "all"}
	c := defaultComponents(t)
	writer := &fakeFilesystemWriter{}
	testHandleCLI(t, c, writer, args)
	assert.NotNil(t, writer.configFiles)
	assert.NotNil(t, writer.fileContents)
}

func defaultComponents(t *testing.T) otelcol.Factories {
	factories, err := components.Components()
	require.NoError(t, err)
	return factories
}

func testHandleCLI(t *testing.T, cs otelcol.Factories, wr *fakeFilesystemWriter, args []string) {
	stdoutWriter := &fakeIOWriter{}
	tmpl := testTemplate(t)
	dr := configschema.NewDirResolver(filepath.Join("..", "..", "..", ".."), configschema.DefaultModule)
	handleCLI(cs, dr, tmpl, wr.writeFile, stdoutWriter, args...)
}

func testTemplate(t *testing.T) *template.Template {
	tmpl, err := template.ParseFiles("testdata/test.tmpl")
	require.NoError(t, err)
	return tmpl
}

type fakeFilesystemWriter struct {
	configFiles, fileContents []string
}

func (wr *fakeFilesystemWriter) writeFile(filename string, data []byte, _ os.FileMode) error {
	wr.configFiles = append(wr.configFiles, filename)
	wr.fileContents = append(wr.fileContents, string(data))
	return nil
}

type fakeIOWriter struct {
	lines []string
}

func (wr *fakeIOWriter) Write(p []byte) (n int, err error) {
	wr.lines = append(wr.lines, string(p))
	return 0, nil
}
