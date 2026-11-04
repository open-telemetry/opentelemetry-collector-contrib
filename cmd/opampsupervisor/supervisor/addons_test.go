// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package supervisor

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/open-telemetry/opamp-go/client/types"
	"github.com/open-telemetry/opamp-go/protobufs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor/addon"
	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor/config"
)

// newAddonTestSupervisor returns a Supervisor with package storage and the addon
// manager wired up, as NewSupervisor does when accepts_packages is enabled.
// extra are registered the way a distribution registers its own addon types.
func newAddonTestSupervisor(t *testing.T, extra ...addon.Factory) *Supervisor {
	t.Helper()

	cfg := config.DefaultSupervisor()
	cfg.Storage.Directory = t.TempDir()
	cfg.Capabilities.AcceptsPackages = true
	cfg.Capabilities.AcceptsRemoteConfig = true

	return newAddonTestSupervisorWithConfig(t, cfg, extra...)
}

func newAddonTestSupervisorWithConfig(t *testing.T, cfg config.Supervisor, extra ...addon.Factory) *Supervisor {
	t.Helper()

	s := &Supervisor{
		config:            cfg,
		telemetrySettings: newNopTelemetrySettings(),
		persistentState: &persistentState{
			InstanceID: uuid.New(),
			configPath: filepath.Join(cfg.Storage.Directory, persistentStateFileName),
		},
		hasNewConfig:    make(chan struct{}, 1),
		effectiveConfig: &atomic.Value{},
		cfgState:        &atomic.Value{},
	}
	s.runCtx, s.runCtxCancel = context.WithCancel(t.Context())
	t.Cleanup(s.runCtxCancel)
	recordReports(s)

	var err error
	s.addons, err = addon.NewManager(zap.NewNop(), append(builtinAddonFactories(), extra...)...)
	require.NoError(t, err)
	s.packages, err = newPackageManager(zap.NewNop(), filepath.Join(cfg.Storage.Directory, packagesDirName), s.addons)
	require.NoError(t, err)

	t.Cleanup(func() { s.stopAddons(t.Context()) })
	return s
}

// reports records what the Supervisor tells the OpAMP server.
type reports struct {
	mu               sync.Mutex
	statuses         []*protobufs.RemoteConfigStatus
	effectiveUpdates int
}

// last returns the most recently reported remote config status.
func (r *reports) last() *protobufs.RemoteConfigStatus {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.statuses) == 0 {
		return nil
	}
	return r.statuses[len(r.statuses)-1]
}

func (r *reports) updates() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.effectiveUpdates
}

// recordReports gives s an OpAMP client that records what it reports.
func recordReports(s *Supervisor) *reports {
	r := &reports{}
	s.opampClient = &mockOpAMPClient{
		setRemoteConfigStatusFunc: func(rcs *protobufs.RemoteConfigStatus) error {
			r.mu.Lock()
			defer r.mu.Unlock()
			r.statuses = append(r.statuses, rcs)
			return nil
		},
		updateEffectiveConfigFunc: func(context.Context) error {
			r.mu.Lock()
			defer r.mu.Unlock()
			r.effectiveUpdates++
			return nil
		},
		setHealthFunc: func(*protobufs.ComponentHealth) {},
	}
	return r
}

// installPackage drives the packageManager through the call sequence the
// opamp-go PackagesSyncer uses for an addon package, so addons are exercised the
// way the real client drives them. It returns the error the syncer would report
// as INSTALL_FAILED.
func installPackage(p *packageManager, name string, content []byte, downloadURL, version string) error {
	const typ = protobufs.PackageType_PackageType_Addon

	state, err := p.PackageState(name)
	if err != nil {
		return err
	}
	if !state.Exists {
		if createErr := p.CreatePackage(name, typ); createErr != nil {
			return createErr
		}
	}

	sum := sha256.Sum256(content)
	existing, err := p.FileContentHash(name)
	if err != nil {
		return err
	}
	if !bytes.Equal(existing, sum[:]) {
		if updateErr := p.UpdateContent(context.Background(), name, downloadURL, bytes.NewReader(content), sum[:], nil); updateErr != nil {
			return updateErr
		}
	}

	return p.SetPackageState(name, types.PackageState{
		Exists: true, Type: typ, Hash: []byte("package-hash-" + version), Version: version,
	})
}

// setAddonConfigs stores a remote config carrying only addon configuration, as
// processRemoteConfigMessage does.
func setAddonConfigs(s *Supervisor, configs map[string]string) {
	s.remoteConfig.Store(addonRemoteConfig(configs))
}

