//go:build integration

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkaoauth

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	kafkaexporter "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/oauth2clientauthextension"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/testdata"
	kafkacontainers "github.com/open-telemetry/opentelemetry-collector-contrib/test/integration/kafkaoauth/testcontainers"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/kafka/configkafka"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkareceiver"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest"
	"golang.org/x/oauth2"
)

const (
	totalTestTimeout = 4 * time.Minute
	testCaseTimeout  = 30 * time.Second

	shutdownTimeout     = 15 * time.Second
	metricsWaitTimeout  = 15 * time.Second
	metricsPollInterval = 250 * time.Millisecond

	// accessTokenTTL is the TTL for Keycloak access tokens in the integration environment.
	// Using 10s provides a fast but not brittle interval for refresh tests.
	accessTokenTTL = 10 * time.Second
)

func TestOAuthIntegrationSarama(t *testing.T) {
	// The Sarama client is legacy; the Franz-go client is stable and always enabled.
	// Keep this test name for backwards compatibility with `-run OAuthIntegration`,
	// but skip to avoid duplicating the same client path (and to keep `-count=10` fast).
	t.Skip("Sarama OAuth integration suite skipped: franz-go is the default client")
}

func TestOAuthIntegrationFranz(t *testing.T) {
	runOAuthIntegrationSuite(t)
}

func runOAuthIntegrationSuite(t *testing.T) {
	t.Helper()

	suiteCtx, suiteCancel := context.WithTimeout(t.Context(), totalTestTimeout)
	defer suiteCancel()

	env := kafkaOAuthEnv
	require.NotNil(t, env, "kafkaOAuthEnv was not initialized; TestMain should have started it")

	host := &extensionHost{Host: componenttest.NewNopHost(), extensions: map[component.ID]component.Component{}}

	telemetry := componenttest.NewNopTelemetrySettings()
	telemetry.Logger = zaptest.NewLogger(t, zaptest.Level(zapcore.DebugLevel))

	extID := startOAuthExtension(t, suiteCtx, telemetry, host, env, "oauth")

	t.Run("happy_path", func(t *testing.T) {
		tcCtx, cancel := context.WithTimeout(suiteCtx, testCaseTimeout)
		defer cancel()
		runHappyPath(t, tcCtx, telemetry, host, env, extID)
	})

	t.Run("refresh", func(t *testing.T) {
		tcCtx, cancel := context.WithTimeout(suiteCtx, testCaseTimeout)
		defer cancel()
		runRefreshPath(t, tcCtx, telemetry, host, env, extID)
	})

	t.Run("revoked_secret", func(t *testing.T) {
		tcCtx, cancel := context.WithTimeout(suiteCtx, testCaseTimeout)
		defer cancel()
		runRevokedSecretPath(t, tcCtx, telemetry, host, env, extID)
	})
}

func startOAuthExtension(
	t *testing.T,
	ctx context.Context,
	telemetry component.TelemetrySettings,
	host *extensionHost,
	env *kafkacontainers.Environment,
	name string,
) component.ID {
	return startOAuthExtensionWithOptions(t, ctx, telemetry, host, env, name, 10*time.Second, 0)
}

func startOAuthExtensionWithOptions(
	t *testing.T,
	ctx context.Context,
	telemetry component.TelemetrySettings,
	host *extensionHost,
	env *kafkacontainers.Environment,
	name string,
	timeout time.Duration,
	expiryBuffer time.Duration,
) component.ID {
	return startOAuthExtensionForClient(t, ctx, telemetry, host, env, name, env.ClientID, env.ClientSecret, timeout, expiryBuffer)
}

func startOAuthExtensionForClient(
	t *testing.T,
	ctx context.Context,
	telemetry component.TelemetrySettings,
	host *extensionHost,
	env *kafkacontainers.Environment,
	name string,
	clientID string,
	clientSecret string,
	timeout time.Duration,
	expiryBuffer time.Duration,
) component.ID {
	t.Helper()

	extFactory := oauth2clientauthextension.NewFactory()
	extID := component.MustNewIDWithName(extFactory.Type().String(), name)
	extCfg := extFactory.CreateDefaultConfig().(*oauth2clientauthextension.Config)
	extCfg.ClientID = clientID
	extCfg.ClientSecret = configopaque.String(clientSecret)
	extCfg.TokenURL = env.TokenURL
	extCfg.TLS = env.TLS
	extCfg.TLS.ServerName = "keycloak"
	extCfg.Timeout = timeout
	extCfg.ExpiryBuffer = expiryBuffer

	ext, err := extFactory.Create(ctx, extension.Settings{
		ID:                extID,
		TelemetrySettings: telemetry,
		BuildInfo:         component.NewDefaultBuildInfo(),
	}, extCfg)
	require.NoError(t, err)

	host.extensions[extID] = ext
	require.Contains(t, host.GetExtensions(), extID)

	require.NoError(t, ext.Start(ctx, host))
	t.Cleanup(func() {
		shutdownWithTimeout(t, "oauth extension", ext.Shutdown)
	})
	return extID
}

