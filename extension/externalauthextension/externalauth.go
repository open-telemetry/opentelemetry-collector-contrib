package externalauthextension

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"go.opentelemetry.io/collector/component"
	auth "go.opentelemetry.io/collector/extension/extensionauth"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

var _ auth.Server = (*externalauth)(nil)

var (
	errNoAuthHeader      = errors.New("no authorization header provided")
	errInvalidAuthHeader = errors.New("invalid authorization header provided")
	errUnauthorized      = errors.New("unauthorized")
	errSendRequest       = errors.New("error sending request")

	DefaultAuthorizationHeader = http.CanonicalHeaderKey("Authorization")
)

type externalauth struct {
	endpoint              string                      // Default endpoint for authentication
	headerEndpointMapping []HeaderMapping             // Maps header values to different endpoints with guaranteed order
	refreshInterval       string                      // How long cached tokens remain valid
	header                string                      // Header name to extract token from
	expectedCodes         []int                       // HTTP status codes indicating successful auth
	scheme                string                      // Authentication scheme (e.g., "Bearer")
	method                string                      // HTTP method for auth requests
	tokenCache            *TokenCache                 // Local cache for token validation results
	telemetry             component.TelemetrySettings // Logging and metrics
	httpClientTimeout     time.Duration               // Timeout for HTTP requests
	client                *http.Client                // HTTP client for external auth calls
	metrics               *authMetrics                // Metrics for monitoring auth performance
	telemetryType         string                      // Type of telemetry (traces/metrics/logs)
	tokenFormat           string                      // Token format (raw/basic_auth)
}

// Creates a new external authentication instance with the given configuration
func newExternalAuth(cfg *Config, telemetry component.TelemetrySettings) (*externalauth, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	metrics, err := newAuthMetrics(telemetry.MeterProvider.Meter("externalauth"))
	if err != nil {
		return nil, err
	}

	ce := &externalauth{
		endpoint:              cfg.Endpoint,
		headerEndpointMapping: cfg.HeaderEndpointMapping,
		refreshInterval:       cfg.RefreshInterval,
		header:                cfg.Header,
		expectedCodes:         cfg.ExpectedCodes,
		scheme:                cfg.Scheme,
		method:                cfg.Method,
		tokenCache:            newTokenCache(),
		telemetry:             telemetry,
		metrics:               metrics,
		client: &http.Client{
			Timeout: cfg.HTTPClientTimeout,
		},
		telemetryType: cfg.TelemetryType,
		tokenFormat:   cfg.TokenFormat,
	}
	return ce, nil
}

func (b *externalauth) Shutdown(context.Context) error {
	b.telemetry.Logger.Info("Shutting down externalauth")
	return nil
}

func (b *externalauth) Start(ctx context.Context, host component.Host) error {
	b.telemetry.Logger.Info("Starting externalauth")
	b.client = &http.Client{
		Timeout: b.httpClientTimeout,
	}
	return nil
}

// Makes an HTTP request to the external authentication service to validate the token
func (b *externalauth) remoteServerAuthenticate(token string, telemetryType string, user string, headers map[string][]string) int {
	b.telemetry.Logger.Debug("Attempting remote server authentication")
	authHeader := fmt.Sprintf("%s %s", b.scheme, token)

	b.metrics.remoteAuthCalls.Add(context.Background(), 1, metric.WithAttributes(
		b.buildConditionalUserAttributes(user, telemetryType, 0)...,
	))

	// Determine the endpoint to use based on headers
	endpoint := b.getEndpointForHeaders(headers)
	b.telemetry.Logger.Debug(fmt.Sprintf("Using endpoint: %s", endpoint))

	request, err := http.NewRequest(b.method, endpoint, nil)
	if err != nil {
		b.telemetry.Logger.Error("Failed to create request")
		return http.StatusInternalServerError
	}
	request.Header.Set(b.header, authHeader)

	b.telemetry.Logger.Debug("Sending authentication request")
	res, err := b.client.Do(request)

	if err != nil {
		b.telemetry.Logger.Error("Failed to send authentication request")
		return http.StatusInternalServerError
	}

	b.telemetry.Logger.Debug(fmt.Sprintf("Received authentication response with status: %d", res.StatusCode))
	for _, code := range b.expectedCodes {
		if res.StatusCode == code {
			b.telemetry.Logger.Debug(fmt.Sprintf("Authentication successful with status: %d", res.StatusCode))
			return http.StatusOK
		}
	}
	b.telemetry.Logger.Debug(fmt.Sprintf("Authentication failed with status: %d", res.StatusCode))
	return http.StatusUnauthorized
}

