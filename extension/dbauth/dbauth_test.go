// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dbauth

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeProvider is a minimal Provider for tests: it returns a fixed credential and
// records the request it was called with.
type fakeProvider struct {
	cred    *Credential
	gotReq  Request
	gotCall bool
}

func (f *fakeProvider) GetCredential(_ context.Context, req Request) (*Credential, error) {
	f.gotCall = true
	f.gotReq = req
	return f.cred, nil
}

func TestProvider_FakeSatisfiesInterface(t *testing.T) {
	var p Provider = &fakeProvider{cred: &Credential{Secret: "tok"}}
	got, err := p.GetCredential(t.Context(), Request{Endpoint: "db:5432", Username: "monitor"})
	require.NoError(t, err)
	assert.Equal(t, "tok", got.Secret)
}

func TestProvider_RequestThreadedToProvider(t *testing.T) {
	f := &fakeProvider{cred: &Credential{Secret: "tok"}}
	var p Provider = f
	_, err := p.GetCredential(t.Context(), Request{Endpoint: "db:5432", Username: "monitor"})
	require.NoError(t, err)
	require.True(t, f.gotCall)
	assert.Equal(t, Request{Endpoint: "db:5432", Username: "monitor"}, f.gotReq,
		"per-connection inputs reach the provider via the Request")
}

func TestCredential_UsernameNilVsEmpty(t *testing.T) {
	empty := ""
	withEmpty := &Credential{Username: &empty}
	withNil := &Credential{Username: nil}

	require.NotNil(t, withEmpty.Username, "pointer to empty string is not nil")
	assert.Empty(t, *withEmpty.Username)
	assert.Nil(t, withNil.Username, "nil means: use the consumer's configured username")
}

func TestCredential_NotAfterNilVsSet(t *testing.T) {
	noExpiry := &Credential{Secret: "static"}
	assert.Equal(t, "static", noExpiry.Secret)
	assert.Nil(t, noExpiry.NotAfter, "nil NotAfter means no expiry applies")

	exp := time.Unix(1000, 0)
	withExpiry := &Credential{Secret: "token", NotAfter: &exp}
	assert.Equal(t, "token", withExpiry.Secret)
	require.NotNil(t, withExpiry.NotAfter)
	assert.Equal(t, exp, *withExpiry.NotAfter)
}
