package natscoreexporter

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/nats-io/jwt/v2"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nkeys"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/exporter/exportertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/natscoreexporter/internal/grouper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/natscoreexporter/internal/marshaler"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/natscoreexporter/internal/metadata"
)

func createNkey(t *testing.T) (string, []byte) {
	keyPair, err := nkeys.CreateUser()
	require.NoError(t, err)
	publicKey, err := keyPair.PublicKey()
	require.NoError(t, err)
	seed, err := keyPair.Seed()
	require.NoError(t, err)

	return publicKey, seed
}

func createNkeyJWT(t *testing.T) (string, []byte) {
	userPublicKey, userSeed := createNkey(t)

	accountKeyPair, err := nkeys.CreateAccount()
	require.NoError(t, err)
	accountPublicKey, err := accountKeyPair.PublicKey()
	require.NoError(t, err)
	defer accountKeyPair.Wipe()

	userJWT, err := jwt.IssueUserJWT(
		accountKeyPair,
		accountPublicKey,
		userPublicKey,
		"user",
		5*time.Minute,
	)
	require.NoError(t, err)

	return userJWT, userSeed
}

func createNkeyUserFile(t *testing.T) string {
	userJWT, userSeed := createNkeyJWT(t)

	userConfig, err := jwt.FormatUserConfig(userJWT, userSeed)
	require.NoError(t, err)

	userFile, err := os.CreateTemp("", "")
	require.NoError(t, err)

	userFilePath := userFile.Name()
	t.Cleanup(func() {
		err := os.Remove(userFilePath)
		require.NoError(t, err)
	})

	_, err = userFile.Write(userConfig)
	require.NoError(t, err)
	err = userFile.Close()
	require.NoError(t, err)

	return userFilePath
}

// TODO: Test that sending stuff works
//  - Test for each signal maybe?
// TODO: Test the Nkey auth options
//
// What other non-trivial things should we test?

type recorder struct {
	t    *testing.T
	conn *nats.Conn
	sub  *nats.Subscription
}

func (r *recorder) connect(url string) {
	conn, err := nats.Connect(url)
	require.NoError(r.t, err)
	r.conn = conn

	r.sub, err = conn.SubscribeSync(">")
	require.NoError(r.t, err)
}

func (r *recorder) record(msgCount int, timeout time.Duration) []nats.Msg {
	ctx, cancel := context.WithTimeout(r.t.Context(), timeout)
	defer cancel()

	msgs := make([]nats.Msg, 0, msgCount)
	for range msgCount {
		msg, err := r.sub.NextMsgWithContext(ctx)
		if err == nil {
			msgs = append(msgs, *msg)
		} else {
			require.ErrorIs(r.t, err, context.DeadlineExceeded)
		}
	}
	return msgs
}

func (r *recorder) disconnect() {
	err := r.sub.Unsubscribe()
	require.NoError(r.t, err)

	r.conn.Close()
}

func newRecorder(t *testing.T) *recorder {
	return &recorder{
		t: t,
	}
}

func withTestServer(
	t *testing.T,
	callback func(cfg *Config, recorder *recorder),
) {
	options := &server.Options{}
	cfg := createDefaultConfig().(*Config)

	recorder := newRecorder(t)

	server, err := server.NewServer(options)
	require.NoError(t, err)
	server.Start()
	defer server.Shutdown()

	recorder.connect(server.ClientURL())
	defer recorder.disconnect()

	cfg.Endpoint = server.ClientURL()
	callback(cfg, recorder)
}

type fakeGrouper struct{}

func (g *fakeGrouper) Group(ctx context.Context, data string) ([]grouper.Group[string], error) {
	return []grouper.Group[string]{{Subject: data, Data: data}}, nil
}

var _ grouper.Grouper[string] = (*fakeGrouper)(nil)

type fakeGenericMarshaler struct{}

func (m *fakeGenericMarshaler) MarshalString(sd string) ([]byte, error) {
	return []byte(sd), nil
}

var _ marshaler.GenericMarshaler = (*fakeGenericMarshaler)(nil)

type fakeResolver struct{}

func (r *fakeResolver) Resolve(host component.Host) (marshaler.GenericMarshaler, error) {
	return &fakeGenericMarshaler{}, nil
}

var _ marshaler.Resolver = (*fakeResolver)(nil)

func fakePick(genericMarshaler marshaler.GenericMarshaler) (marshaler.MarshalFunc[string], error) {
	return genericMarshaler.(*fakeGenericMarshaler).MarshalString, nil
}

var _ marshaler.PickFunc[string] = fakePick

func newNatsCoreExporterWithFakes(cfg *Config) *natsCoreExporter[string] {
	set := exportertest.NewNopSettings(metadata.Type)
	grouper := &fakeGrouper{}
	resolver := &fakeResolver{}
	marshaler := marshaler.NewMarshaler(resolver, fakePick)
	return &natsCoreExporter[string]{
		set:       set,
		cfg:       cfg,
		grouper:   grouper,
		marshaler: marshaler,
	}
}

