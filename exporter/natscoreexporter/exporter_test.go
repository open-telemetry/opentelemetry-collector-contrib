package natscoreexporter

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/jwt/v2"
	"github.com/nats-io/nkeys"
	"github.com/stretchr/testify/require"
)

func createToken(t *testing.T) string {
	t.Helper()

	return uuid.NewString()
}

func createUserInfo(t *testing.T) (string, string) {
	t.Helper()

	return uuid.NewString(), uuid.NewString()
}

func createNkey(t *testing.T) (string, string) {
	t.Helper()

	userKeyPair, err := nkeys.CreateUser()
	require.NoError(t, err)
	t.Cleanup(func() {
		userKeyPair.Wipe()
	})

	userPubKey, err := userKeyPair.PublicKey()
	require.NoError(t, err)

	userSeed, err := userKeyPair.Seed()
	require.NoError(t, err)

	return userPubKey, string(userSeed)
}

func createUserJWT(t *testing.T) (string, string) {
	t.Helper()

	accountKeyPair, err := nkeys.CreateAccount()
	require.NoError(t, err)
	t.Cleanup(func() {
		accountKeyPair.Wipe()
	})

	accountPubKey, err := accountKeyPair.PublicKey()
	require.NoError(t, err)

	userPubKey, userSeed := createNkey(t)

	userJwt, err := jwt.IssueUserJWT(
		accountKeyPair,
		accountPubKey,
		userPubKey,
		fmt.Sprintf("%s/%s", t.Name(), userPubKey),
		5*time.Minute,
	)
	require.NoError(t, err)

	return userJwt, userSeed
}

func createUserCredentials(t *testing.T) string {
	t.Helper()

	userJwt, userSeed := createUserJWT(t)

	userConfig, err := jwt.FormatUserConfig(userJwt, []byte(userSeed))
	require.NoError(t, err)

	tempFile, err := os.CreateTemp("", "")
	require.NoError(t, err)
	t.Cleanup(func() {
		os.Remove(tempFile.Name())
	})

	_, err = tempFile.Write(userConfig)
	require.NoError(t, err)

	return tempFile.Name()
}

// TODO: Test that sending stuff works
//  - Test for each signal maybe?
// TODO: Test config options
//  - TLS option
//  - All the auth options
//  - We don't need to test options where we just set values
//
// I guess we're just after e2e testing all configurations?
