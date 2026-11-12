// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package supervisor

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// jmxScraperJarEnv names an environment variable holding the path to a
// downloaded opentelemetry-jmx-scraper.jar. The jar is ~6 MiB, so it is not
// committed; download it from
// https://github.com/open-telemetry/opentelemetry-java-contrib/releases/latest/download/opentelemetry-jmx-scraper.jar
// and point this variable at it to run the test.
const jmxScraperJarEnv = "OPAMP_JMX_SCRAPER_JAR"

// TestJMXScraperAddonEndToEnd is the anchor use case for addon packages, run
// for real: the Supervisor installs the JMX Metric Scraper as an addon
// package, configures it from remote config, runs it as a managed child
// process, and the scraper connects to a JVM and exports OTLP to a local
// endpoint. This is the deployment story that disappeared when jmxreceiver was
// removed in v0.157.0.
func TestJMXScraperAddonEndToEnd(t *testing.T) {
	jarPath := os.Getenv(jmxScraperJarEnv)
	if jarPath == "" {
		t.Skipf("set %s to the path of opentelemetry-jmx-scraper.jar to run this test", jmxScraperJarEnv)
	}
	jar, err := os.ReadFile(jarPath)
	require.NoError(t, err, "reading the scraper jar named by %s", jmxScraperJarEnv)

	javaPath, err := exec.LookPath("java")
	if err != nil {
		t.Skip("java is not installed; executable addons require their runtime on the host")
	}
	t.Logf("using java at %s", javaPath)

	// Capture the OTLP the scraper exports. This stands in for the Collector's
	// OTLP receiver: the addon ships data to a local endpoint, exactly as the
	// jmxreceiver's bundled gatherer did.
	var metricRequests atomic.Int64
	otlp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/metrics" {
			metricRequests.Add(1)
		}
		w.Header().Set("Content-Type", "application/x-protobuf")
		w.WriteHeader(http.StatusOK)
	}))
	defer otlp.Close()

	jmxPort := freePort(t)
	target := startJMXTargetJVM(t, jmxPort)
	defer target()

	s := newAddonTestSupervisor(t)
	ctx := t.Context()

	// The package is the upstream release asset exactly as published: the jmx
	// type knows how to run it, so there is no wrapper and no manifest.
	const name = "jmx:tomcat"
	require.NoError(t, installPackage(s.packages, name, jar,
		"https://github.com/open-telemetry/opentelemetry-java-contrib/releases/download/v1.61.0/opentelemetry-jmx-scraper.jar",
		"1.61.0"))

	installedAddon, dir, ok := s.addons.Get(name)
	require.True(t, ok)
	installed := installedAddon.(runningAddon)

	// Configuration is delivered as a remote config entry keyed by the same
	// package name, whose role marks it as addon config so it never reaches
	// the Collector's YAML.
	scraperConfig := fmt.Sprintf(`otel.jmx.service.url=service:jmx:rmi:///jndi/rmi://localhost:%d/jmxrmi
otel.jmx.target.system=jvm
otel.metrics.exporter=otlp
otel.exporter.otlp.protocol=http/protobuf
otel.exporter.otlp.endpoint=%s
otel.metric.export.interval=2000
`, jmxPort, otlp.URL)

	setAddonConfigs(s, map[string]string{name: scraperConfig})
	assert.False(t, s.reconcileAddons(ctx))
	require.True(t, installed.IsRunning())
	t.Logf("scraper addon running with pid %d", installed.Pid())

	logPath := filepath.Join(dir, "addon.log")

	// The scraper read the configuration the Supervisor delivered.
	requireEventuallyFileContains(t, logPath, "JMX scraping started")
	requireEventuallyFileContains(t, logPath, strconv.Itoa(jmxPort))

	// And it exported metrics to the local endpoint.
	require.Eventually(t, func() bool {
		return metricRequests.Load() > 0
	}, 90*time.Second, 250*time.Millisecond, "expected the scraper to export OTLP metrics")

	t.Logf("received %d OTLP metrics requests from the supervised addon", metricRequests.Load())

	// Stopping the addon leaves the package installed. This is the mechanism a
	// remotely requested disabled state would drive, and the reason disable
	// cannot be expressed as removal from packages_available.
	require.NoError(t, installedAddon.Shutdown(ctx))
	assert.False(t, installed.IsRunning())
	assert.FileExists(t, filepath.Join(dir, "opentelemetry-jmx-scraper.jar"),
		"stopping an addon must not remove its files")

	exported := metricRequests.Load()
	time.Sleep(3 * time.Second)
	assert.Equal(t, exported, metricRequests.Load(), "a stopped addon must not keep exporting")

	// Starting it again resumes without re-downloading anything.
	_, err = installedAddon.Start(ctx)
	require.NoError(t, err)
	assert.True(t, installed.IsRunning())
	require.Eventually(t, func() bool {
		return metricRequests.Load() > exported
	}, 90*time.Second, 250*time.Millisecond, "expected exports to resume after restart")
}

// startJMXTargetJVM starts a JVM with the JMX remote agent enabled on port and
// returns a function that stops it. It is the application an operator wants to
// monitor.
func startJMXTargetJVM(t *testing.T, port int) func() {
	t.Helper()

	dir := t.TempDir()
	source := filepath.Join(dir, "Target.java")
	require.NoError(t, os.WriteFile(source, []byte(`
public class Target {
    public static void main(String[] args) throws Exception {
        Thread.sleep(Long.MAX_VALUE);
    }
}
`), 0o600))

	logFile, err := os.Create(filepath.Join(dir, "target.log"))
	require.NoError(t, err)

	cmd := exec.Command("java", //nolint:gosec // G204: fixed JVM flags; the only variable is a local port
		"-Dcom.sun.management.jmxremote",
		"-Dcom.sun.management.jmxremote.port="+strconv.Itoa(port),
		"-Dcom.sun.management.jmxremote.rmi.port="+strconv.Itoa(port),
		"-Dcom.sun.management.jmxremote.authenticate=false",
		"-Dcom.sun.management.jmxremote.ssl=false",
		"-Dcom.sun.management.jmxremote.local.only=false",
		"-Djava.rmi.server.hostname=localhost",
		source,
	)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	require.NoError(t, cmd.Start())

	// Wait for the JMX port to accept connections before the scraper tries it.
	require.Eventually(t, func() bool {
		conn, err := net.DialTimeout("tcp", net.JoinHostPort("localhost", strconv.Itoa(port)), time.Second)
		if err != nil {
			return false
		}
		conn.Close()
		return true
	}, 90*time.Second, 250*time.Millisecond, "JMX target JVM did not open port %d", port)

	t.Logf("JMX target JVM listening on %d with pid %d", port, cmd.Process.Pid)

	return func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
		logFile.Close()
	}
}

// freePort returns a TCP port that was free at the time of the call.
func freePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}