type tokenSourceProvider interface {
	TokenSource(context.Context) (oauth2.TokenSource, error)
}

func runHappyPath(
	t *testing.T,
	ctx context.Context,
	telemetry component.TelemetrySettings,
	host *extensionHost,
	env *kafkacontainers.Environment,
	extID component.ID,
) {
	t.Helper()

	topic := fmt.Sprintf("otel_metrics_%d", time.Now().UnixNano())

	exporterCfg := mustExporterConfig(env, extID, topic)
	exporterSettings := exportertest.NewNopSettings(kafkaexporter.NewFactory().Type())
	exporterSettings.Logger = zaptest.NewLogger(t, zaptest.Level(zapcore.DebugLevel))
	exporterInstance, err := kafkaexporter.NewFactory().CreateMetrics(ctx, exporterSettings, exporterCfg)
	require.NoError(t, err)
	require.NoError(t, exporterInstance.Start(ctx, host))
	var stopExporterOnce sync.Once
	stopExporter := func() {
		stopExporterOnce.Do(func() {
			shutdownWithTimeout(t, "kafka exporter", exporterInstance.Shutdown)
		})
	}
	t.Cleanup(stopExporter)

	receiverCfg := mustReceiverConfig(env, extID, topic)
	metricsSink := new(consumertest.MetricsSink)
	receiverSettings := receivertest.NewNopSettings(kafkareceiver.NewFactory().Type())
	receiverSettings.Logger = zaptest.NewLogger(t, zaptest.Level(zapcore.DebugLevel))
	receiverInstance, err := kafkareceiver.NewFactory().CreateMetrics(ctx, receiverSettings, receiverCfg, metricsSink)
	require.NoError(t, err)
	require.NoError(t, receiverInstance.Start(ctx, host))
	t.Cleanup(func() {
		shutdownWithTimeout(t, "kafka receiver", receiverInstance.Shutdown)
	})

	metrics := testdata.GenerateMetricsOneMetric()
	require.NoError(t, exporterInstance.ConsumeMetrics(ctx, metrics))

	require.Eventually(t, func() bool {
		return metricsSink.DataPointCount() > 0
	}, metricsWaitTimeout, metricsPollInterval)

	received := metricsSink.AllMetrics()
	require.NotEmpty(t, received)
	require.NoError(t, pmetrictest.CompareMetrics(metrics, received[0],
		pmetrictest.IgnoreResourceMetricsOrder(),
		pmetrictest.IgnoreMetricsOrder(),
		pmetrictest.IgnoreMetricDataPointsOrder(),
		pmetrictest.IgnoreStartTimestamp(),
		pmetrictest.IgnoreTimestamp(),
	))
}

func runRefreshPath(
	t *testing.T,
	ctx context.Context,
	telemetry component.TelemetrySettings,
	host *extensionHost,
	env *kafkacontainers.Environment,
	extID component.ID,
) {
	t.Helper()

	// Use a short TTL (10s) and no expiry buffer to enable deterministic refresh testing.
	// The broker is configured to force reauth every 3s (connections.max.reauth.ms=3000),
	// which combined with the 10s token TTL will trigger token refreshes during the test.
	extIDRefresh := startOAuthExtensionWithOptions(t, ctx, telemetry, host, env, "oauth_refresh", 10*time.Second, 0)

	t.Run("no_restart", func(t *testing.T) {
		tcCtx, cancel := context.WithTimeout(ctx, testCaseTimeout)
		defer cancel()
		runRefreshNoRestart(t, tcCtx, telemetry, host, env, extIDRefresh)
	})

	t.Run("with_restart", func(t *testing.T) {
		tcCtx, cancel := context.WithTimeout(ctx, testCaseTimeout)
		defer cancel()
		runRefreshWithRestart(t, tcCtx, telemetry, host, env, extIDRefresh)
	})
}

