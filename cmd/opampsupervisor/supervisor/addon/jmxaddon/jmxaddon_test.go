// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package jmxaddon

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor/addon"
)

// fakeJar is the minimum content the jmx type accepts as a jar.
var fakeJar = append([]byte("PK\x03\x04"), []byte("not really a jar")...)

// installFakeJava puts a stand-in JVM on JAVA_HOME. It records its arguments
// and the config file it was pointed at, then idles like a running scraper.
func installFakeJava(t *testing.T) (observedDir string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the fake JVM is a shell script")
	}

	home := t.TempDir()
	observedDir = t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(home, "bin"), 0o700))

	// The config file is the last argument: java -jar <jar> -config <file>.
	script := `#!/bin/sh
echo "$@" > "` + observedDir + `/args"
for last; do :; done
cp "$last" "` + observedDir + `/config"
while true; do sleep 0.05; done
`
	require.NoError(t, os.WriteFile(filepath.Join(home, "bin", "java"), []byte(script), 0o700)) //nolint:gosec // G306: the stand-in JVM must be executable
	t.Setenv("JAVA_HOME", home)
	return observedDir
}

// withoutJava removes every way of finding a JVM.
func withoutJava(t *testing.T) {
	t.Helper()
	t.Setenv("JAVA_HOME", "")
	t.Setenv("PATH", t.TempDir())
}

func newInstalled(t *testing.T) (addon.Addon, string) {
	t.Helper()
	dir := t.TempDir()
	f := NewFactory()
	id := addon.ID{Type: Type, Name: "tomcat"}
	require.NoError(t, f.Install(t.Context(), addon.InstallSettings{
		ID: id, Dir: dir, Content: fakeJar,
		DownloadURL: "https://example.invalid/opentelemetry-jmx-scraper.jar",
		Logger:      zap.NewNop(),
	}))
	a, err := f.Create(t.Context(), addon.Settings{ID: id, Dir: dir, Logger: zap.NewNop()})
	require.NoError(t, err)
	t.Cleanup(func() { _ = a.Shutdown(t.Context()) })
	return a, dir
}

type running interface {
	IsRunning() bool
	Pid() int
}

func eventuallyFileContains(t *testing.T, path, want string) {
	t.Helper()
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		body, err := os.ReadFile(path)
		if !assert.NoError(c, err) {
			return
		}
		assert.Contains(c, string(body), want)
	}, 10*time.Second, 25*time.Millisecond)
}

// TestInstallWithoutJavaFails is the runtime boundary: on a host without a JVM
// the install fails with a reason, which the syncer reports as INSTALL_FAILED,
// instead of succeeding and never running.
func TestInstallWithoutJavaFails(t *testing.T) {
	withoutJava(t)

	err := NewFactory().Install(t.Context(), addon.InstallSettings{
		ID: addon.ID{Type: Type, Name: "tomcat"}, Dir: t.TempDir(), Content: fakeJar, Logger: zap.NewNop(),
	})
	require.ErrorIs(t, err, addon.ErrRuntimeNotFound)
	require.ErrorContains(t, err, "JAVA_HOME")
}

func TestInstallRejectsNonJar(t *testing.T) {
	installFakeJava(t)

	err := NewFactory().Install(t.Context(), addon.InstallSettings{
		ID: addon.ID{Type: Type, Name: "tomcat"}, Dir: t.TempDir(), Content: []byte("#!/bin/sh\nrm -rf /\n"), Logger: zap.NewNop(),
	})
	require.ErrorContains(t, err, "must be the opentelemetry-jmx-scraper jar")
}

// TestJMXAddonLifecycle covers the scraper type end to end with a stand-in JVM:
// idle until configured, launched with the fixed command line, restarted on a
// config change, and stopped when its configuration is withdrawn.
func TestJMXAddonLifecycle(t *testing.T) {
	observed := installFakeJava(t)
	a, dir := newInstalled(t)
	proc := a.(running)
	ctx := t.Context()

	// Installed but not configured: nothing runs.
	_, err := a.Configure(ctx, nil)
	require.NoError(t, err)
	_, err = a.Start(ctx)
	require.NoError(t, err)
	assert.False(t, proc.IsRunning(), "an unconfigured scraper must not start")

	// Configured and started: the type, not the package, decides the command.
	_, err = a.Configure(ctx, []byte("otel.jmx.target.system=tomcat\n"))
	require.NoError(t, err)
	effect, err := a.Start(ctx)
	require.NoError(t, err)
	assert.Equal(t, addon.Effect{}, effect, "the scraper is its own process and never needs a Collector restart")
	require.True(t, proc.IsRunning())

	eventuallyFileContains(t, filepath.Join(observed, "config"), "otel.jmx.target.system=tomcat")
	args, err := os.ReadFile(filepath.Join(observed, "args"))
	require.NoError(t, err)
	assert.Equal(t,
		"-jar "+filepath.Join(dir, jarFile)+" -config "+filepath.Join(dir, configFile),
		strings.TrimSpace(string(args)))

	// A second Start is a no-op.
	firstPid := proc.Pid()
	_, err = a.Start(ctx)
	require.NoError(t, err)
	assert.Equal(t, firstPid, proc.Pid())

	// A config change restarts the scraper so it rereads its properties.
	require.NoError(t, os.Remove(filepath.Join(observed, "config")))
	_, err = a.Configure(ctx, []byte("otel.jmx.target.system=kafka\n"))
	require.NoError(t, err)
	require.True(t, proc.IsRunning())
	assert.NotEqual(t, firstPid, proc.Pid())
	eventuallyFileContains(t, filepath.Join(observed, "config"), "otel.jmx.target.system=kafka")

	// Withdrawing the configuration stops it and removes the file.
	_, err = a.Configure(ctx, nil)
	require.NoError(t, err)
	assert.False(t, proc.IsRunning())
	assert.NoFileExists(t, filepath.Join(dir, configFile))

	// Shutdown leaves the jar in place.
	require.NoError(t, a.Shutdown(ctx))
	assert.FileExists(t, filepath.Join(dir, jarFile))
}

// TestStartAfterJavaRemoved checks that a JVM removed after install is reported
// at start rather than leaving the addon half-started.
func TestStartAfterJavaRemoved(t *testing.T) {
	installFakeJava(t)
	a, _ := newInstalled(t)
	ctx := t.Context()

	_, err := a.Configure(ctx, []byte("otel.jmx.target.system=tomcat\n"))
	require.NoError(t, err)

	withoutJava(t)
	_, err = a.Start(ctx)
	require.ErrorIs(t, err, addon.ErrRuntimeNotFound)
	assert.False(t, a.(running).IsRunning())
}
