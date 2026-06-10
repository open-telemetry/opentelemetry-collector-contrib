// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package basicauthextension

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.uber.org/zap/zaptest"
)

func TestBasicAuth_ClientFromFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	userFile := filepath.Join(dir, "username")
	passFile := filepath.Join(dir, "password")
	require.NoError(t, os.WriteFile(userFile, []byte("fileuser"), 0o600))
	require.NoError(t, os.WriteFile(passFile, []byte("filepass"), 0o600))

	ext := newClientAuthExtension(&Config{
		ClientAuth: &ClientAuthSettings{
			UsernameFile: userFile,
			PasswordFile: passFile,
		},
	})
	ext.logger = zaptest.NewLogger(t)
	require.NoError(t, ext.Start(t.Context(), componenttest.NewNopHost()))
	defer func() { require.NoError(t, ext.Shutdown(t.Context())) }()

	base := &mockRoundTripper{}
	rt, err := ext.RoundTripper(base)
	require.NoError(t, err)

	authCreds := base64.StdEncoding.EncodeToString([]byte("fileuser:filepass"))
	resp, err := rt.RoundTrip(&http.Request{Method: http.MethodGet})
	require.NoError(t, err)
	assert.Equal(t, fmt.Sprintf("Basic %s", authCreds), resp.Header.Get("Authorization"))

	cred, err := ext.PerRPCCredentials()
	require.NoError(t, err)
	md, err := cred.GetRequestMetadata(t.Context())
	require.NoError(t, err)
	assert.Equal(t, fmt.Sprintf("Basic %s", authCreds), md["authorization"])
}

func TestBasicAuth_FilePreferredOverInline(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	passFile := filepath.Join(dir, "password")
	require.NoError(t, os.WriteFile(passFile, []byte("fromfile"), 0o600))

	ext := newClientAuthExtension(&Config{
		ClientAuth: &ClientAuthSettings{
			Username:     "user",
			Password:     "frominline",
			PasswordFile: passFile,
		},
	})
	ext.logger = zaptest.NewLogger(t)
	require.NoError(t, ext.Start(t.Context(), componenttest.NewNopHost()))
	defer func() { require.NoError(t, ext.Shutdown(t.Context())) }()

	base := &mockRoundTripper{}
	rt, err := ext.RoundTripper(base)
	require.NoError(t, err)

	authCreds := base64.StdEncoding.EncodeToString([]byte("user:fromfile"))
	resp, err := rt.RoundTrip(&http.Request{Method: http.MethodGet})
	require.NoError(t, err)
	assert.Equal(t, fmt.Sprintf("Basic %s", authCreds), resp.Header.Get("Authorization"))
}

func TestBasicAuth_FileChangesPickedUp(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	passFile := filepath.Join(dir, "password")
	require.NoError(t, os.WriteFile(passFile, []byte("original"), 0o600))

	ext := newClientAuthExtension(&Config{
		ClientAuth: &ClientAuthSettings{
			Username:     "user",
			PasswordFile: passFile,
		},
	})
	ext.logger = zaptest.NewLogger(t)
	require.NoError(t, ext.Start(t.Context(), componenttest.NewNopHost()))
	defer func() { require.NoError(t, ext.Shutdown(t.Context())) }()

	base := &mockRoundTripper{}
	rt, err := ext.RoundTripper(base)
	require.NoError(t, err)

	// Verify initial value
	authCreds := base64.StdEncoding.EncodeToString([]byte("user:original"))
	resp, err := rt.RoundTrip(&http.Request{Method: http.MethodGet})
	require.NoError(t, err)
	assert.Equal(t, fmt.Sprintf("Basic %s", authCreds), resp.Header.Get("Authorization"))

	// Update file
	require.NoError(t, os.WriteFile(passFile, []byte("rotated"), 0o600))

	// Wait for watcher to pick up change
	updatedCreds := base64.StdEncoding.EncodeToString([]byte("user:rotated"))
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		resp, err := rt.RoundTrip(&http.Request{Method: http.MethodGet})
		if assert.NoError(c, err) {
			assert.Equal(c, fmt.Sprintf("Basic %s", updatedCreds), resp.Header.Get("Authorization"))
		}
	}, 5*time.Second, 50*time.Millisecond)
}

func TestBasicAuth_StartFailsMissingFile(t *testing.T) {
	t.Parallel()
	ext := newClientAuthExtension(&Config{
		ClientAuth: &ClientAuthSettings{
			Username:     "user",
			PasswordFile: "/nonexistent/path",
		},
	})
	ext.logger = zaptest.NewLogger(t)
	require.Error(t, ext.Start(t.Context(), componenttest.NewNopHost()))
}