func addonRemoteConfig(configs map[string]string) *protobufs.AgentRemoteConfig {
	configMap := map[string]*protobufs.AgentConfigObject{}
	names := make([]string, 0, len(configs))
	for name, body := range configs {
		configMap[name] = &protobufs.AgentConfigObject{Body: []byte(body), ContentType: "text/plain", Role: addonConfigRole}
		names = append(names, name)
	}
	// A deterministic hash, so that distinct offers are distinct configs the
	// way a server would hash them.
	sort.Strings(names)
	h := sha256.New()
	for _, name := range names {
		h.Write([]byte(name + "=" + configs[name] + "\n"))
	}
	return &protobufs.AgentRemoteConfig{Config: &protobufs.AgentConfigMap{ConfigMap: configMap}, ConfigHash: h.Sum(nil)}
}

func tarGz(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, body := range files {
		require.NoError(t, tw.WriteHeader(&tar.Header{Name: name, Mode: 0o600, Size: int64(len(body)), Typeflag: tar.TypeReg}))
		_, err := tw.Write([]byte(body))
		require.NoError(t, err)
	}
	require.NoError(t, tw.Close())
	require.NoError(t, gz.Close())
	return buf.Bytes()
}

// restartSignals returns how many Collector restarts are pending. The channel
// coalesces, so this is 0 or 1.
func restartSignals(s *Supervisor) int {
	return len(s.hasNewConfig)
}

func requireEventuallyFileContains(t *testing.T, path, want string) {
	t.Helper()
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		body, err := os.ReadFile(path)
		if !assert.NoError(c, err) {
			return
		}
		assert.Contains(c, string(body), want)
	}, 10*time.Second, 25*time.Millisecond, "waiting for %s to contain %q", path, want)
}

type runningAddon interface {
	IsRunning() bool
	Pid() int
}

// --- jmx: the anchor use case ---

// fakeJar is the minimum content the jmx type accepts as a jar.
func fakeJar(version string) []byte {
	return append([]byte("PK\x03\x04"), []byte("scraper "+version)...)
}

// installFakeJava puts a stand-in JVM on JAVA_HOME that records the config it
// was given and idles like a running scraper.
func installFakeJava(t *testing.T) (observedConfig string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the fake JVM is a shell script")
	}
	home := t.TempDir()
	observed := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(home, "bin"), 0o700))
	script := `#!/bin/sh
for last; do :; done
cp "$last" "` + observed + `/config"
while true; do sleep 0.05; done
`
	require.NoError(t, os.WriteFile(filepath.Join(home, "bin", "java"), []byte(script), 0o700)) //nolint:gosec // G306: the stand-in JVM must be executable
	t.Setenv("JAVA_HOME", home)
	return filepath.Join(observed, "config")
}

// TestJMXAddonThroughSupervisor walks the scraper through the Supervisor:
// offered as "jmx:tomcat" with the jar as-is, configured by a remote config
// entry under the same key, run, reconfigured, and deleted.
func TestJMXAddonThroughSupervisor(t *testing.T) {
	observed := installFakeJava(t)
	s := newAddonTestSupervisor(t)
	ctx := t.Context()

	require.NoError(t, installPackage(s.packages, "jmx:tomcat", fakeJar("1.61.0"), "https://github.com/open-telemetry/opentelemetry-java-contrib/releases/download/v1.61.0/opentelemetry-jmx-scraper.jar", "1.61.0"))

	setAddonConfigs(s, map[string]string{"jmx:tomcat": "otel.jmx.target.system=tomcat\n"})
	assert.False(t, s.reconcileAddons(ctx), "a scraper never needs a Collector restart")

	a, dir, ok := s.addons.Get("jmx:tomcat")
	require.True(t, ok)
	scraper := a.(runningAddon)
	require.True(t, scraper.IsRunning())
	requireEventuallyFileContains(t, observed, "otel.jmx.target.system=tomcat")
	firstPid := scraper.Pid()

	// An unchanged offer does not touch the running scraper.
	s.reconcileAddons(ctx)
	assert.Equal(t, firstPid, scraper.Pid())

	// A changed one restarts it.
	setAddonConfigs(s, map[string]string{"jmx:tomcat": "otel.jmx.target.system=kafka\n"})
	s.reconcileAddons(ctx)
	assert.NotEqual(t, firstPid, scraper.Pid())
	requireEventuallyFileContains(t, observed, "otel.jmx.target.system=kafka")
	assert.Zero(t, restartSignals(s))

	// Deleting the package stops the scraper and removes it.
	require.NoError(t, s.packages.DeletePackage("jmx:tomcat"))
	assert.False(t, scraper.IsRunning())
	assert.NoDirExists(t, dir)
	assert.Empty(t, s.addons.Names())
}

