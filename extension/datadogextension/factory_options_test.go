// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !aix

package datadogextension

import (
	"testing"

	pkgconfigmodel "github.com/DataDog/datadog-agent/pkg/config/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension/extensiontest"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/agentcomponents"
)

const testOptionsAPIKey = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

// testOptionsConfig returns a Config that create() will accept.
func testOptionsConfig(t *testing.T) *Config {
	t.Helper()
	cfg, ok := NewFactory().CreateDefaultConfig().(*Config)
	require.True(t, ok)
	cfg.API.Key = testOptionsAPIKey
	cfg.API.FailOnInvalidKey = false
	// Set a hostname so create() skips the cloud-metadata probe.
	cfg.Hostname = "test-host"
	return cfg
}

// Host options must run after the extension's own: a host write to api_key survives,
// where if it ran first WithAPIConfig would have overwritten it. Options cannot read the
// config (it has no schema yet), so the component is read after construction.
func TestBuildAgentConfigHostOptionsWinOverBuiltIns(t *testing.T) {
	cfg := testOptionsConfig(t)

	got := buildAgentConfig(cfg, func(pkgconfig pkgconfigmodel.Config) {
		pkgconfig.Set("api_key", "rotated-key", pkgconfigmodel.SourceAgentRuntime)
	})

	assert.Equal(t, "rotated-key", got.GetString("api_key"),
		"host option did not win over WithAPIConfig; options are applied in the wrong order")
	// A key the host did not touch must still carry the extension's own value.
	assert.Equal(t, cfg.API.Site, got.GetString("site"))
}

// TestBuildAgentConfigWithoutHostOptions pins the baseline the above test contrasts with.
func TestBuildAgentConfigWithoutHostOptions(t *testing.T) {
	cfg := testOptionsConfig(t)
	got := buildAgentConfig(cfg)
	assert.Equal(t, testOptionsAPIKey, got.GetString("api_key"))
}

// TestBuildAgentConfigAppliesHostOptionsInOrder verifies multiple host options are applied
// in the order supplied.
func TestBuildAgentConfigAppliesHostOptionsInOrder(t *testing.T) {
	var order []string
	first := agentcomponents.ConfigOption(func(pkgconfigmodel.Config) { order = append(order, "first") })
	second := agentcomponents.ConfigOption(func(pkgconfigmodel.Config) { order = append(order, "second") })

	buildAgentConfig(testOptionsConfig(t), first, second)

	assert.Equal(t, []string{"first", "second"}, order)
}

// No-regression guard for distributions that supply no options — notably OCB-generated
// builds, whose components.go can only ever call NewFactory().
func TestNewFactoryUnchangedWithNoOptions(t *testing.T) {
	cfg := testOptionsConfig(t)

	baseline := buildAgentConfig(cfg)
	withNoOptions := buildAgentConfig(cfg, []agentcomponents.ConfigOption{}...)

	// Every key the extension's own options set must match.
	for _, key := range []string{
		"api_key",
		"site",
		"logging_frequency",
		"forwarder_apikey_validation_interval",
		"forwarder_num_workers",
		"forwarder_backoff_factor",
		"forwarder_backoff_base",
		"forwarder_backoff_max",
		"forwarder_recovery_interval",
		"forwarder_http_protocol",
		"forwarder_max_concurrent_requests",
		"enable_payloads.events",
		"enable_payloads.json_to_v1_intake",
		"skip_ssl_validation",
	} {
		assert.Equal(t, baseline.Get(key), withNoOptions.Get(key), "key %q diverged", key)
	}

	// And the zero-option factory must still behave like the plain one.
	plain := NewFactory()
	withOpts := NewFactoryWithOptions()
	assert.Equal(t, plain.Type(), withOpts.Type())
	assert.Equal(t, plain.CreateDefaultConfig(), withOpts.CreateDefaultConfig())
}

// TestNewFactoryWithOptions verifies a host option survives the
// factory -> create -> newExtension -> buildAgentConfig path.
func TestNewFactoryWithOptions(t *testing.T) {
	var applied bool
	f := NewFactoryWithOptions(WithConfigOptions(func(pkgconfig pkgconfigmodel.Config) {
		applied = true
		pkgconfig.Set("api_key", "rotated-key", pkgconfigmodel.SourceAgentRuntime)
	}))

	ext, err := f.Create(t.Context(), extensiontest.NewNopSettings(component.MustNewType("datadog")), testOptionsConfig(t))
	require.NoError(t, err)
	require.NotNil(t, ext)
	assert.True(t, applied, "host option did not reach the extension's config component")
}

// TestWithConfigOptionsAccumulates verifies repeated WithConfigOptions calls append
// rather than replace.
func TestWithConfigOptionsAccumulates(t *testing.T) {
	var count int
	first := func(pkgconfigmodel.Config) { count++ }
	second := func(pkgconfigmodel.Config) { count += 10 }
	third := func(pkgconfigmodel.Config) { count += 100 }

	f := &factory{}
	WithConfigOptions(first)(f)
	WithConfigOptions(second, third)(f)
	require.Len(t, f.configOptions, 3)

	buildAgentConfig(testOptionsConfig(t), f.configOptions...)
	assert.Equal(t, 111, count, "all three options should have run exactly once")
}

// The handle a host option receives must stay live after construction — a static one-shot
// write would make the seam useless for its stated purpose. Rotate the key through the
// handle after buildAgentConfig has returned, and assert the extension's forwarder applies
// it, via the log its domain resolver emits from the callback NewOptions arms on the config.
func TestConfigOptionHandleRotatesKeyAfterConstruction(t *testing.T) {
	const rotatedKey = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

	var handle pkgconfigmodel.Config
	agentConfig := buildAgentConfig(testOptionsConfig(t), func(c pkgconfigmodel.Config) { handle = c })
	require.NotNil(t, handle, "host option must receive a config handle")
	require.Equal(t, testOptionsAPIKey, agentConfig.GetString("api_key"))

	// Build the serializer the way newExtension does: this constructs the forwarder, which
	// is what registers the resolver's rotation callback on agentConfig.
	core, logs := observer.New(zapcore.InfoLevel)
	logComponent := agentcomponents.NewLogComponent(component.TelemetrySettings{Logger: zap.New(core)})
	require.NotNil(t, agentcomponents.NewSerializerComponent(agentConfig, logComponent, "test-host"))

	handle.Set("api_key", rotatedKey, pkgconfigmodel.SourceAgentRuntime)

	assert.Equal(t, rotatedKey, agentConfig.GetString("api_key"))
	rotations := logs.FilterMessageSnippet("rotating API key for 'api_key'").All()
	assert.Len(t, rotations, 1, "forwarder's resolver should have applied the rotation")
}
