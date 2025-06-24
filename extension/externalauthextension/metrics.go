package externalauthextension

import (
	"strings"

	"go.opentelemetry.io/otel/metric"
)

type authMetrics struct {
	authAttempts    metric.Int64Counter
	authSuccesses   metric.Int64Counter
	authFailures    metric.Int64Counter
	authLatency     metric.Float64Histogram
	cacheHits       metric.Int64Counter
	cacheMisses     metric.Int64Counter
	remoteAuthCalls metric.Int64Counter
}

func extractUserBasicAuthHeader(authHeader string) string {
	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		return ""
	}
	credentials := strings.Split(parts[1], ":")
	if len(credentials) != 2 {
		return ""
	}
	return credentials[0]
}

func newAuthMetrics(meter metric.Meter) (*authMetrics, error) {
	authAttempts, err := meter.Int64Counter(
		"externalauth_attempts_total",
		metric.WithDescription("Total number of authentication attempts"),
	)
	if err != nil {
		return nil, err
	}

	authSuccesses, err := meter.Int64Counter(
		"externalauth_successes_total",
		metric.WithDescription("Total number of successful authentications"),
	)
	if err != nil {
		return nil, err
	}

	authFailures, err := meter.Int64Counter(
		"externalauth_failures_total",
		metric.WithDescription("Total number of failed authentications"),
	)
	if err != nil {
		return nil, err
	}

	authLatency, err := meter.Float64Histogram(
		"externalauth_latency_seconds",
		metric.WithDescription("Authentication latency in seconds"),
	)
	if err != nil {
		return nil, err
	}

	cacheHits, err := meter.Int64Counter(
		"externalauth_cache_hits_total",
		metric.WithDescription("Total number of cache hits"),
	)
	if err != nil {
		return nil, err
	}

	cacheMisses, err := meter.Int64Counter(
		"externalauth_cache_misses_total",
		metric.WithDescription("Total number of cache misses"),
	)
	if err != nil {
		return nil, err
	}

	remoteAuthCalls, err := meter.Int64Counter(
		"externalauth_remote_calls_total",
		metric.WithDescription("Total number of remote authentication calls"),
	)
	if err != nil {
		return nil, err
	}

	return &authMetrics{
		authAttempts:    authAttempts,
		authSuccesses:   authSuccesses,
		authFailures:    authFailures,
		authLatency:     authLatency,
		cacheHits:       cacheHits,
		cacheMisses:     cacheMisses,
		remoteAuthCalls: remoteAuthCalls,
	}, nil
}