// TestJMXAddonWithoutJavaReportsInstallFailure checks that the runtime boundary
// surfaces where the server can see it.
func TestJMXAddonWithoutJavaReportsInstallFailure(t *testing.T) {
	t.Setenv("JAVA_HOME", "")
	t.Setenv("PATH", t.TempDir())
	s := newAddonTestSupervisor(t)

	err := installPackage(s.packages, "jmx:tomcat", fakeJar("1"), "https://example.invalid/opentelemetry-jmx-scraper.jar", "1")
	require.ErrorIs(t, err, addon.ErrRuntimeNotFound)
	assert.Empty(t, s.addons.Names())
	assert.NoDirExists(t, s.packages.AddonDir("jmx:tomcat"))
}

func TestJMXAddonUpgradeReplacesProcess(t *testing.T) {
	installFakeJava(t)
	s := newAddonTestSupervisor(t)
	ctx := t.Context()
	setAddonConfigs(s, map[string]string{"jmx:tomcat": "otel.jmx.target.system=tomcat\n"})

	require.NoError(t, installPackage(s.packages, "jmx:tomcat", fakeJar("1"), "https://example.invalid/opentelemetry-jmx-scraper.jar", "1.0.0"))
	s.reconcileAddons(ctx)
	first, _, _ := s.addons.Get("jmx:tomcat")
	require.True(t, first.(runningAddon).IsRunning())

	require.NoError(t, installPackage(s.packages, "jmx:tomcat", fakeJar("2"), "https://example.invalid/opentelemetry-jmx-scraper.jar", "2.0.0"))
	assert.False(t, first.(runningAddon).IsRunning(), "the previous scraper must be stopped before its files are replaced")

	s.reconcileAddons(ctx)
	second, _, _ := s.addons.Get("jmx:tomcat")
	assert.NotSame(t, first, second)
	assert.True(t, second.(runningAddon).IsRunning(), "the upgraded scraper must be configured and started")

	state, err := s.packages.PackageState("jmx:tomcat")
	require.NoError(t, err)
	assert.Equal(t, "2.0.0", state.Version)
}

// --- file ---

func TestFileAddonThroughSupervisor(t *testing.T) {
	s := newAddonTestSupervisor(t)

	require.NoError(t, installPackage(s.packages, "file:geoip", []byte("pretend-database"), "https://example.invalid/GeoLite2-City.mmdb", "2026.10.28"))

	assert.False(t, s.reconcileAddons(t.Context()))
	_, dir, ok := s.addons.Get("file:geoip")
	require.True(t, ok)
	// This path is what a geoipprocessor database_path points at.
	assert.FileExists(t, filepath.Join(dir, "GeoLite2-City.mmdb"))
}

// --- framework behavior ---

// TestUnsupportedAddonTypeRejectedBeforeDownload checks that the Supervisor
// refuses an addon type it was not built with, before fetching anything. The
// server can only choose among types the Supervisor supports.
func TestUnsupportedAddonTypeRejectedBeforeDownload(t *testing.T) {
	s := newAddonTestSupervisor(t)

	err := s.packages.CreatePackage("custom_rules:base", protobufs.PackageType_PackageType_Addon)
	require.ErrorContains(t, err, `unsupported addon type "custom_rules"`)

	err = s.packages.CreatePackage("no-type", protobufs.PackageType_PackageType_Addon)
	require.ErrorContains(t, err, `must have the form "<type>:<name>"`)

	names, err := s.packages.Packages()
	require.NoError(t, err)
	assert.Empty(t, names)

	// Top-level packages are not addons and are not subject to the naming rule.
	require.NoError(t, s.packages.CreatePackage("", protobufs.PackageType_PackageType_TopLevel))
}

func TestAddonContentHashMismatchRejected(t *testing.T) {
	s := newAddonTestSupervisor(t)
	require.NoError(t, s.packages.CreatePackage("file:bad", protobufs.PackageType_PackageType_Addon))

	err := s.packages.UpdateContent(t.Context(), "file:bad", "https://example.invalid/bad.mmdb",
		bytes.NewReader([]byte("x")), []byte("not-the-right-hash"), nil)
	require.ErrorContains(t, err, "content hash mismatch")
	assert.Empty(t, s.addons.Names())
	assert.NoDirExists(t, s.packages.AddonDir("file:bad"))
}