// Returns the appropriate endpoint based on header values for dynamic routing
func (b *externalauth) getEndpointForHeaders(headers map[string][]string) string {
	if b.headerEndpointMapping == nil {
		return b.endpoint
	}

	// Convert headers to canonical form for lookup
	canonicalHeaders := make(map[string][]string)
	for k, v := range headers {
		canonicalHeaders[http.CanonicalHeaderKey(k)] = v
	}

	// Check each header in the mapping
	for _, mapping := range b.headerEndpointMapping {
		canonicalHeaderName := http.CanonicalHeaderKey(mapping.Header)
		if headerValues, exists := canonicalHeaders[canonicalHeaderName]; exists && len(headerValues) > 0 {
			headerValue := headerValues[0]
			if endpoint, found := mapping.Values[headerValue]; found {
				b.telemetry.Logger.Debug(fmt.Sprintf("Found endpoint mapping for header %s=%s: %s", mapping.Header, headerValue, endpoint))
				return endpoint
			}
		}
	}

	// Fall back to default endpoint if no mapping found
	b.telemetry.Logger.Debug(fmt.Sprintf("No endpoint mapping found, using default: %s", b.endpoint))
	return b.endpoint
}

// Creates metric attributes for monitoring with telemetry type, status code, and user info
func (b *externalauth) buildConditionalUserAttributes(user string, telemetryType string, code int) []attribute.KeyValue {
	attrs := []attribute.KeyValue{
		attribute.String("telemetry_type", telemetryType),
		attribute.Int("code", code),
	}
	if user != "" {
		attrs = append(attrs, attribute.String("user", user))
	}
	return attrs
}