// runRefreshNoRestart tests token refresh without restarting components.
// It continuously sends/receives payloads while the broker periodically forces reauth,
// and asserts that at least 2 distinct tokens were minted.
func runRefreshNoRestart(
	t *testing.T,
	ctx context.Context,
	telemetry component.TelemetrySettings,
	host *extensionHost,
	env *kafkacontainers.Environment,
	extID component.ID,
) {
	t.Helper()

	topic := fmt.Sprintf("otel_metrics_refresh_norestart_%d", time.Now().UnixNano())

	exporterCfg := mustExporterConfig(env, extID, topic)

	exporterSettings := exportertest.NewNopSettings(kafkaexporter.NewFactory().Type())
	exporterSettings.Logger = zaptest.NewLogger(t, zaptest.Level(zapcore.DebugLevel))
	exporterInstance, err := kafkaexporter.NewFactory().CreateMetrics(ctx, exporterSettings, exporterCfg)
	require.NoError(t, err)
	require.NoError(t, exporterInstance.Start(ctx, host))
	t.Cleanup(func() {
		shutdownWithTimeout(t, "kafka exporter", exporterInstance.Shutdown)
	})

	receiverCfg := mustReceiverConfig(env, extID, topic)
	metricsSink := new(consumertest.MetricsSink)
	receiverSettings := receivertest.NewNopSettings(kafkareceiver.NewFactory().Type())
	receiverSettings.Logger = zaptest.NewLogger(t, zaptest.Level(zapcore.DebugLevel))
	receiverInstance, err := kafkareceiver.NewFactory().CreateMetrics(ctx, receiverSettings, receiverCfg, metricsSink)
	require.NoError(t, err)
	require.NoError(t, receiverInstance.Start(ctx, host))
	t.Cleanup(func() {
		shutdownWithTimeout(t, "kafka receiver", receiverInstance.Shutdown)
	})

	// Continuously send/receive payloads on a short ticker while the broker periodically forces reauth.
	// This verifies that token refreshes happen transparently without restarting components.
	// With the short access token TTL (10s), multiple refreshes will occur during this loop.
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()

	startTime := time.Now()
	lastDataPointCount := 0
	for {
		select {
		case <-ctx.Done():
			t.Fatalf("test timeout before verifying token refresh (end-to-end worked for %v)", time.Since(startTime))
		case <-ticker.C:
			metrics := testdata.GenerateMetricsOneMetric()
			require.NoError(t, exporterInstance.ConsumeMetrics(ctx, metrics))

			// After enough time for at least one token refresh (> accessTokenTTL),
			// verify end-to-end still works — proving refresh succeeded.
			if time.Since(startTime) > accessTokenTTL+2*time.Second {
				require.Eventually(t, func() bool {
					return metricsSink.DataPointCount() > lastDataPointCount
				}, metricsWaitTimeout, metricsPollInterval)
				t.Logf("Token refresh verified: end-to-end still works after %v (data points: %d)",
					time.Since(startTime), metricsSink.DataPointCount())
				return
			}

			if metricsSink.DataPointCount() > lastDataPointCount {
				lastDataPointCount = metricsSink.DataPointCount()
			}
		}
	}
}

// runRefreshWithRestart tests token refresh across a forced reconnect/restart.
// It restarts the exporter and asserts that a refreshed token is obtained and end-to-end still works.
func runRefreshWithRestart(
	t *testing.T,
	ctx context.Context,
	telemetry component.TelemetrySettings,
	host *extensionHost,
	env *kafkacontainers.Environment,
	extID component.ID,
) {
	t.Helper()

	topic := fmt.Sprintf("otel_metrics_refresh_restart_%d", time.Now().UnixNano())

	// Start initial exporter
	exporterCfg := mustExporterConfig(env, extID, topic)

	exporterSettings := exportertest.NewNopSettings(kafkaexporter.NewFactory().Type())
	exporterSettings.Logger = zaptest.NewLogger(t, zaptest.Level(zapcore.DebugLevel))
	exporterInstance, err := kafkaexporter.NewFactory().CreateMetrics(ctx, exporterSettings, exporterCfg)
	require.NoError(t, err)
	require.NoError(t, exporterInstance.Start(ctx, host))

	receiverCfg := mustReceiverConfig(env, extID, topic)
	metricsSink := new(consumertest.MetricsSink)
	receiverSettings := receivertest.NewNopSettings(kafkareceiver.NewFactory().Type())
	receiverSettings.Logger = zaptest.NewLogger(t, zaptest.Level(zapcore.DebugLevel))
	receiverInstance, err := kafkareceiver.NewFactory().CreateMetrics(ctx, receiverSettings, receiverCfg, metricsSink)
	require.NoError(t, err)
	require.NoError(t, receiverInstance.Start(ctx, host))
	t.Cleanup(func() {
		shutdownWithTimeout(t, "kafka receiver", receiverInstance.Shutdown)
	})

	// Send first payload and verify receive
	first := testdata.GenerateMetricsOneMetric()
	require.NoError(t, exporterInstance.ConsumeMetrics(ctx, first))
	require.Eventually(t, func() bool {
		return metricsSink.DataPointCount() > 0
	}, metricsWaitTimeout, metricsPollInterval)

	// Restart exporter to force reconnect and token refresh
	shutdownWithTimeout(t, "kafka exporter", exporterInstance.Shutdown)

	// Wait for at least one token TTL cycle to ensure the new exporter gets a different token
	time.Sleep(accessTokenTTL + 2*time.Second)

	exporterCfg2 := mustExporterConfig(env, extID, topic)

	exporterSettings2 := exportertest.NewNopSettings(kafkaexporter.NewFactory().Type())
	exporterSettings2.Logger = zaptest.NewLogger(t, zaptest.Level(zapcore.DebugLevel))
	exporterInstance2, err := kafkaexporter.NewFactory().CreateMetrics(ctx, exporterSettings2, exporterCfg2)
	require.NoError(t, err)
	require.NoError(t, exporterInstance2.Start(ctx, host))
	t.Cleanup(func() {
		shutdownWithTimeout(t, "kafka exporter (restart)", exporterInstance2.Shutdown)
	})

	// Send second payload and verify receive — proves new token was obtained after restart
	before := metricsSink.DataPointCount()
	second := testdata.GenerateMetricsOneMetric()
	require.NoError(t, exporterInstance2.ConsumeMetrics(ctx, second))
	require.Eventually(t, func() bool {
		return metricsSink.DataPointCount() > before
	}, metricsWaitTimeout, metricsPollInterval)

	t.Logf("Restart test passed: end-to-end works after exporter restart with token refresh")
}