func TestFailedInstallLeavesPreviousVersion(t *testing.T) {
	s := newAddonTestSupervisor(t)
	require.NoError(t, installPackage(s.packages, "file:geoip", tarGz(t, map[string]string{"db.mmdb": "original"}), "https://example.invalid/geoip.tar.gz", "1"))

	// Not a gzip stream, though the URL says it is.
	err := installPackage(s.packages, "file:geoip", []byte("corrupt"), "https://example.invalid/geoip.tar.gz", "2")
	require.ErrorContains(t, err, "gzip")

	body, readErr := os.ReadFile(filepath.Join(s.packages.AddonDir("file:geoip"), "db.mmdb"))
	require.NoError(t, readErr)
	assert.Equal(t, "original", string(body))
	_, _, ok := s.addons.Get("file:geoip")
	assert.True(t, ok)
}

// TestAddonsSurviveRestart checks that a restarted Supervisor adopts its
// installed addons and runs them against the persisted config, before the
// Collector starts and without waiting to be re-offered anything.
func TestAddonsSurviveRestart(t *testing.T) {
	installFakeJava(t)
	s := newAddonTestSupervisor(t)
	require.NoError(t, installPackage(s.packages, "jmx:tomcat", fakeJar("1"), "https://example.invalid/opentelemetry-jmx-scraper.jar", "1"))
	require.NoError(t, s.packages.SetAllPackagesHash([]byte("all-hash")))
	s.stopAddons(t.Context())

	restarted := newAddonTestSupervisorWithConfig(t, s.config)
	setAddonConfigs(restarted, map[string]string{"jmx:tomcat": "otel.jmx.target.system=tomcat\n"})
	restarted.startAddons(t.Context())

	a, _, ok := restarted.addons.Get("jmx:tomcat")
	require.True(t, ok)
	assert.True(t, a.(runningAddon).IsRunning())
	assert.Zero(t, restartSignals(restarted), "startup reconciliation runs before the Collector starts")

	hash, err := restarted.packages.AllPackagesHash()
	require.NoError(t, err)
	assert.Equal(t, []byte("all-hash"), hash)
}

func TestShutdownStopsAddons(t *testing.T) {
	installFakeJava(t)
	s := newAddonTestSupervisor(t)
	require.NoError(t, installPackage(s.packages, "jmx:tomcat", fakeJar("1"), "https://example.invalid/opentelemetry-jmx-scraper.jar", "1"))
	setAddonConfigs(s, map[string]string{"jmx:tomcat": "otel.jmx.target.system=tomcat\n"})
	s.reconcileAddons(t.Context())

	a, _, _ := s.addons.Get("jmx:tomcat")
	require.True(t, a.(runningAddon).IsRunning())
	s.stopAddons(t.Context())
	assert.False(t, a.(runningAddon).IsRunning(), "the Supervisor must not leave orphaned addon processes")
}

func TestLastReportedStatusesRoundTrip(t *testing.T) {
	s := newAddonTestSupervisor(t)

	statuses, err := s.packages.LastReportedStatuses()
	require.NoError(t, err)
	assert.Empty(t, statuses.GetPackages())

	require.NoError(t, s.packages.SetLastReportedStatuses(&protobufs.PackageStatuses{
		ServerProvidedAllPackagesHash: []byte("offer-hash"),
		Packages: map[string]*protobufs.PackageStatus{
			"jmx:tomcat": {Name: "jmx:tomcat", AgentHasVersion: "1.0.0", Status: protobufs.PackageStatusEnum_PackageStatusEnum_Installed},
		},
	}))

	statuses, err = s.packages.LastReportedStatuses()
	require.NoError(t, err)
	assert.Equal(t, []byte("offer-hash"), statuses.GetServerProvidedAllPackagesHash())
	assert.Equal(t, protobufs.PackageStatusEnum_PackageStatusEnum_Installed, statuses.GetPackages()["jmx:tomcat"].GetStatus())
}

// TestAddonConfigIsNotMergedIntoCollectorConfig checks that addon config,
// selected by role, never reaches the Collector's configuration.
func TestAddonConfigIsNotMergedIntoCollectorConfig(t *testing.T) {
	s := newAddonTestSupervisor(t)

	remoteConfig := &protobufs.AgentRemoteConfig{Config: &protobufs.AgentConfigMap{ConfigMap: map[string]*protobufs.AgentConfigObject{
		"collector.yaml": {Body: []byte("receivers:\n  nop:\n"), ContentType: "text/yaml"},
		"jmx:tomcat":     {Body: []byte("otel.jmx.target.system=tomcat\n"), ContentType: "text/plain", Role: addonConfigRole},
		// A Collector config entry whose key merely looks like an addon name is
		// still Collector config: the role decides, not the key.
		"file:base.yaml": {Body: []byte("exporters:\n  nop:\n"), ContentType: "text/yaml"},
	}}}

	var composed []byte
	for _, compose := range s.createRemoteConfigComposers(remoteConfig) {
		composed = append(composed, compose()...)
	}
	assert.Contains(t, string(composed), "receivers")
	assert.Contains(t, string(composed), "exporters", "a key shaped like an addon name without the role is Collector config")
	assert.NotContains(t, string(composed), "otel.jmx.target.system")

	assert.Equal(t, map[string][]byte{"jmx:tomcat": []byte("otel.jmx.target.system=tomcat\n")}, addonConfigs(remoteConfig))
}

