// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// skipping windows to avoid this golang bug: https://github.com/golang/go/issues/51442
//go:build !windows

package cfgmetadatagen

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/configschema"
)

func TestMetadataFileWriter(t *testing.T) {
	tempDir := t.TempDir()
	w := newMetadataFileWriter(tempDir)
	err := w.write(configschema.CfgInfo{Group: "mygroup", Type: "mytype"}, []byte("hello"))
	require.NoError(t, err)
	file, err := os.Open(filepath.Join(tempDir, "mygroup", "mytype.yaml"))
	require.NoError(t, err)
	bytes, err := io.ReadAll(file)
	require.NoError(t, err)
	assert.EqualValues(t, "hello", bytes)
}
