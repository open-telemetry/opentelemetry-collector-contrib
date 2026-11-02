// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package oracledbreceiver

import (
	"encoding/binary"
	"os"
	"testing"

	"github.com/jcmturner/gokrb5/v8/client"
	"github.com/jcmturner/gokrb5/v8/config"
	"github.com/jcmturner/gokrb5/v8/gssapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeMinimalKrb5Conf writes a minimal krb5.conf that gokrb5 can parse.
func writeMinimalKrb5Conf(path string) error {
	const conf = `[libdefaults]
  default_realm = EXAMPLE.COM

[realms]
  EXAMPLE.COM = {
    kdc = kdc.example.com
  }
`
	return os.WriteFile(path, []byte(conf), 0o600)
}

func TestGSSAPIChecksum(t *testing.T) {
	c := gssAPIChecksum([]int{gssapi.ContextFlagMutual, gssapi.ContextFlagInteg})

	// RFC 4121 §4.1.1: 24-byte checksum, channel-binding length 16 in the first
	// four bytes and the context flags in the last four, both little-endian.
	require.Len(t, c, 24)
	assert.Equal(t, uint32(16), binary.LittleEndian.Uint32(c[:4]))

	wantFlags := uint32(gssapi.ContextFlagMutual | gssapi.ContextFlagInteg)
	assert.Equal(t, wantFlags, binary.LittleEndian.Uint32(c[20:24]))

	// Bytes between the length and the flags are reserved and must be zero.
	for _, b := range c[4:20] {
		assert.Equal(t, byte(0), b)
	}
}

func TestNewKerberosClientInvalidConfigFile(t *testing.T) {
	_, err := newKerberosClient(&KerberosConfig{
		CredentialType: KerberosCredentialKeytab,
		Realm:          "EXAMPLE.COM",
		Principal:      "otel",
		ConfigFile:     "/nonexistent/krb5.conf",
		KeytabFile:     "/nonexistent/otel.keytab",
	})
	require.Error(t, err)
	assert.ErrorContains(t, err, "load krb5 config")
}

func TestNewKerberosClientUnknownCredentialType(t *testing.T) {
	// A valid krb5.conf is required to reach the credential-type switch.
	dir := t.TempDir()
	confPath := dir + "/krb5.conf"
	require.NoError(t, writeMinimalKrb5Conf(confPath))

	_, err := newKerberosClient(&KerberosConfig{
		CredentialType: "smartcard",
		Realm:          "EXAMPLE.COM",
		Principal:      "otel",
		ConfigFile:     confPath,
	})
	require.ErrorIs(t, err, errInvalidCredentialType)
}

// TestKerberosAuthClose verifies that close destroys the underlying gokrb5
// client (which stops the background TGT-renewal goroutine for keytab/password
// clients). Destroy resets the client's credentials, so an empty username after
// close confirms the client was torn down.
func TestKerberosAuthClose(t *testing.T) {
	dir := t.TempDir()
	confPath := dir + "/krb5.conf"
	require.NoError(t, writeMinimalKrb5Conf(confPath))

	krbCfg, err := config.Load(confPath)
	require.NoError(t, err)

	cl := client.NewWithPassword("otel", "EXAMPLE.COM", "secret", krbCfg)
	require.Equal(t, "otel", cl.Credentials.UserName())

	k := &kerberosAuth{
		cfg: &KerberosConfig{CredentialType: KerberosCredentialPassword},
		cl:  cl,
	}

	k.close()
	assert.Empty(t, cl.Credentials.UserName())
}

// TestKerberosAuthCloseNilClient verifies close is a safe no-op when no client
// was ever set (e.g. the opener failed before assigning one).
func TestKerberosAuthCloseNilClient(t *testing.T) {
	k := &kerberosAuth{cfg: &KerberosConfig{CredentialType: KerberosCredentialCache}}
	assert.NotPanics(t, k.close)
}

// TestAuthenticateNonCacheNoReload verifies that a service-ticket failure for a
// non-ccache credential type is returned directly, without attempting a
// credential cache reload. The password client has no TGT (Login was never
// called), so GetServiceTicket fails.
func TestAuthenticateNonCacheNoReload(t *testing.T) {
	dir := t.TempDir()
	confPath := dir + "/krb5.conf"
	require.NoError(t, writeMinimalKrb5Conf(confPath))

	krbCfg, err := config.Load(confPath)
	require.NoError(t, err)

	k := &kerberosAuth{
		cfg: &KerberosConfig{CredentialType: KerberosCredentialPassword},
		cl:  client.NewWithPassword("otel", "EXAMPLE.COM", "secret", krbCfg),
	}

	_, err = k.Authenticate("oracledb", "oracle")
	require.Error(t, err)
	assert.ErrorContains(t, err, "get service ticket")
	// The non-ccache path must not attempt a reload.
	assert.NotContains(t, err.Error(), "reload credential cache")
}

// TestAuthenticateCacheReloadFails verifies that when a ccache client fails to
// get a service ticket, Authenticate attempts to reload from the cache file and
// surfaces the reload error when that file is unreadable.
func TestAuthenticateCacheReloadFails(t *testing.T) {
	dir := t.TempDir()
	confPath := dir + "/krb5.conf"
	require.NoError(t, writeMinimalKrb5Conf(confPath))

	krbCfg, err := config.Load(confPath)
	require.NoError(t, err)

	k := &kerberosAuth{
		cfg: &KerberosConfig{
			CredentialType:  KerberosCredentialCache,
			Realm:           "EXAMPLE.COM",
			Principal:       "otel",
			ConfigFile:      confPath,
			CredentialCache: "/nonexistent/orauser.ccache",
		},
		cl: client.NewWithPassword("otel", "EXAMPLE.COM", "secret", krbCfg),
	}

	_, err = k.Authenticate("oracledb", "oracle")
	require.Error(t, err)
	assert.ErrorContains(t, err, "reload credential cache")
}