// --- distribution-defined type ---

// customRulesFactory is the shape of an addon type a distribution registers
// with WithAddonFactories: a rule set the Collector itself loads at startup,
// rather than a process. Applying a change to it requires restarting the
// Collector, which is what Effect.RestartAgent is for.
type customRulesFactory struct{}

func (customRulesFactory) Type() addon.Type { return "custom_rules" }

func (customRulesFactory) Install(_ context.Context, set addon.InstallSettings) error {
	return addon.ExtractTarGz(set.Content, set.Dir)
}

func (customRulesFactory) Create(_ context.Context, set addon.Settings) (addon.Addon, error) {
	return &customRules{dir: set.Dir}, nil
}

type customRules struct {
	dir        string
	registered bool
	state      string
}

// Configure takes the rule set's state ("enabled" or "disabled") from its
// config entry and writes it where the Collector reads it.
func (r *customRules) Configure(_ context.Context, body []byte) (addon.Effect, error) {
	state := "enabled"
	if body != nil {
		state = strings.TrimSpace(string(body))
	}
	if state != "enabled" && state != "disabled" {
		return addon.Effect{}, fmt.Errorf("unknown rule set state %q", state)
	}
	if state == r.state {
		return addon.Effect{}, nil
	}
	r.state = state
	if err := os.WriteFile(filepath.Join(r.dir, "state"), []byte("state = "+state+"\n"), 0o600); err != nil {
		return addon.Effect{}, err
	}
	// A state change on a rule set the Collector already loaded takes a restart.
	return addon.Effect{RestartAgent: r.registered}, nil
}

// Start registers the rule set with the Collector the first time.
func (r *customRules) Start(context.Context) (addon.Effect, error) {
	if r.registered {
		return addon.Effect{}, nil
	}
	r.registered = true
	return addon.Effect{RestartAgent: true}, nil
}

func (*customRules) Shutdown(context.Context) error { return nil }

// gatedSyncer stands in for the opamp-go PackagesSyncer: its sync runs only
// once the test releases the gate, so the test can observe the Supervisor
// before and after the packages are in place.
type gatedSyncer struct {
	gate chan struct{}
	sync func()
	done chan struct{}
}

func newGatedSyncer(sync func()) *gatedSyncer {
	return &gatedSyncer{gate: make(chan struct{}), sync: sync, done: make(chan struct{})}
}

func (g *gatedSyncer) Sync(context.Context) error {
	go func() {
		<-g.gate
		g.sync()
		close(g.done)
	}()
	return nil
}

func (g *gatedSyncer) Done() <-chan struct{} { return g.done }

func TestDistributionRegistersAddonType(t *testing.T) {
	s := newAddonTestSupervisor(t, customRulesFactory{})
	assert.Equal(t, []addon.Type{"custom_rules", "file", "jmx"}, s.addons.Types())

	_, err := addon.NewManager(zap.NewNop(), append(builtinAddonFactories(), customRulesFactory{}, customRulesFactory{})...)
	require.ErrorContains(t, err, "registered more than once")
}

// TestOneCollectorRestartPerMessage covers the ordering concern: a message that
// changes the Collector config and installs a rule set the Collector reads
// must cause exactly one Collector restart, after the rule set is in place, rather than
// one restart with the old addons and another with the new.
func TestOneCollectorRestartPerMessage(t *testing.T) {
	s := newAddonTestSupervisor(t, customRulesFactory{})
	ctx := t.Context()

	// The same message carries the rule set's state in the remote config.
	setAddonConfigs(s, map[string]string{"custom_rules:base": "disabled"})

	syncer := newGatedSyncer(func() {
		require.NoError(t, installPackage(s.packages, "custom_rules:base", tarGz(t, map[string]string{"rules.yaml": "rules: []\n"}),
			"https://example.invalid/rules.tar.gz", "9.0.0"))
	})

	// restartPending: the Collector config changed in this message too.
	s.syncPackages(ctx, syncer, true)

	// While the package is still syncing, the Collector must not restart.
	time.Sleep(100 * time.Millisecond)
	assert.Zero(t, restartSignals(s), "the Collector must not restart before the rule set is in place")

	close(syncer.gate)
	require.Eventually(t, func() bool { return restartSignals(s) == 1 }, 10*time.Second, 10*time.Millisecond)

	// When the restart is signaled, the rule set and its state are already on disk.
	_, dir, ok := s.addons.Get("custom_rules:base")
	require.True(t, ok)
	conf, err := os.ReadFile(filepath.Join(dir, "state"))
	require.NoError(t, err)
	assert.Contains(t, string(conf), "state = disabled")
}

