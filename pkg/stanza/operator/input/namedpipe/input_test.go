// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package namedpipe

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"golang.org/x/sys/unix"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/pipeline"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/testutil"
)

// filename attempts to get an unused filename.
func filename(tb testing.TB) string {
	tb.Helper()

	file, err := os.CreateTemp(tb.TempDir(), "")
	require.NoError(tb, err)

	name := file.Name()
	require.NoError(tb, file.Close())
	require.NoError(tb, os.Remove(name))

	return name
}

// TestCreatePipe tests that the named pipe is created as a pipe with the correct permissions.
func TestCreatePipe(t *testing.T) {
	conf := NewConfig()
	conf.Path = filename(t)
	conf.Permissions = 0o666

	set := componenttest.NewNopTelemetrySettings()
	op, err := conf.Build(set)
	require.NoError(t, err)

	require.NoError(t, op.Start(testutil.NewUnscopedMockPersister()))
	defer func() {
		require.NoError(t, op.Stop())
	}()

	stat, err := os.Stat(conf.Path)
	require.NoError(t, err)

	isPipe := stat.Mode()&os.ModeNamedPipe != 0
	require.True(t, isPipe, "file is not a named pipe")
	require.Equal(t, conf.Permissions, uint32(stat.Mode().Perm()))
}

// TestCreatePipeThatExists tests that the input errors if the named pipe already exists as a file.
func TestCreatePipeFailsWithFile(t *testing.T) {
	conf := NewConfig()
	conf.Path = filename(t)
	conf.Permissions = 0o666

	pipe, err := os.OpenFile(conf.Path, os.O_CREATE, 0)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, pipe.Close())
	}()

	set := componenttest.NewNopTelemetrySettings()
	op, err := conf.Build(set)
	require.NoError(t, err)

	require.Error(t, op.Start(testutil.NewUnscopedMockPersister()))
}

// TestCreatePipeAlreadyExists tests that the input works if the file already exists as a pipe.
func TestCreatePipeAlreadyExists(t *testing.T) {
	conf := NewConfig()
	conf.Path = filename(t)
	conf.Permissions = 0o666

	require.NoError(t, unix.Mkfifo(conf.Path, conf.Permissions))

	set := componenttest.NewNopTelemetrySettings()
	op, err := conf.Build(set)
	require.NoError(t, err)

	require.NoError(t, op.Start(testutil.NewUnscopedMockPersister()))
	require.NoError(t, op.Stop())
}

// TestPipeWrites writes a few logs to the pipe over a few different connections and verifies that they are received.
func TestPipeWrites(t *testing.T) {
	fake := testutil.NewFakeOutput(t)

	conf := NewConfig()
	conf.Path = filename(t)
	conf.Permissions = 0o666
	conf.OutputIDs = []string{fake.ID()}

	set := componenttest.NewNopTelemetrySettings()
	op, err := conf.Build(set)
	require.NoError(t, err)
	ops := []operator.Operator{op, fake}

	p, err := pipeline.NewDirectedPipeline(ops)
	require.NoError(t, err)

	require.NoError(t, p.Start(testutil.NewUnscopedMockPersister()))
	defer func() {
		require.NoError(t, p.Stop())
	}()

	logs := [][]string{
		{"log1\n", "log2\n"},
		{"log3\n", "log4\n"},
		{"log5\n"},
	}

	for _, toSend := range logs {
		pipe, err := os.OpenFile(conf.Path, os.O_WRONLY, 0)
		require.NoError(t, err)
		defer func() {
			require.NoError(t, pipe.Close())
		}()

		for _, log := range toSend {
			_, err = pipe.WriteString(log)
			require.NoError(t, err)
		}

		for _, log := range toSend {
			expect := &entry.Entry{
				Body: strings.TrimSpace(log),
			}

			select {
			case e := <-fake.Received:
				obs := time.Now()
				expect.ObservedTimestamp = obs
				e.ObservedTimestamp = obs
				require.Equal(t, expect, e)
			case <-time.After(time.Second):
				t.Fatal("timed out waiting for entry")
			}
		}
	}
}

// TestPipeHasDataAtStartup will test if the receiver can consume from a named
// pipe that has buffered data before startup.
func TestPipeHasDataAtStartup(t *testing.T) {
	fake := testutil.NewFakeOutput(t)

	conf := NewConfig()
	conf.Path = filename(t)
	conf.Permissions = 0o666
	conf.OutputIDs = []string{fake.ID()}

	set := componenttest.NewNopTelemetrySettings()
	op, err := conf.Build(set)
	require.NoError(t, err)
	ops := []operator.Operator{op, fake}

	// create pipe
	require.NoError(t, unix.Mkfifo(conf.Path, conf.Permissions))

	pipe, err := os.OpenFile(conf.Path, os.O_RDWR|os.O_APPEND, os.ModeNamedPipe)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, pipe.Close())
	}()

	logs := []string{"log1\n", "log2\n"}

	for _, log := range logs {
		_, err = pipe.WriteString(log)
		require.NoError(t, err)
	}

	p, err := pipeline.NewDirectedPipeline(ops)
	require.NoError(t, err)

	// start receiver
	require.NoError(t, p.Start(testutil.NewUnscopedMockPersister()))
	defer func() {
		require.NoError(t, p.Stop())
	}()

	for _, log := range logs {
		expect := &entry.Entry{
			Body: strings.TrimSpace(log),
		}

		select {
		case e := <-fake.Received:
			obs := time.Now()
			expect.ObservedTimestamp = obs
			e.ObservedTimestamp = obs
			require.Equal(t, expect, e)
		case <-time.After(time.Second):
			t.Fatal("timed-out waiting for log entry")
		}
	}
}
