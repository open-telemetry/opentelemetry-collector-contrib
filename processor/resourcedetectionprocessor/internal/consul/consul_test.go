// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package consul

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/processor/processortest"
	"go.opentelemetry.io/otel/attribute"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/consul/internal/metadata"
)

const (
	testNodeID    = "00000000-0000-0000-0000-000000000000"
	testSchemaURL = "https://opentelemetry.io/schemas/1.43.0"

	fullAgentResponse = `{
		"Config": {"NodeName": "hostname", "Datacenter": "dc1", "NodeID": "` + testNodeID + `"},
		"Meta": {"test": "test", "ignored": "ignored"}
	}`
)

// fakeDetector stands in for the upstream SDK detector.
type fakeDetector struct {
	res *sdkresource.Resource
	err error
}

func (f fakeDetector) Detect(context.Context) (*sdkresource.Resource, error) {
	return f.res, f.err
}

func withFakeDetector(t *testing.T, res *sdkresource.Resource, err error) {
	t.Helper()

	orig := newResourceDetector
	newResourceDetector = func(Config) sdkresource.Detector {
		return fakeDetector{res: res, err: err}
	}
	t.Cleanup(func() { newResourceDetector = orig })
}

// fullResource mirrors what the SDK detector reports for an agent with one allowlisted meta key.
func fullResource() *sdkresource.Resource {
	return sdkresource.NewWithAttributes(testSchemaURL,
		attribute.String("host.name", "hostname"),
		attribute.String("cloud.region", "dc1"),
		attribute.String("host.id", testNodeID),
		attribute.String("consul.meta.test", "test"),
	)
}

// setPrefixMetaAttributesGate forces the state of the meta attribute prefix gate
// for the duration of the test.
func setPrefixMetaAttributesGate(t *testing.T, enabled bool) {
	gate := metadata.ProcessorResourcedetectionConsulPrefixMetaAttributesFeatureGate
	originalValue := gate.IsEnabled()
	require.NoError(t, featuregate.GlobalRegistry().Set(gate.ID(), enabled))
	t.Cleanup(func() {
		require.NoError(t, featuregate.GlobalRegistry().Set(gate.ID(), originalValue))
	})
}

func newTestDetector(t *testing.T, cfg Config, failOnMissingMetadata bool) internal.Detector {
	t.Helper()

	d, err := NewDetector(processortest.NewNopSettings(processortest.NopType), cfg, failOnMissingMetadata)
	require.NoError(t, err)
	require.NotNil(t, d)
	return d
}

func TestDetect(t *testing.T) {
	withFakeDetector(t, fullResource(), nil)

	res, schemaURL, err := newTestDetector(t, CreateDefaultConfig(), false).Detect(t.Context())
	require.NoError(t, err)
	assert.Equal(t, testSchemaURL, schemaURL)
	assert.Equal(t, map[string]any{
		"host.name":    "hostname",
		"cloud.region": "dc1",
		"host.id":      testNodeID,
		"test":         "test",
	}, res.Attributes().AsRaw())
}

func TestDetectPrefixedMetaAttributes(t *testing.T) {
	setPrefixMetaAttributesGate(t, true)
	withFakeDetector(t, fullResource(), nil)

	res, schemaURL, err := newTestDetector(t, CreateDefaultConfig(), false).Detect(t.Context())
	require.NoError(t, err)
	assert.Equal(t, testSchemaURL, schemaURL)
	assert.Equal(t, map[string]any{
		"host.name":        "hostname",
		"cloud.region":     "dc1",
		"host.id":          testNodeID,
		"consul.meta.test": "test",
	}, res.Attributes().AsRaw())
}

// Meta keys are not declared in metadata.yaml, so they must survive RemoveDisabledAttributes
// even when every declared attribute is disabled.
func TestDetectMetaSurvivesDisabledAttributes(t *testing.T) {
	for _, gateEnabled := range []bool{false, true} {
		t.Run(fmt.Sprintf("gate=%v", gateEnabled), func(t *testing.T) {
			setPrefixMetaAttributesGate(t, gateEnabled)
			withFakeDetector(t, fullResource(), nil)

			cfg := CreateDefaultConfig()
			cfg.ResourceAttributes.HostName.Enabled = false
			cfg.ResourceAttributes.CloudRegion.Enabled = false
			cfg.ResourceAttributes.HostID.Enabled = false

			res, _, err := newTestDetector(t, cfg, false).Detect(t.Context())
			require.NoError(t, err)

			key := "test"
			if gateEnabled {
				key = "consul.meta.test"
			}
			assert.Equal(t, map[string]any{key: "test"}, res.Attributes().AsRaw())
		})
	}
}

