// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package natsexporter

import (
	"testing"
	"time"

	natstest "github.com/nats-io/nats-server/v2/test"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/testdata"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/natsexporter/internal/metadata"
)

// runServer starts an embedded nats-server on a random port for a test.
func runServer(t *testing.T) string {
	t.Helper()
	opts := natstest.DefaultTestOptions
	opts.Port = -1 // random free port
	srv := natstest.RunServer(&opts)
	t.Cleanup(srv.Shutdown)
	return srv.ClientURL()
}

// TestExporter_CoreNATS verifies the core-NATS path publishes each exported
// payload to the configured subject.
func TestExporter_CoreNATS(t *testing.T) {
	t.Parallel()

	url := runServer(t)
	ctx := t.Context()

	nc, err := nats.Connect(url)
	require.NoError(t, err)
	defer nc.Close()

	sub, err := nc.SubscribeSync("otel_logs")
	require.NoError(t, err)
	require.NoError(t, nc.Flush())

	cfg := createDefaultConfig().(*Config)
	cfg.Endpoint = url

	set := exportertest.NewNopSettings(metadata.Type)
	exp := newExporter(set, cfg)

	require.NoError(t, exp.Start(ctx, componenttest.NewNopHost()))
	require.NoError(t, exp.pushLogs(ctx, testdata.GenerateLogs(1)))

	msg, err := sub.NextMsg(5 * time.Second)
	require.NoError(t, err)
	assert.Equal(t, "otel_logs", msg.Subject)
	assert.NotEmpty(t, msg.Data)

	require.NoError(t, exp.Shutdown(ctx))
}

// TestExporter_PermanentError verifies that a deterministic subject-evaluation
// failure is reported as a permanent (non-retryable) error rather than one that
// would be retried forever.
func TestExporter_PermanentError(t *testing.T) {
	t.Parallel()

	url := runServer(t)
	ctx := t.Context()

	cfg := createDefaultConfig().(*Config)
	cfg.Endpoint = url
	// Valid OTTL that evaluates to an int, so the subject is never a string.
	cfg.Logs.Subject = "123"

	set := exportertest.NewNopSettings(metadata.Type)
	exp := newExporter(set, cfg)

	require.NoError(t, exp.Start(ctx, componenttest.NewNopHost()))
	t.Cleanup(func() { _ = exp.Shutdown(ctx) })

	err := exp.pushLogs(ctx, testdata.GenerateLogs(1))
	require.Error(t, err)
	assert.True(t, consumererror.IsPermanent(err), "a non-string subject should be a permanent error")
}

// TestExporter_InvalidSubjectIsPermanent verifies that a subject expression that
// evaluates to a valid string which NATS nonetheless rejects as a subject (here,
// empty) is reported as a permanent error rather than retried forever.
func TestExporter_InvalidSubjectIsPermanent(t *testing.T) {
	t.Parallel()

	url := runServer(t)
	ctx := t.Context()

	cfg := createDefaultConfig().(*Config)
	cfg.Endpoint = url
	// Valid OTTL that yields an empty string; NATS rejects an empty subject.
	cfg.Logs.Subject = `""`

	set := exportertest.NewNopSettings(metadata.Type)
	exp := newExporter(set, cfg)

	require.NoError(t, exp.Start(ctx, componenttest.NewNopHost()))
	t.Cleanup(func() { _ = exp.Shutdown(ctx) })

	err := exp.pushLogs(ctx, testdata.GenerateLogs(1))
	require.Error(t, err)
	assert.True(t, consumererror.IsPermanent(err), "an invalid NATS subject should be a permanent error")
}