func TestNatsCoreExporter(t *testing.T) {
	t.Parallel()

	withTestServer(t, func(cfg *Config, recorder *recorder) {
		exporter := newNatsCoreExporterWithFakes(cfg)

		err := exporter.start(t.Context(), componenttest.NewNopHost())
		assert.NoError(t, err)

		err = exporter.export(t.Context(), "test")
		assert.NoError(t, err)
		err = exporter.export(t.Context(), "test")
		assert.NoError(t, err)

		err = exporter.shutdown(t.Context())
		assert.NoError(t, err)

		msgs := recorder.record(2, 5*time.Second)
		for _, msg := range msgs {
			t.Logf("%s: %s", msg.Subject, string(msg.Data))
		}
	})
}

func TestNewNatsCoreLogsExporter(t *testing.T) {
	t.Parallel()

	withTestServer(t, func(cfg *Config, recorder *recorder) {
		set := exportertest.NewNopSettings(metadata.Type)
		exporter, err := newNatsCoreLogsExporter(set, cfg)
		assert.NoError(t, err)

		err = exporter.start(t.Context(), componenttest.NewNopHost())
		assert.NoError(t, err)

		err = exporter.export(t.Context(), generateLifecycleTestLogs())
		assert.NoError(t, err)
		err = exporter.export(t.Context(), generateLifecycleTestLogs())
		assert.NoError(t, err)

		err = exporter.shutdown(t.Context())
		assert.NoError(t, err)

		msgs := recorder.record(2, 5*time.Second)
		for _, msg := range msgs {
			t.Logf("%s: %s", msg.Subject, string(msg.Data))
		}
	})
}

func TestNewNatsCoreMetricsExporter(t *testing.T) {
	t.Parallel()

	withTestServer(t, func(cfg *Config, recorder *recorder) {
		set := exportertest.NewNopSettings(metadata.Type)
		exporter, err := newNatsCoreMetricsExporter(set, cfg)
		assert.NoError(t, err)

		err = exporter.start(t.Context(), componenttest.NewNopHost())
		assert.NoError(t, err)

		err = exporter.export(t.Context(), generateLifecycleTestMetrics())
		assert.NoError(t, err)
		err = exporter.export(t.Context(), generateLifecycleTestMetrics())
		assert.NoError(t, err)

		err = exporter.shutdown(t.Context())
		assert.NoError(t, err)

		msgs := recorder.record(2, 5*time.Second)
		for _, msg := range msgs {
			t.Logf("%s: %s", msg.Subject, string(msg.Data))
		}
	})
}

func TestNewNatsCoreTracesExporter(t *testing.T) {
	t.Parallel()

	withTestServer(t, func(cfg *Config, recorder *recorder) {
		set := exportertest.NewNopSettings(metadata.Type)
		exporter, err := newNatsCoreTracesExporter(set, cfg)
		assert.NoError(t, err)

		err = exporter.start(t.Context(), componenttest.NewNopHost())
		assert.NoError(t, err)

		err = exporter.export(t.Context(), generateLifecycleTestTraces())
		assert.NoError(t, err)
		err = exporter.export(t.Context(), generateLifecycleTestTraces())
		assert.NoError(t, err)

		err = exporter.shutdown(t.Context())
		assert.NoError(t, err)

		msgs := recorder.record(2, 5*time.Second)
		for _, msg := range msgs {
			t.Logf("%s: %s", msg.Subject, string(msg.Data))
		}
	})
}

func TestSetNatsNkeyOption(t *testing.T) {
	t.Parallel()

	withTestServer(t, func(cfg *Config, recorder *recorder) {
		exporter := newNatsCoreExporterWithFakes(cfg)

		err := exporter.start(t.Context(), componenttest.NewNopHost())
		assert.NoError(t, err)

		err = exporter.shutdown(t.Context())
		assert.NoError(t, err)
	})
}

func TestSetNatsNkeyJWTOption(t *testing.T) {
	t.Parallel()

	withTestServer(t, func(cfg *Config, recorder *recorder) {
		exporter := newNatsCoreExporterWithFakes(cfg)

		err := exporter.start(t.Context(), componenttest.NewNopHost())
		assert.NoError(t, err)

		err = exporter.shutdown(t.Context())
		assert.NoError(t, err)
	})
}

func TestSetNatsNkeyUserFileOption(t *testing.T) {
	t.Parallel()

	withTestServer(t, func(cfg *Config, recorder *recorder) {
		exporter := newNatsCoreExporterWithFakes(cfg)

		err := exporter.start(t.Context(), componenttest.NewNopHost())
		assert.NoError(t, err)

		err = exporter.shutdown(t.Context())
		assert.NoError(t, err)
	})
}
