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
// records the request and extension args it was called with.
type fakeProvider struct {
	cred    *Credential
	gotReq  Request
	gotArgs map[string]any
	gotCall bool
}

func (f *fakeProvider) GetCredential(_ context.Context, req Request, extensionArgs map[string]any) (*Credential, error) {
	f.gotCall = true
	f.gotReq = req
	f.gotArgs = extensionArgs
	return f.cred, nil
}

func TestProvider_FakeSatisfiesInterface(t *testing.T) {
	var p Provider = &fakeProvider{cred: &Credential{Secret: "tok"}}
	got, err := p.GetCredential(context.Background(), Request{Endpoint: "db:5432", Username: "monitor"}, nil)
	require.NoError(t, err)
	assert.Equal(t, "tok", got.Secret)
}

func TestProvider_RequestThreadedToProvider(t *testing.T) {
	f := &fakeProvider{cred: &Credential{Secret: "tok"}}
	var p Provider = f
	args := map[string]any{"region": "us-east-1"}
	_, err := p.GetCredential(context.Background(), Request{Endpoint: "db:5432", Username: "monitor"}, args)
	require.NoError(t, err)
	require.True(t, f.gotCall)
	assert.Equal(t, Request{Endpoint: "db:5432", Username: "monitor"}, f.gotReq,
		"per-connection inputs reach the provider via the Request")
	assert.Equal(t, args, f.gotArgs,
		"the consumer's inline override reaches the provider as extensionArgs")
}

func TestCredential_UsernameNilVsEmpty(t *testing.T) {
	empty := ""
	withEmpty := &Credential{Username: &empty}
	withNil := &Credential{Username: nil}

	require.NotNil(t, withEmpty.Username, "pointer to empty string is not nil")
	assert.Equal(t, "", *withEmpty.Username)
	assert.Nil(t, withNil.Username, "nil means: use the consumer's configured username")
}

func TestCredential_NotAfterNilVsSet(t *testing.T) {
	noExpiry := &Credential{Secret: "static"}
	assert.Nil(t, noExpiry.NotAfter, "nil NotAfter means no expiry applies")

	exp := time.Unix(1000, 0)
	withExpiry := &Credential{Secret: "token", NotAfter: &exp}
	require.NotNil(t, withExpiry.NotAfter)
	assert.Equal(t, exp, *withExpiry.NotAfter)
}