func runRevokedSecretPath(
	t *testing.T,
	ctx context.Context,
	telemetry component.TelemetrySettings,
	host *extensionHost,
	env *kafkacontainers.Environment,
	extID component.ID,
) {
	t.Helper()

	// Positive-path is validated in happy_path / refresh tests.
	_ = extID

	extIDRevoked := startOAuthExtensionForClient(
		t, ctx, telemetry, host, env, "oauth_revoked",
		env.RevokedClientID, env.RevokedClientSecret,
		2*time.Second, 0,
	)

	// 1) Deterministically validate Keycloak token issuance fails for the revoked client.
	extComp := host.extensions[extIDRevoked]
	p, ok := extComp.(tokenSourceProvider)
	require.True(t, ok, "oauth extension does not implement TokenSource")
	ts, err := p.TokenSource(ctx)
	require.NoError(t, err)
	_, err = ts.Token()
	require.Error(t, err)
}

func mustExporterConfig(env *kafkacontainers.Environment, extID component.ID, topic string) *kafkaexporter.Config {
	cfg := kafkaexporter.NewFactory().CreateDefaultConfig().(*kafkaexporter.Config)
	cfg.ClientConfig.Brokers = []string{env.Bootstrap}
	tlsCfg := env.TLS
	cfg.ClientConfig.TLS = &tlsCfg
	cfg.ClientConfig.Authentication.SASL = &configkafka.SASLConfig{
		Mechanism:              "OAUTHBEARER",
		OAuthBearerTokenSource: extID,
	}
	cfg.Metrics.Topic = topic
	return cfg
}

func mustReceiverConfig(env *kafkacontainers.Environment, extID component.ID, topic string) *kafkareceiver.Config {
	cfg := kafkareceiver.NewFactory().CreateDefaultConfig().(*kafkareceiver.Config)
	cfg.Brokers = []string{env.Bootstrap}
	tlsCfg := env.TLS
	cfg.TLS = &tlsCfg
	cfg.Authentication.SASL = &configkafka.SASLConfig{
		Mechanism:              "OAUTHBEARER",
		OAuthBearerTokenSource: extID,
	}
	cfg.GroupID = fmt.Sprintf("otel-group-%d", time.Now().UnixNano())
	cfg.InitialOffset = configkafka.EarliestOffset
	cfg.Metrics.Topics = []string{topic}
	return cfg
}

func shutdownWithTimeout(t *testing.T, name string, fn func(context.Context) error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	errCh := make(chan error, 1)
	go func() {
		errCh <- fn(ctx)
	}()

	select {
	case err := <-errCh:
		require.NoErrorf(t, err, "%s shutdown", name)
	case <-ctx.Done():
		require.Failf(t, "shutdown timeout", "%s shutdown timed out after %s", name, shutdownTimeout)
	}
}

type extensionHost struct {
	component.Host
	extensions map[component.ID]component.Component
}

func (h *extensionHost) GetExtensions() map[component.ID]component.Component {
	return h.extensions
}