func TestDetectDisabledAttributeRemoved(t *testing.T) {
	withFakeDetector(t, fullResource(), nil)

	cfg := CreateDefaultConfig()
	cfg.ResourceAttributes.HostID.Enabled = false

	res, _, err := newTestDetector(t, cfg, false).Detect(t.Context())
	require.NoError(t, err)
	assert.Equal(t, map[string]any{
		"host.name":    "hostname",
		"cloud.region": "dc1",
		"test":         "test",
	}, res.Attributes().AsRaw())
}

// A meta key named like a declared attribute replaces it, even when that attribute is disabled.
func TestDetectMetaKeyMatchingDeclaredAttribute(t *testing.T) {
	withFakeDetector(t, sdkresource.NewSchemaless(
		attribute.String("host.name", "hostname"),
		attribute.String("consul.meta.host.name", "from-meta"),
	), nil)

	cfg := CreateDefaultConfig()
	cfg.ResourceAttributes.HostName.Enabled = false

	res, _, err := newTestDetector(t, cfg, false).Detect(t.Context())
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"host.name": "from-meta"}, res.Attributes().AsRaw())
}

func TestDetectEmpty(t *testing.T) {
	withFakeDetector(t, sdkresource.Empty(), nil)

	t.Run("error ignored", func(t *testing.T) {
		res, schemaURL, err := newTestDetector(t, CreateDefaultConfig(), false).Detect(t.Context())
		require.NoError(t, err)
		assert.True(t, internal.IsEmptyResource(res))
		assert.Empty(t, schemaURL)
	})
	t.Run("error returned", func(t *testing.T) {
		res, schemaURL, err := newTestDetector(t, CreateDefaultConfig(), true).Detect(t.Context())
		require.ErrorContains(t, err, "consul metadata unavailable")
		assert.True(t, internal.IsEmptyResource(res))
		assert.Empty(t, schemaURL)
	})
}

func TestDetectError(t *testing.T) {
	someErr := errors.New("consul agent did not report a Config section")
	withFakeDetector(t, nil, someErr)

	t.Run("error ignored", func(t *testing.T) {
		res, schemaURL, err := newTestDetector(t, CreateDefaultConfig(), false).Detect(t.Context())
		require.NoError(t, err)
		assert.True(t, internal.IsEmptyResource(res))
		assert.Empty(t, schemaURL)
	})
	t.Run("error returned", func(t *testing.T) {
		res, schemaURL, err := newTestDetector(t, CreateDefaultConfig(), true).Detect(t.Context())
		require.ErrorIs(t, err, someErr)
		assert.True(t, internal.IsEmptyResource(res))
		assert.Empty(t, schemaURL)
	})
}

// A partial result is kept with or without fail_on_missing_metadata.
func TestDetectPartial(t *testing.T) {
	partial := sdkresource.NewSchemaless(
		attribute.String("host.name", "hostname"),
		attribute.String("consul.meta.test", "test"),
	)
	withFakeDetector(t, partial, fmt.Errorf("%w: Datacenter: not present in agent configuration", sdkresource.ErrPartialResource))

	for _, failOnMissingMetadata := range []bool{false, true} {
		t.Run(fmt.Sprintf("fail=%v", failOnMissingMetadata), func(t *testing.T) {
			res, _, err := newTestDetector(t, CreateDefaultConfig(), failOnMissingMetadata).Detect(t.Context())
			require.NoError(t, err)
			assert.Equal(t, map[string]any{"host.name": "hostname", "test": "test"}, res.Attributes().AsRaw())
		})
	}
}

