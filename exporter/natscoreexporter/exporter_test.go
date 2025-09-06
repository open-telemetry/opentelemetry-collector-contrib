package natscoreexporter

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/jwt/v2"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nkeys"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/testdata"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/natscoreexporter/internal/metadata"
)

type mockOption interface {
	set(cfg *Config, options *server.Options)
}

type mockSecureOption struct {
	clientConfig configtls.ClientConfig
	verifyAndMap bool
}

func (m *mockSecureOption) set(cfg *Config, options *server.Options) {
	cfg.TLS = m.clientConfig

}

var _ mockOption = (*mockSecureOption)(nil)

func createMockSecureOption(verifyAndMap bool) *mockSecureOption {
	// TODO: Implement TLS
	return &mockSecureOption{
		clientConfig: configtls.NewDefaultClientConfig(),
		verifyAndMap: verifyAndMap,
	}
}

type mockTokenOption struct {
	token string
}

func (m *mockTokenOption) set(cfg *Config, options *server.Options) {
	cfg.Auth.Token = &TokenConfig{
		Token: m.token,
	}
	options.Authorization = m.token
}

var _ mockOption = (*mockTokenOption)(nil)

func createMockTokenOption() *mockTokenOption {
	return &mockTokenOption{
		token: uuid.NewString(),
	}
}

type mockUserInfoOption struct {
	user     string
	password string
}

func (m *mockUserInfoOption) set(cfg *Config, options *server.Options) {
	cfg.Auth.UserInfo = &UserInfoConfig{
		User:     m.user,
		Password: m.password,
	}
	options.Users = []*server.User{
		{
			Username: m.user,
			Password: m.password,
		},
	}
}

var _ mockOption = (*mockUserInfoOption)(nil)

func createMockUserInfoOption() *mockUserInfoOption {
	return &mockUserInfoOption{
		user:     uuid.NewString(),
		password: uuid.NewString(),
	}
}

type mockNkeyOption struct {
	publicKey string
	seed      []byte
}

func (m *mockNkeyOption) set(cfg *Config, options *server.Options) {
	cfg.Auth.Nkey = &NkeyConfig{
		PublicKey: m.publicKey,
		Seed:      m.seed,
	}
	options.Nkeys = []*server.NkeyUser{
		{
			Nkey: m.publicKey,
		},
	}
}

var _ mockOption = (*mockNkeyOption)(nil)

func createMockNkeyOption(t *testing.T) *mockNkeyOption {
	keyPair, err := nkeys.CreateUser()
	require.NoError(t, err)
	t.Cleanup(func() {
		keyPair.Wipe()
	})

	publicKey, err := keyPair.PublicKey()
	require.NoError(t, err)

	seed, err := keyPair.Seed()
	require.NoError(t, err)

	return &mockNkeyOption{
		publicKey: publicKey,
		seed:      seed,
	}
}

type mockUserJWTOption struct {
	jwt  string
	seed []byte
}

func (m *mockUserJWTOption) set(cfg *Config, options *server.Options) {
	cfg.Auth.UserJWT = &UserJWTConfig{
		JWT:  m.jwt,
		Seed: m.seed,
	}
}

var _ mockOption = (*mockUserJWTOption)(nil)

func createMockUserJWTOption(t *testing.T) *mockUserJWTOption {
	accountKeyPair, err := nkeys.CreateAccount()
	require.NoError(t, err)
	t.Cleanup(func() {
		accountKeyPair.Wipe()
	})

	accountPublicKey, err := accountKeyPair.PublicKey()
	require.NoError(t, err)

	nkey := createMockNkeyOption(t)
	var (
		userPublicKey = nkey.publicKey
		userSeed      = nkey.seed
	)

	userJwt, err := jwt.IssueUserJWT(
		accountKeyPair,
		accountPublicKey,
		userPublicKey,
		fmt.Sprintf("%s/%s", t.Name(), userPublicKey),
		5*time.Minute,
	)
	require.NoError(t, err)

	return &mockUserJWTOption{
		jwt:  userJwt,
		seed: userSeed,
	}
}

type mockUserCredentialsOption struct {
	userFilePath string
}

func (m *mockUserCredentialsOption) set(cfg *Config, options *server.Options) {
	cfg.Auth.UserCredentials = &UserCredentialsConfig{
		UserFilePath: m.userFilePath,
	}
}

var _ mockOption = (*mockUserCredentialsOption)(nil)

func createMockUserCredentialsOption(t *testing.T) *mockUserCredentialsOption {
	userJWT := createMockUserJWTOption(t)
	var (
		jwtString = userJWT.jwt
		seed      = userJWT.seed
	)

	userConfig, err := jwt.FormatUserConfig(jwtString, seed)
	require.NoError(t, err)

	userFile, err := os.CreateTemp("", "")
	require.NoError(t, err)
	t.Cleanup(func() {
		os.Remove(userFile.Name())
	})
	userFilePath := userFile.Name()

	_, err = userFile.Write(userConfig)
	require.NoError(t, err)

	return &mockUserCredentialsOption{
		userFilePath: userFilePath,
	}
}

func mockAndRun(
	t *testing.T,
	mockOptions []mockOption,
	cb func(
		set exporter.Settings,
		cfg *Config,
		ctx context.Context,
		host component.Host,
	),
) {
	var options server.Options
	set := exportertest.NewNopSettings(metadata.Type)
	cfg := createDefaultConfig().(*Config)
	ctx := t.Context()
	host := componenttest.NewNopHost()

	for _, mockOption := range mockOptions {
		mockOption.set(cfg, &options)
	}

	server, err := server.NewServer(&options)
	require.NoError(t, err)

	server.Start()
	defer server.Shutdown()

	cb(set, cfg, ctx, host)
}

// TODO: Test that sending stuff works
//  - Test for each signal maybe?
// TODO: Test config options
//  - TLS option
//  - All the auth options
//  - We don't need to test options where we just set values
//
// I guess we're just after e2e testing all configurations?

func TestNatsCoreExporter(t *testing.T) {
	t.Parallel()

	t.Run("basic", func(t *testing.T) {
		mockAndRun(t, []mockOption{
			createMockSecureOption(false),
			createMockTokenOption(),
			createMockUserInfoOption(),
			createMockNkeyOption(t),
			createMockUserJWTOption(t),
			createMockUserCredentialsOption(t),
		}, func(
			set exporter.Settings,
			cfg *Config,
			ctx context.Context,
			host component.Host,
		) {
			require.NoError(t, cfg.Validate())

			exporter, err := newNatsCoreLogsExporter(set, cfg)
			assert.NoError(t, err)

			err = exporter.start(ctx, host)
			assert.NoError(t, err)

			err = exporter.export(ctx, testdata.GenerateLogs(10))
			assert.NoError(t, err)

			err = exporter.shutdown(ctx)
			assert.NoError(t, err)
		})
	})
}
