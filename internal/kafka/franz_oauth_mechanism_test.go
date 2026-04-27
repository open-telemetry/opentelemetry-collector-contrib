// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafka

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type stubTokenProvider struct {
	token string
	err   error
}

func (s stubTokenProvider) Token(context.Context) (string, error) { return s.token, s.err }

func TestNewKgoOAuthMechanism(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		m := newKgoOAuthMechanism(stubTokenProvider{token: "test-token"})
		require.Equal(t, "OAUTHBEARER", m.Name())

		_, init, err := m.Authenticate(t.Context(), "")
		require.NoError(t, err)
		require.True(t, bytes.Contains(init, []byte("auth=Bearer test-token")), "unexpected initial response: %q", string(init))
	})

	t.Run("token provider error bubbles", func(t *testing.T) {
		want := errors.New("boom")
		m := newKgoOAuthMechanism(stubTokenProvider{err: want})

		_, _, err := m.Authenticate(t.Context(), "")
		require.ErrorIs(t, err, want)
	})

	t.Run("empty token from provider fails fast", func(t *testing.T) {
		m := newKgoOAuthMechanism(stubTokenProvider{token: ""})

		_, _, err := m.Authenticate(t.Context(), "")
		require.ErrorIs(t, err, ErrEmptyToken)
	})
}
