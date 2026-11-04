// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package addon

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// recordingFactory is a test addon type that records every call made to its
// addons.
type recordingFactory struct {
	typ        Type
	installErr error
	// startEffect is returned by Start the first time it is called on an addon.
	startEffect Effect
	addons      map[string]*recordingAddon
}

func newRecordingFactory(typ Type) *recordingFactory {
	return &recordingFactory{typ: typ, addons: map[string]*recordingAddon{}}
}

func (f *recordingFactory) Type() Type { return f.typ }

func (f *recordingFactory) Install(context.Context, InstallSettings) error { return f.installErr }

func (f *recordingFactory) Create(_ context.Context, set Settings) (Addon, error) {
	a := &recordingAddon{startEffect: f.startEffect}
	f.addons[set.ID.String()] = a
	return a, nil
}

type recordingAddon struct {
	configs      [][]byte
	starts       int
	shutdowns    int
	started      bool
	startEffect  Effect
	configureErr error
}

func (a *recordingAddon) Configure(_ context.Context, body []byte) (Effect, error) {
	if a.configureErr != nil {
		return Effect{}, a.configureErr
	}
	a.configs = append(a.configs, body)
	return Effect{}, nil
}

func (a *recordingAddon) Start(context.Context) (Effect, error) {
	a.starts++
	if a.started {
		return Effect{}, nil
	}
	a.started = true
	return a.startEffect, nil
}

func (a *recordingAddon) Shutdown(context.Context) error {
	a.shutdowns++
	a.started = false
	return nil
}

func TestNewManagerRejectsDuplicateTypes(t *testing.T) {
	_, err := NewManager(zap.NewNop(), newRecordingFactory("file"), newRecordingFactory("file"))
	require.ErrorContains(t, err, `addon type "file" is registered more than once`)
}

func TestNewManagerRejectsInvalidType(t *testing.T) {
	_, err := NewManager(zap.NewNop(), newRecordingFactory("bad-type"))
	require.ErrorContains(t, err, "invalid addon type")
}

func TestManagerSupports(t *testing.T) {
	m, err := NewManager(zap.NewNop(), newRecordingFactory("file"), newRecordingFactory("jmx"))
	require.NoError(t, err)

	require.NoError(t, m.Supports("file:geoip"))
	require.NoError(t, m.Supports("jmx:tomcat"))

	err = m.Supports("custom_rules:base")
	require.ErrorContains(t, err, `unsupported addon type "custom_rules"`)
	require.ErrorContains(t, err, "[file jmx]", "the error should list the supported types")

	require.ErrorContains(t, m.Supports("no-type"), `must have the form "<type>:<name>"`)
}

// TestManagerReconcile covers the reconciliation contract addon types rely on:
// configure only on change, nil when configuration is withdrawn, start on
// every pass, and effects merged across addons.
func TestManagerReconcile(t *testing.T) {
	plain := newRecordingFactory("file")
	restarting := newRecordingFactory("restarting")
	restarting.startEffect = Effect{RestartAgent: true}

	m, err := NewManager(zap.NewNop(), plain, restarting)
	require.NoError(t, err)

	ctx := t.Context()
	require.NoError(t, m.Load(ctx, "file:a", t.TempDir()))
	require.NoError(t, m.Load(ctx, "restarting:b", t.TempDir()))
	assert.Equal(t, []string{"file:a", "restarting:b"}, m.Names())

	a := plain.addons["file:a"]
	b := restarting.addons["restarting:b"]

	// First pass: both are configured, even with no configuration, and started.
	// b's first start asks for a Collector restart.
	effect, err := m.Reconcile(ctx, map[string][]byte{"restarting:b": []byte("v1")})
	require.NoError(t, err)
	assert.True(t, effect.RestartAgent)
	assert.Equal(t, [][]byte{nil}, a.configs)
	assert.Equal(t, [][]byte{[]byte("v1")}, b.configs)
	assert.Equal(t, 1, a.starts)

	// Unchanged offer: no Configure, Start is called again but changes nothing.
	effect, err = m.Reconcile(ctx, map[string][]byte{"restarting:b": []byte("v1")})
	require.NoError(t, err)
	assert.False(t, effect.RestartAgent, "an unchanged offer must not restart the Collector")
	assert.Len(t, b.configs, 1)
	assert.Equal(t, 2, b.starts)

	// Changed configuration is applied.
	_, err = m.Reconcile(ctx, map[string][]byte{"restarting:b": []byte("v2")})
	require.NoError(t, err)
	assert.Equal(t, []byte("v2"), b.configs[len(b.configs)-1])

	// Withdrawn configuration is applied as nil.
	_, err = m.Reconcile(ctx, map[string][]byte{})
	require.NoError(t, err)
	assert.Nil(t, b.configs[len(b.configs)-1])

	// Configuration for an addon that is not installed is not an error.
	_, err = m.Reconcile(ctx, map[string][]byte{"file:not-installed": []byte("x")})
	require.NoError(t, err)
}

