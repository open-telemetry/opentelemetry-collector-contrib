// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package source

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/internal/credentialsfile"
)

func TestFileSource(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "header-value")
	require.NoError(t, os.WriteFile(f, []byte("secret123"), 0o600))

	resolver, err := credentialsfile.NewValueResolver("", f, zaptest.NewLogger(t))
	require.NoError(t, err)
	require.NoError(t, resolver.Start(t.Context()))
	defer func() { require.NoError(t, resolver.Shutdown()) }()

	fs := &FileSource{Resolver: resolver}
	value, err := fs.Get(t.Context())
	assert.NoError(t, err)
	assert.Equal(t, "secret123", value)
}

func TestFileSource_UpdatesOnFileChange(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "header-value")
	require.NoError(t, os.WriteFile(f, []byte("original"), 0o600))

	resolver, err := credentialsfile.NewValueResolver("", f, zaptest.NewLogger(t))
	require.NoError(t, err)
	require.NoError(t, resolver.Start(t.Context()))
	defer func() { require.NoError(t, resolver.Shutdown()) }()

	fs := &FileSource{Resolver: resolver}
	value, err := fs.Get(t.Context())
	assert.NoError(t, err)
	assert.Equal(t, "original", value)

	// Update file
	require.NoError(t, os.WriteFile(f, []byte("updated"), 0o600))

	// Wait for file watcher to pick up the change
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		value, err := fs.Get(t.Context())
		assert.NoError(c, err)
		assert.Equal(c, "updated", value)
	}, 5*time.Second, 50*time.Millisecond)
}