// fakeAgent serves /v1/agent/self with body and returns the agent address along with the last
// request it received.
func fakeAgent(t *testing.T, body string) (string, *atomic.Pointer[http.Request]) {
	t.Helper()

	received := &atomic.Pointer[http.Request]{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received.Store(r)
		if r.URL.Path != "/v1/agent/self" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return strings.TrimPrefix(srv.URL, "http://"), received
}

// clearConsulEnv keeps the machine's own CONSUL_* variables out of tests using the real SDK detector.
func clearConsulEnv(t *testing.T) {
	t.Helper()

	for _, name := range []string{"CONSUL_HTTP_ADDR", "CONSUL_HTTP_TOKEN", "CONSUL_HTTP_TOKEN_FILE", "CONSUL_NAMESPACE"} {
		t.Setenv(name, "")
	}
}

func TestDetectAgentOptions(t *testing.T) {
	clearConsulEnv(t)
	address, received := fakeAgent(t, fullAgentResponse)

	cfg := CreateDefaultConfig()
	cfg.Address = address
	cfg.Datacenter = "dc2"
	cfg.Namespace = "team-a"
	cfg.Token = "direct-token"
	cfg.MetaLabels = map[string]any{"test": ""}

	res, schemaURL, err := newTestDetector(t, cfg, false).Detect(t.Context())
	require.NoError(t, err)
	assert.Equal(t, testSchemaURL, schemaURL)
	assert.Equal(t, map[string]any{
		"host.name":    "hostname",
		"cloud.region": "dc1",
		"host.id":      testNodeID,
		"test":         "test",
	}, res.Attributes().AsRaw())

	req := received.Load()
	assert.Equal(t, "dc2", req.URL.Query().Get("dc"))
	assert.Equal(t, "team-a", req.URL.Query().Get("ns"))
	assert.Equal(t, "direct-token", req.Header.Get("X-Consul-Token"))
}

func TestDetectAgentEmptyMetaAllowlist(t *testing.T) {
	clearConsulEnv(t)
	address, _ := fakeAgent(t, fullAgentResponse)

	cfg := CreateDefaultConfig()
	cfg.Address = address

	res, _, err := newTestDetector(t, cfg, false).Detect(t.Context())
	require.NoError(t, err)
	assert.Equal(t, map[string]any{
		"host.name":    "hostname",
		"cloud.region": "dc1",
		"host.id":      testNodeID,
	}, res.Attributes().AsRaw())
}

func TestDetectAgentEnvironmentDefaults(t *testing.T) {
	clearConsulEnv(t)
	address, received := fakeAgent(t, fullAgentResponse)
	t.Setenv("CONSUL_HTTP_ADDR", address)
	t.Setenv("CONSUL_HTTP_TOKEN", "env-token")
	t.Setenv("CONSUL_NAMESPACE", "env-namespace")

	res, _, err := newTestDetector(t, CreateDefaultConfig(), true).Detect(t.Context())
	require.NoError(t, err)
	assert.Equal(t, 3, res.Attributes().Len())
	req := received.Load()
	assert.Equal(t, "env-token", req.Header.Get("X-Consul-Token"))
	assert.Equal(t, "env-namespace", req.URL.Query().Get("ns"))
}

func TestDetectAgentTokenFile(t *testing.T) {
	clearConsulEnv(t)
	address, received := fakeAgent(t, fullAgentResponse)

	tokenFile := filepath.Join(t.TempDir(), "token")
	require.NoError(t, os.WriteFile(tokenFile, []byte("secret-token-value"), 0o600))

	cfg := CreateDefaultConfig()
	cfg.Address = address
	cfg.TokenFile = tokenFile

	_, _, err := newTestDetector(t, cfg, true).Detect(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "secret-token-value", received.Load().Header.Get("X-Consul-Token"))
}

// A client creation failure no longer fails NewDetector; it surfaces from Detect.
func TestDetectAgentMissingTokenFile(t *testing.T) {
	clearConsulEnv(t)

	cfg := CreateDefaultConfig()
	cfg.TokenFile = filepath.Join(t.TempDir(), "missing-token")

	res, _, err := newTestDetector(t, cfg, false).Detect(t.Context())
	require.NoError(t, err)
	assert.True(t, internal.IsEmptyResource(res))

	_, _, err = newTestDetector(t, cfg, true).Detect(t.Context())
	require.ErrorContains(t, err, "failed creating consul client")
}

// A refused connection means no agent here, which is an empty result rather than an SDK error.
func TestDetectAgentUnreachable(t *testing.T) {
	clearConsulEnv(t)
	srv := httptest.NewServer(http.NotFoundHandler())
	address := strings.TrimPrefix(srv.URL, "http://")
	srv.Close()

	cfg := CreateDefaultConfig()
	cfg.Address = address

	res, _, err := newTestDetector(t, cfg, false).Detect(t.Context())
	require.NoError(t, err)
	assert.True(t, internal.IsEmptyResource(res))

	_, _, err = newTestDetector(t, cfg, true).Detect(t.Context())
	require.ErrorContains(t, err, "consul metadata unavailable")
}

// Missing agent fields give a partial result, and a non-string meta value is skipped.
func TestDetectAgentPartialAndNonStringMeta(t *testing.T) {
	clearConsulEnv(t)
	address, _ := fakeAgent(t, `{
		"Config": {"NodeName": "hostname"},
		"Meta": {"test": "test", "number": 1}
	}`)

	cfg := CreateDefaultConfig()
	cfg.Address = address
	cfg.MetaLabels = map[string]any{"test": "", "number": ""}

	res, _, err := newTestDetector(t, cfg, true).Detect(t.Context())
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"host.name": "hostname", "test": "test"}, res.Attributes().AsRaw())
}

func TestCreateDefaultConfig(t *testing.T) {
	assert.Equal(t, Config{ResourceAttributes: metadata.DefaultResourceAttributesConfig()}, CreateDefaultConfig())
}