// TestAddonStateChangeRestartsCollectorOnce covers a state-only change to an
// installed rule set: no download, one restart.
func TestAddonStateChangeRestartsCollectorOnce(t *testing.T) {
	s := newAddonTestSupervisor(t, customRulesFactory{})
	ctx := t.Context()

	require.NoError(t, installPackage(s.packages, "custom_rules:base", tarGz(t, map[string]string{"rules.yaml": "rules: []\n"}),
		"https://example.invalid/rules.tar.gz", "9.0.0"))
	setAddonConfigs(s, map[string]string{"custom_rules:base": "enabled"})
	require.True(t, s.reconcileAddons(ctx), "registering a new rule set needs a Collector restart")

	// Same state again: nothing to do.
	assert.False(t, s.reconcileAddons(ctx))

	// Disable: the Collector must restart to stop reading the rule set.
	setAddonConfigs(s, map[string]string{"custom_rules:base": "disabled"})
	assert.True(t, s.reconcileAddons(ctx))

	_, dir, _ := s.addons.Get("custom_rules:base")
	conf, err := os.ReadFile(filepath.Join(dir, "state"))
	require.NoError(t, err)
	assert.Contains(t, string(conf), "state = disabled")
}

// TestNoRestartWhenNothingChanged checks that a package offer carrying no
// change to the Collector does not restart it.
func TestNoRestartWhenNothingChanged(t *testing.T) {
	s := newAddonTestSupervisor(t)
	syncer := newGatedSyncer(func() {
		require.NoError(t, installPackage(s.packages, "file:geoip", []byte("db"), "https://example.invalid/GeoLite2-City.mmdb", "1"))
	})
	close(syncer.gate)

	s.syncPackages(t.Context(), syncer, false)
	<-syncer.Done()
	time.Sleep(100 * time.Millisecond)
	assert.Zero(t, restartSignals(s))
}

