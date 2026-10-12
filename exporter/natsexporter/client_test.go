// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package natsexporter

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nats-io/jwt/v2"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nkeys"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newUserNKey returns a fresh user key pair and its public key / seed.
func newUserNKey(t *testing.T) (nkeys.KeyPair, string, []byte) {
	t.Helper()
	kp, err := nkeys.CreateUser()
	require.NoError(t, err)
	pub, err := kp.PublicKey()
	require.NoError(t, err)
	seed, err := kp.Seed()
	require.NoError(t, err)
	return kp, pub, seed
}

// writeUserCreds builds a decorated NATS credentials file and returns its path
// together with the user JWT it embeds.
func writeUserCreds(t *testing.T) (path, userJWT string) {
	t.Helper()
	accountKey, err := nkeys.CreateAccount()
	require.NoError(t, err)
	userKey, err := nkeys.CreateUser()
	require.NoError(t, err)
	userPub, err := userKey.PublicKey()
	require.NoError(t, err)
	userSeed, err := userKey.Seed()
	require.NoError(t, err)

	userJWT, err = jwt.NewUserClaims(userPub).Encode(accountKey)
	require.NoError(t, err)

	creds, err := jwt.FormatUserConfig(userJWT, userSeed)
	require.NoError(t, err)

	path = filepath.Join(t.TempDir(), "user.creds")
	require.NoError(t, os.WriteFile(path, creds, 0o600))
	return path, userJWT
}

func TestSetTokenOption(t *testing.T) {
	t.Parallel()

	var options nats.Options
	setTokenOption(&options, &TokenConfig{Token: "s3cret"})
	assert.Equal(t, "s3cret", options.Token)
}

func TestSetUserOption(t *testing.T) {
	t.Parallel()

	var options nats.Options
	setUserOption(&options, &UserConfig{Username: "otel", Password: "pw"})
	assert.Equal(t, "otel", options.User)
	assert.Equal(t, "pw", options.Password)
}

func TestSetNkeyOption(t *testing.T) {
	t.Parallel()

	t.Run("sets nkey and a working signature callback", func(t *testing.T) {
		kp, pub, seed := newUserNKey(t)

		var options nats.Options
		require.NoError(t, setNkeyOption(&options, &NkeyConfig{PublicKey: pub, Seed: seed}))
		assert.Equal(t, pub, options.Nkey)
		require.NotNil(t, options.SignatureCB)

		nonce := []byte("nonce")
		sig, err := options.SignatureCB(nonce)
		require.NoError(t, err)
		assert.NoError(t, kp.Verify(nonce, sig))
	})

	t.Run("returns error for an invalid seed", func(t *testing.T) {
		var options nats.Options
		assert.Error(t, setNkeyOption(&options, &NkeyConfig{PublicKey: "U", Seed: []byte("not-a-seed")}))
	})
}

func TestSetNkeyJWTOption(t *testing.T) {
	t.Parallel()

	t.Run("sets the JWT callback and a signature callback", func(t *testing.T) {
		_, _, seed := newUserNKey(t)

		var options nats.Options
		require.NoError(t, setNkeyJWTOption(&options, &NkeyJWTConfig{JWT: "the-jwt", Seed: seed}))
		require.NotNil(t, options.UserJWT)
		gotJWT, err := options.UserJWT()
		require.NoError(t, err)
		assert.Equal(t, "the-jwt", gotJWT)
		assert.NotNil(t, options.SignatureCB)
	})

	t.Run("returns error for an invalid seed", func(t *testing.T) {
		var options nats.Options
		assert.Error(t, setNkeyJWTOption(&options, &NkeyJWTConfig{JWT: "the-jwt", Seed: []byte("bad")}))
	})
}

func TestSetNkeyUserFileOption(t *testing.T) {
	t.Parallel()

	t.Run("loads the JWT and signature callback from a creds file", func(t *testing.T) {
		path, wantJWT := writeUserCreds(t)

		var options nats.Options
		require.NoError(t, setNkeyUserFileOption(&options, &NkeyUserFileConfig{UserFilePath: path}))
		require.NotNil(t, options.UserJWT)
		gotJWT, err := options.UserJWT()
		require.NoError(t, err)
		assert.Equal(t, wantJWT, gotJWT)
		assert.NotNil(t, options.SignatureCB)
	})

	t.Run("returns error when the file is missing", func(t *testing.T) {
		var options nats.Options
		assert.Error(t, setNkeyUserFileOption(&options, &NkeyUserFileConfig{UserFilePath: filepath.Join(t.TempDir(), "absent.creds")}))
	})
}

func TestSetAuthOption(t *testing.T) {
	t.Parallel()

	t.Run("dispatches to the configured method", func(t *testing.T) {
		var options nats.Options
		require.NoError(t, setAuthOption(&options, &AuthConfig{
			User: &UserConfig{Username: "otel", Password: "pw"},
		}))
		assert.Equal(t, "otel", options.User)
		assert.Equal(t, "pw", options.Password)
	})

	t.Run("propagates a setter error", func(t *testing.T) {
		var options nats.Options
		assert.Error(t, setAuthOption(&options, &AuthConfig{
			Nkey: &NkeyConfig{PublicKey: "U", Seed: []byte("bad")},
		}))
	})

	t.Run("no auth configured is a no-op", func(t *testing.T) {
		var options nats.Options
		require.NoError(t, setAuthOption(&options, &AuthConfig{}))
		assert.Empty(t, options.Token)
		assert.Empty(t, options.User)
		assert.Nil(t, options.SignatureCB)
	})
}