// TestManagerReconcileIsolatesFailures checks that one addon failing to
// configure does not stop the others from running.
func TestManagerReconcileIsolatesFailures(t *testing.T) {
	f := newRecordingFactory("file")
	m, err := NewManager(zap.NewNop(), f)
	require.NoError(t, err)

	ctx := t.Context()
	require.NoError(t, m.Load(ctx, "file:broken", t.TempDir()))
	require.NoError(t, m.Load(ctx, "file:healthy", t.TempDir()))
	f.addons["file:broken"].configureErr = errors.New("boom")

	_, err = m.Reconcile(ctx, nil)
	require.ErrorContains(t, err, `configure addon "file:broken": boom`)
	assert.True(t, f.addons["file:healthy"].started)
	assert.False(t, f.addons["file:broken"].started, "an addon that failed to configure must not be started")

	// The failed addon is retried on the next pass.
	f.addons["file:broken"].configureErr = nil
	_, err = m.Reconcile(ctx, nil)
	require.NoError(t, err)
	assert.True(t, f.addons["file:broken"].started)
}

func TestManagerLoadReplacesAndUnloadShutsDown(t *testing.T) {
	f := newRecordingFactory("file")
	m, err := NewManager(zap.NewNop(), f)
	require.NoError(t, err)

	ctx := t.Context()
	require.NoError(t, m.Load(ctx, "file:a", t.TempDir()))
	first := f.addons["file:a"]
	_, err = m.Reconcile(ctx, nil)
	require.NoError(t, err)

	// Loading again, as an upgrade does, shuts the previous addon down and
	// configures the new one from scratch.
	require.NoError(t, m.Load(ctx, "file:a", t.TempDir()))
	second := f.addons["file:a"]
	assert.Equal(t, 1, first.shutdowns)
	assert.NotSame(t, first, second)

	_, err = m.Reconcile(ctx, nil)
	require.NoError(t, err)
	assert.Len(t, second.configs, 1, "a reloaded addon must be configured again")

	require.NoError(t, m.Unload(ctx, "file:a"))
	assert.Equal(t, 1, second.shutdowns)
	assert.Empty(t, m.Names())

	// Unloading an unknown addon is a no-op.
	require.NoError(t, m.Unload(ctx, "file:a"))
}

func TestManagerInstallDispatchesByType(t *testing.T) {
	f := newRecordingFactory("file")
	f.installErr = errors.New("install failed")
	m, err := NewManager(zap.NewNop(), f)
	require.NoError(t, err)

	err = m.Install(t.Context(), "file:a", t.TempDir(), nil, "https://example.invalid/a", nil)
	require.ErrorContains(t, err, "install failed")

	err = m.Install(t.Context(), "jmx:a", t.TempDir(), nil, "https://example.invalid/a", nil)
	require.ErrorContains(t, err, `unsupported addon type "jmx"`)
}