// TestPackageDirIsSafeFileName pins the escaping of package names on disk. An
// addon name always contains ":", which is illegal in Windows file names and is
// the JVM's class path separator: a scraper jar under "jmx:tomcat/" fails to
// load its main class. Only the real JVM surfaced this.
func TestPackageDirIsSafeFileName(t *testing.T) {
	s := newAddonTestSupervisor(t)
	seen := map[string]string{}

	for _, name := range []string{"jmx:tomcat", "custom_rules:a/b", `file:x*y?<>|"`, "file:..", "file:a b", "file:a+b"} {
		base := filepath.Base(s.packages.AddonDir(name))
		require.Equal(t, packageFilesDir, base)
		dirName := filepath.Base(filepath.Dir(s.packages.AddonDir(name)))

		assert.NotContains(t, dirName, ":", name)
		assert.NotContains(t, dirName, string(os.PathListSeparator), name)
		assert.NotContains(t, dirName, "/", name)
		assert.NotContains(t, dirName, `\`, name)
		assert.NotEqual(t, "..", dirName, name)

		// Distinct names must not collide on disk.
		if other, ok := seen[dirName]; ok {
			t.Fatalf("%q and %q map to the same directory %q", name, other, dirName)
		}
		seen[dirName] = name
	}
}

// TestNewSupervisorRegistersAddonTypes checks the distribution hook through the
// real constructor rather than a hand-built Supervisor.
func TestNewSupervisorRegistersAddonTypes(t *testing.T) {
	executable := filepath.Join(t.TempDir(), "otelcol")
	require.NoError(t, os.WriteFile(executable, []byte("#!/bin/sh\n"), 0o700)) //nolint:gosec // G306: agent::executable must be executable

	cfg := config.DefaultSupervisor()
	cfg.Server.Endpoint = "ws://localhost:4320/v1/opamp"
	cfg.Storage.Directory = t.TempDir()
	cfg.Agent.Executable = executable
	cfg.Capabilities.AcceptsPackages = true

	s, err := NewSupervisor(t.Context(), zap.NewNop(), cfg, WithAddonFactories(customRulesFactory{}))
	require.NoError(t, err)
	assert.Equal(t, []addon.Type{"custom_rules", "file", "jmx"}, s.addons.Types())

	// A distribution cannot replace a built-in type by registering it again.
	_, err = NewSupervisor(t.Context(), zap.NewNop(), cfg, WithAddonFactories(fileaddonDuplicate{}))
	require.ErrorContains(t, err, `addon type "file" is registered more than once`)

	// Without the packages capability nothing is wired, and no addon runs.
	cfg.Capabilities.AcceptsPackages = false
	s, err = NewSupervisor(t.Context(), zap.NewNop(), cfg, WithAddonFactories(customRulesFactory{}))
	require.NoError(t, err)
	assert.Nil(t, s.addons)
	assert.Nil(t, s.packages)
}

type fileaddonDuplicate struct{ customRulesFactory }

func (fileaddonDuplicate) Type() addon.Type { return "file" }

// TestHeldBackRestartSurvivesCancelledMessageContext covers a dropped
// connection during a package sync. Over WebSocket the context passed with a
// message is the connection's, and it is cancelled on disconnect. The
// reconciliation and the held-back Collector restart must still happen once the
// sync completes.
func TestHeldBackRestartSurvivesCancelledMessageContext(t *testing.T) {
	s := newAddonTestSupervisor(t, customRulesFactory{})
	setAddonConfigs(s, map[string]string{"custom_rules:base": "disabled"})

	syncer := newGatedSyncer(func() {
		require.NoError(t, installPackage(s.packages, "custom_rules:base", tarGz(t, map[string]string{"rules.yaml": "rules: []\n"}),
			"https://example.invalid/rules.tar.gz", "9.0.0"))
	})

	msgCtx, cancel := context.WithCancel(t.Context())
	s.syncPackages(msgCtx, syncer, true)

	// The connection drops before the sync finishes.
	cancel()
	close(syncer.gate)

	require.Eventually(t, func() bool { return restartSignals(s) == 1 }, 10*time.Second, 10*time.Millisecond,
		"the held-back Collector restart must not be lost when the message context ends")
	_, dir, ok := s.addons.Get("custom_rules:base")
	require.True(t, ok)
	conf, err := os.ReadFile(filepath.Join(dir, "state"))
	require.NoError(t, err)
	assert.Contains(t, string(conf), "state = disabled", "the addon must still be reconciled")
}

// --- reporting addon configuration to the server ---

// flakyFactory is an addon type whose addons reject non-empty configuration
// while reject is set.
type flakyFactory struct{ reject *atomic.Bool }

func (flakyFactory) Type() addon.Type { return "flaky" }

func (flakyFactory) Install(context.Context, addon.InstallSettings) error { return nil }

func (f flakyFactory) Create(context.Context, addon.Settings) (addon.Addon, error) {
	return &flakyAddon{reject: f.reject}, nil
}

type flakyAddon struct{ reject *atomic.Bool }

func (a *flakyAddon) Configure(_ context.Context, body []byte) (addon.Effect, error) {
	if body != nil && a.reject.Load() {
		return addon.Effect{}, errors.New("invalid configuration")
	}
	return addon.Effect{}, nil
}

func (*flakyAddon) Start(context.Context) (addon.Effect, error) { return addon.Effect{}, nil }

func (*flakyAddon) Shutdown(context.Context) error { return nil }

// TestAddonConfigReportedInEffectiveConfig checks that the configuration each
// addon runs with is reported in the effective config, under the key and role
// it was offered with, alongside the Collector's own effective config.
func TestAddonConfigReportedInEffectiveConfig(t *testing.T) {
	s := newAddonTestSupervisor(t, customRulesFactory{})
	ctx := t.Context()

	collectorConfig := &protobufs.EffectiveConfig{ConfigMap: &protobufs.AgentConfigMap{
		ConfigMap: map[string]*protobufs.AgentConfigObject{"": {Body: []byte("receivers: {}\n")}},
	}}
	s.effectiveConfig.Store(collectorConfig)

	require.NoError(t, installPackage(s.packages, "custom_rules:base",
		tarGz(t, map[string]string{"rules.yaml": "rules: []\n"}), "https://example.invalid/rules.tar.gz", "1"))

	r := recordReports(s)
	setAddonConfigs(s, map[string]string{"custom_rules:base": "enabled"})
	s.reconcileAddons(ctx)
	assert.Equal(t, 1, r.updates(), "an applied addon configuration must update the effective config")

	effective := s.createEffectiveConfigMsg().GetConfigMap().GetConfigMap()
	require.Contains(t, effective, "")
	assert.Equal(t, "receivers: {}\n", string(effective[""].GetBody()))
	require.Contains(t, effective, "custom_rules:base")
	assert.Equal(t, "enabled", string(effective["custom_rules:base"].GetBody()))
	assert.Equal(t, "text/plain", effective["custom_rules:base"].GetContentType())
	assert.Equal(t, addonConfigRole, effective["custom_rules:base"].GetRole())
	assert.Len(t, collectorConfig.GetConfigMap().GetConfigMap(), 1, "the Collector's effective config must not be modified")

	// Unchanged: no update.
	s.reconcileAddons(ctx)
	assert.Equal(t, 1, r.updates())

	// Withdrawn: the entry goes away.
	setAddonConfigs(s, map[string]string{})
	s.reconcileAddons(ctx)
	assert.Equal(t, 2, r.updates())
	assert.NotContains(t, s.createEffectiveConfigMsg().GetConfigMap().GetConfigMap(), "custom_rules:base")
}

// TestAddonConfigRejectionReportedAsFailedStatus checks that an addon rejecting
// its configuration is reported as a FAILED remote config status, that the
// Collector applying its part of the same config does not hide it, and that the
// config is reported applied once the addon accepts it.
func TestAddonConfigRejectionReportedAsFailedStatus(t *testing.T) {
	reject := &atomic.Bool{}
	reject.Store(true)
	s := newAddonTestSupervisor(t, flakyFactory{reject: reject})
	ctx := t.Context()

	require.NoError(t, installPackage(s.packages, "flaky:x", []byte("x"), "https://example.invalid/x", "1"))
	r := recordReports(s)
	setAddonConfigs(s, map[string]string{"flaky:x": "v1"})
	hash := s.remoteConfig.Load().GetConfigHash()

	s.reconcileAddons(ctx)
	last := r.last()
	require.NotNil(t, last)
	assert.Equal(t, protobufs.RemoteConfigStatuses_RemoteConfigStatuses_FAILED, last.GetStatus())
	assert.Equal(t, hash, last.GetLastRemoteConfigHash())
	assert.Contains(t, last.GetErrorMessage(), `"flaky:x"`)
	assert.Contains(t, last.GetErrorMessage(), "invalid configuration")

	// The Collector applying its part of the config must not report it applied.
	s.saveAndReportConfigStatus(protobufs.RemoteConfigStatuses_RemoteConfigStatuses_APPLIED, "")
	assert.Equal(t, protobufs.RemoteConfigStatuses_RemoteConfigStatuses_FAILED, r.last().GetStatus())
	assert.Contains(t, r.last().GetErrorMessage(), `"flaky:x"`)

	// Once the addon accepts the same config, it is applied.
	reject.Store(false)
	s.reconcileAddons(ctx)
	assert.Equal(t, protobufs.RemoteConfigStatuses_RemoteConfigStatuses_APPLIED, r.last().GetStatus())
	assert.Empty(t, r.last().GetErrorMessage())
	assert.Equal(t, protobufs.RemoteConfigStatuses_RemoteConfigStatuses_APPLIED,
		s.persistentState.GetLastRemoteConfigStatus().GetStatus(), "the persisted status must follow")
}

// TestAddonConfigErrorDoesNotOutliveItsConfig checks that a rejection is tied
// to the config it was found in: a new config is reported on its own merits.
func TestAddonConfigErrorDoesNotOutliveItsConfig(t *testing.T) {
	reject := &atomic.Bool{}
	reject.Store(true)
	s := newAddonTestSupervisor(t, flakyFactory{reject: reject})
	ctx := t.Context()

	require.NoError(t, installPackage(s.packages, "flaky:x", []byte("x"), "https://example.invalid/x", "1"))
	r := recordReports(s)
	setAddonConfigs(s, map[string]string{"flaky:x": "v1"})
	s.reconcileAddons(ctx)
	require.Equal(t, protobufs.RemoteConfigStatuses_RemoteConfigStatuses_FAILED, r.last().GetStatus())

	// A new config arrives and is reported applied before addons are reconciled,
	// as processRemoteConfigMessage does when the Collector config is unchanged.
	reject.Store(false)
	setAddonConfigs(s, map[string]string{"flaky:x": "v2"})
	s.saveAndReportConfigStatus(protobufs.RemoteConfigStatuses_RemoteConfigStatuses_APPLIED, "")
	assert.Equal(t, protobufs.RemoteConfigStatuses_RemoteConfigStatuses_APPLIED, r.last().GetStatus(),
		"the previous config's rejection must not apply to a new config")

	s.reconcileAddons(ctx)
	assert.Equal(t, protobufs.RemoteConfigStatuses_RemoteConfigStatuses_APPLIED, r.last().GetStatus())
	assert.Equal(t, s.remoteConfig.Load().GetConfigHash(), r.last().GetLastRemoteConfigHash())
}