// Main authentication entry point implementing caching and dynamic endpoint routing
func (b *externalauth) Authenticate(ctx context.Context, headers map[string][]string) (context.Context, error) {
	start := time.Now()

	// Convert headers to canonical form for consistent lookup
	canonicalHeaders := make(map[string][]string)
	for k, v := range headers {
		canonicalHeaders[http.CanonicalHeaderKey(k)] = v
	}

	b.telemetry.Logger.Debug("Starting server authentication")

	// Extract and validate the authorization header
	canonicalHeader := http.CanonicalHeaderKey(b.header)

	potentialAuthorization, ok := canonicalHeaders[canonicalHeader]
	if !ok {
		b.telemetry.Logger.Debug("No authorization header found")
		b.metrics.authFailures.Add(ctx, 1, metric.WithAttributes(
			b.buildConditionalUserAttributes("", b.telemetryType, http.StatusUnauthorized)...,
		))
		return nil, errNoAuthHeader
	}
	if len(potentialAuthorization) != 1 {
		b.telemetry.Logger.Debug("Invalid number of authorization headers")
		b.metrics.authFailures.Add(ctx, 1, metric.WithAttributes(
			b.buildConditionalUserAttributes("", b.telemetryType, http.StatusBadRequest)...,
		))
		return nil, errInvalidAuthHeader
	}

	if potentialAuthorization[0] == "" {
		b.telemetry.Logger.Debug("Empty authorization header value")
		b.metrics.authFailures.Add(ctx, 1, metric.WithAttributes(
			b.buildConditionalUserAttributes("", b.telemetryType, http.StatusBadRequest)...,
		))
		return nil, errInvalidAuthHeader
	}
	b.telemetry.Logger.Debug("Authorization header validation passed")

	authHeader := potentialAuthorization[0]

	// Extract user information if using basic auth format
	var user string
	if b.tokenFormat == "basic_auth" {
		user = extractUserBasicAuthHeader(authHeader)
	} else {
		user = ""
	}

	telemetryType := b.telemetryType

	b.metrics.authAttempts.Add(ctx, 1, metric.WithAttributes(
		b.buildConditionalUserAttributes(user, telemetryType, 0)...,
	))

	// Record authentication latency when function returns
	defer func() {
		duration := time.Since(start).Seconds()
		b.metrics.authLatency.Record(ctx, duration, metric.WithAttributes(
			b.buildConditionalUserAttributes(user, telemetryType, 0)...,
		))
	}()

	// Parse the authorization header to extract the token
	potentialAuthorizationSegments := strings.Split(authHeader, " ")
	if len(potentialAuthorizationSegments) != 2 {
		b.telemetry.Logger.Debug("Invalid authorization header format")
		b.metrics.authFailures.Add(ctx, 1, metric.WithAttributes(
			b.buildConditionalUserAttributes(user, telemetryType, http.StatusBadRequest)...,
		))
		return nil, errInvalidAuthHeader
	}
	if potentialAuthorizationSegments[0] != b.scheme {
		b.telemetry.Logger.Debug("Invalid authorization scheme")
		b.metrics.authFailures.Add(ctx, 1, metric.WithAttributes(
			b.buildConditionalUserAttributes(user, telemetryType, http.StatusBadRequest)...,
		))
		return nil, errInvalidAuthHeader
	}
	auth := potentialAuthorizationSegments[1]
	if auth == "" {
		b.telemetry.Logger.Debug("Empty authorization token")
		b.metrics.authFailures.Add(ctx, 1, metric.WithAttributes(
			b.buildConditionalUserAttributes(user, telemetryType, http.StatusBadRequest)...,
		))
		return nil, errInvalidAuthHeader
	}

	b.telemetry.Logger.Debug("Checking token cache")

	// Cache Miss: Token not found in cache
	if !b.tokenCache.tokenExists(auth) {
		b.telemetry.Logger.Debug("Token not found in cache, attempting remote authentication")
		b.metrics.cacheMisses.Add(ctx, 1, metric.WithAttributes(
			b.buildConditionalUserAttributes(user, telemetryType, 0)...,
		))
		status := b.remoteServerAuthenticate(auth, telemetryType, user, headers)
		if status == http.StatusOK {
			b.telemetry.Logger.Debug("Remote authentication successful, adding token to cache")
			b.tokenCache.addToken(auth, true)
			b.metrics.authSuccesses.Add(ctx, 1, metric.WithAttributes(
				b.buildConditionalUserAttributes(user, telemetryType, status)...,
			))
			return ctx, nil
		}
		b.telemetry.Logger.Debug(fmt.Sprintf("Remote authentication failed, caching invalid token with status: %d", status))
		b.tokenCache.addToken(auth, false)
		b.metrics.authFailures.Add(ctx, 1, metric.WithAttributes(
			b.buildConditionalUserAttributes(user, telemetryType, status)...,
		))
		if status == http.StatusInternalServerError {
			return nil, errSendRequest
		}
		return nil, errUnauthorized
	}

	// Cache Hit - Expired Token: Token exists but has expired
	if b.tokenCache.isTokenExpired(auth, b.refreshInterval) {
		b.telemetry.Logger.Debug("Token expired, attempting remote authentication")
		b.metrics.cacheMisses.Add(ctx, 1, metric.WithAttributes(
			b.buildConditionalUserAttributes(user, telemetryType, 0)...,
		))
		status := b.remoteServerAuthenticate(auth, telemetryType, user, headers)
		if status == http.StatusOK {
			b.telemetry.Logger.Debug("Remote authentication successful for expired token")
			b.metrics.authSuccesses.Add(ctx, 1, metric.WithAttributes(
				b.buildConditionalUserAttributes(user, telemetryType, status)...,
			))
			return ctx, nil
		}
		b.telemetry.Logger.Debug(fmt.Sprintf("Remote authentication failed for expired token, invalidating cache with status: %d", status))
		b.tokenCache.invalidateToken(auth)
		b.metrics.authFailures.Add(ctx, 1, metric.WithAttributes(
			b.buildConditionalUserAttributes(user, telemetryType, status)...,
		))
		if status == http.StatusInternalServerError {
			return nil, errSendRequest
		}
		return nil, errUnauthorized
	}

	// Cache Hit - Valid Token: Token exists and is still valid
	if b.tokenCache.isTokenValid(auth) {
		b.telemetry.Logger.Debug("Using cached valid token")
		b.metrics.cacheHits.Add(ctx, 1, metric.WithAttributes(
			b.buildConditionalUserAttributes(user, telemetryType, 0)...,
		))
		b.metrics.authSuccesses.Add(ctx, 1, metric.WithAttributes(
			b.buildConditionalUserAttributes(user, telemetryType, http.StatusOK)...,
		))
		return ctx, nil
	}

	// Cache Hit - Invalid Token: Token exists but is marked as invalid
	b.telemetry.Logger.Debug("Token found but invalid")
	b.metrics.authFailures.Add(ctx, 1, metric.WithAttributes(
		b.buildConditionalUserAttributes(user, telemetryType, http.StatusUnauthorized)...,
	))
	return nil, errUnauthorized
}
