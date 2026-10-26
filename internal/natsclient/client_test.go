// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package natsclient

import (
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/nats-io/jwt/v2"
	"github.com/nats-io/nats-server/v2/server"
	natstest "github.com/nats-io/nats-server/v2/test"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nkeys"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
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

func TestConnect(t *testing.T) {
	t.Parallel()

	opts := natstest.DefaultTestOptions
	opts.Port = -1 // random free port
	srv := natstest.RunServer(&opts)
	defer srv.Shutdown()

	cfg := NewDefaultClientConfig()
	cfg.Endpoint = srv.ClientURL()
	cfg.Pedantic = true

	conn, err := Connect(t.Context(), &cfg, "otelcol-test", &statusHost{}, zap.NewNop())
	require.NoError(t, err)
	defer conn.Close()
	assert.True(t, conn.IsConnected())
	assert.Equal(t, "otelcol-test", conn.Opts.Name)
	assert.True(t, conn.Opts.Pedantic)
	assert.Equal(t, -1, conn.Opts.MaxReconnect, "the connection must never give up reconnecting")
}

func TestConnectInvalidAuth(t *testing.T) {
	t.Parallel()

	cfg := NewDefaultClientConfig()
	cfg.Auth.Nkey = &NkeyConfig{PublicKey: "U", Seed: []byte("bad")}

	_, err := Connect(t.Context(), &cfg, "otelcol-test", &statusHost{}, zap.NewNop())
	assert.Error(t, err)
}

// TestConnectLogsSlowConsumer verifies that async errors such as a slow
// consumer dropping messages are logged with the affected subject.
func TestConnectLogsSlowConsumer(t *testing.T) {
	t.Parallel()

	opts := natstest.DefaultTestOptions
	opts.Port = -1 // random free port
	srv := natstest.RunServer(&opts)
	defer srv.Shutdown()

	cfg := NewDefaultClientConfig()
	cfg.Endpoint = srv.ClientURL()
	core, logs := observer.New(zapcore.WarnLevel)

	conn, err := Connect(t.Context(), &cfg, "otelcol-test", &statusHost{}, zap.New(core))
	require.NoError(t, err)
	defer conn.Close()

	// A handler that blocks with a pending limit of one message makes the
	// following publishes overflow the subscription.
	release := make(chan struct{})
	defer close(release)
	sub, err := conn.Subscribe("slow", func(*nats.Msg) { <-release })
	require.NoError(t, err)
	require.NoError(t, sub.SetPendingLimits(1, -1))
	for range 3 {
		require.NoError(t, conn.Publish("slow", []byte("x")))
	}
	require.NoError(t, conn.Flush())

	require.Eventually(t, func() bool { return logs.FilterMessage("NATS async error").Len() > 0 }, 5*time.Second, 10*time.Millisecond)
	entry := logs.FilterMessage("NATS async error").All()[0]
	assert.Equal(t, "slow", entry.ContextMap()["subject"])
	assert.Contains(t, entry.ContextMap()["error"], nats.ErrSlowConsumer.Error())
}

// statusHost is a component.Host that records the reported status events.
type statusHost struct {
	mu     sync.Mutex
	events []*componentstatus.Event
}

func (*statusHost) GetExtensions() map[component.ID]component.Component { return nil }

func (h *statusHost) Report(e *componentstatus.Event) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.events = append(h.events, e)
}

func (h *statusHost) statuses() []componentstatus.Status {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]componentstatus.Status, len(h.events))
	for i, e := range h.events {
		out[i] = e.Status()
	}
	return out
}

func (h *statusHost) last() *componentstatus.Event {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.events) == 0 {
		return nil
	}
	return h.events[len(h.events)-1]
}

// runRestartableServer starts a server on a random port and returns its
// options with that port fixed, so it can be restarted on the same address.
func runRestartableServer(t *testing.T) (*server.Server, server.Options) {
	t.Helper()
	opts := natstest.DefaultTestOptions
	opts.Port = -1 // random free port
	srv := natstest.RunServer(&opts)
	opts.Port = srv.Addr().(*net.TCPAddr).Port
	return srv, opts
}

// TestConnectReportsStatus verifies that a lost connection is reported as a
// recoverable error, a reconnect as OK, and an intentional close not at all.
func TestConnectReportsStatus(t *testing.T) {
	t.Parallel()

	srv, opts := runRestartableServer(t)
	cfg := NewDefaultClientConfig()
	cfg.Endpoint = srv.ClientURL()
	host := &statusHost{}

	conn, err := Connect(t.Context(), &cfg, "otelcol-test", host, zap.NewNop())
	require.NoError(t, err)
	assert.Empty(t, host.statuses())

	srv.Shutdown()
	require.Eventually(t, func() bool {
		e := host.last()
		return e != nil && e.Status() == componentstatus.StatusRecoverableError
	}, 5*time.Second, 10*time.Millisecond)

	srv = natstest.RunServer(&opts)
	defer srv.Shutdown()
	require.Eventually(t, func() bool {
		e := host.last()
		return e != nil && e.Status() == componentstatus.StatusOK
	}, 10*time.Second, 10*time.Millisecond)

	reported := len(host.statuses())
	conn.Close()
	require.Eventually(t, conn.IsClosed, 5*time.Second, 10*time.Millisecond)
	// Close callbacks run asynchronously; give them time to (not) report.
	time.Sleep(100 * time.Millisecond)
	assert.Len(t, host.statuses(), reported, "an intentional close must not report status")
}

// TestConnectReportsPermanentWhenGivingUp verifies that a connection nats.go
// closes for good, here after the server rejects the credentials twice, is
// reported as a permanent error rather than going silent.
func TestConnectReportsPermanentWhenGivingUp(t *testing.T) {
	t.Parallel()

	srv, opts := runRestartableServer(t)
	cfg := NewDefaultClientConfig()
	cfg.Endpoint = srv.ClientURL()
	host := &statusHost{}

	conn, err := Connect(t.Context(), &cfg, "otelcol-test", host, zap.NewNop())
	require.NoError(t, err)
	defer conn.Close()

	// Come back requiring credentials the client does not have.
	srv.Shutdown()
	opts.Username, opts.Password = "otel", "s3cret"
	srv = natstest.RunServer(&opts)
	defer srv.Shutdown()

	require.Eventually(t, func() bool {
		e := host.last()
		return e != nil && e.Status() == componentstatus.StatusPermanentError
	}, 20*time.Second, 50*time.Millisecond)
	assert.ErrorIs(t, host.last().Err(), nats.ErrAuthorization)
}
