// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package proxy provides an http server to act as a signing proxy for SDKs calling AWS X-Ray APIs
package proxy // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/proxy"

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/awsutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/sanitize"
)

const (
	connHeader = "Connection"
)

// Server represents HTTP server.
type Server interface {
	ListenAndServe() error
	Shutdown(ctx context.Context) error
}

// NewServer returns a local TCP server that proxies requests to AWS
// backend using the given credentials.
func NewServer(cfg *Config, logger *zap.Logger) (Server, error) {
	_, err := net.ResolveTCPAddr("tcp", cfg.Endpoint)
	if err != nil {
		return nil, err
	}
	if cfg.ProxyAddress != "" {
		logger.Debug("Using remote proxy", zap.String("address", cfg.ProxyAddress))
	}
	if cfg.ServiceName == "" {
		cfg.ServiceName = "xray"
	}

	ctx := context.Background()
	awsCfg, err := getAWSConfigSession(ctx, cfg, logger)
	if err != nil {
		return nil, err
	}

	var awsEndPoint string
	if cfg.AWSEndpoint != "" {
		awsEndPoint = cfg.AWSEndpoint
	} else {
		awsEndPoint, err = getServiceEndpoint(awsCfg.Region, cfg.ServiceName)
		if err != nil {
			return nil, err
		}
	}

	// Parse url from endpoint
	awsURL, err := url.Parse(awsEndPoint)
	if err != nil {
		return nil, fmt.Errorf("unable to parse AWS service endpoint: %w", err)
	}

	signer := v4.NewSigner()
	defaultCreds := awsCfg.Credentials

	sessionCfg := cfg.toSessionConfig()
	apiRouteMap, credsByRole := buildRoutingMaps(ctx, cfg.AdditionalRoutingRules, cfg.RoleARN, defaultCreds, awsCfg.Region, sessionCfg, logger)

	transport, err := proxyServerTransport(cfg)
	if err != nil {
		return nil, err
	}

	region := awsCfg.Region
	serviceName := cfg.ServiceName

	// Reverse proxy handler
	reverseProxy := &httputil.ReverseProxy{
		Transport: transport,

		Rewrite: func(r *httputil.ProxyRequest) {
			if r.In.URL != nil {
				logger.Debug("Received request on X-Ray receiver TCP proxy server", zap.String("URL", sanitize.URL(r.In.URL)))
			}

			// Strip Connection header before signing. ReverseProxy strips
			// hop-by-hop headers after Rewrite, so signing with it present
			// would produce a signature that doesn't match what's forwarded.
			r.Out.Header.Del(connHeader)

			ruleServiceName := serviceName
			ruleRegion := region
			ruleTargetURL := awsURL
			ruleCreds := defaultCreds

			apiName := strings.TrimPrefix(r.In.URL.Path, "/")
			if rule := apiRouteMap[apiName]; rule != nil {
				if rule.ServiceName != "" {
					ruleServiceName = rule.ServiceName
				}
				if rule.Region != "" {
					ruleRegion = rule.Region
				}
				if rule.RoleARN != "" {
					if roleCreds, ok := credsByRole[rule.RoleARN]; ok {
						ruleCreds = roleCreds
					}
				}
				if rule.AWSEndpoint != "" {
					if parsed, parseErr := url.Parse(rule.AWSEndpoint); parseErr == nil {
						ruleTargetURL = parsed
					} else {
						logger.Error("Unable to parse endpoint", zap.Error(parseErr))
					}
				}
			}

			// Set outbound request URL to the AWS endpoint
			r.Out.URL.Scheme = ruleTargetURL.Scheme
			r.Out.URL.Host = ruleTargetURL.Host
			r.Out.Host = ruleTargetURL.Host

			// Consume body and calculate payload hash for signing
			body, payloadHash, err := consumeBody(r.In.Body)
			if err != nil {
				logger.Error("Unable to consume request body", zap.Error(err))
				return
			}

			// Restore body for the request
			if body != nil {
				r.Out.Body = io.NopCloser(bytes.NewReader(body))
			}

			// Retrieve credentials for signing
			creds, err := ruleCreds.Retrieve(r.Out.Context())
			if err != nil {
				logger.Error("Unable to retrieve credentials", zap.Error(err))
				return
			}

			// Sign request using v4 signer
			err = signer.SignHTTP(r.Out.Context(), creds, r.Out, payloadHash, ruleServiceName, ruleRegion, time.Now())
			if err != nil {
				logger.Error("Unable to sign request", zap.Error(err))
			}
		},
	}

	handler := &proxyHandler{
		proxy:       reverseProxy,
		apiRouteMap: apiRouteMap,
		logger:      logger,
	}

	return &http.Server{
		Addr:              cfg.Endpoint,
		Handler:           handler,
		ReadHeaderTimeout: 20 * time.Second,
	}, nil
}

// proxyHandler wraps a reverse proxy with routing rule validation. Paths
// mapped to nil in apiRouteMap (rules that failed validation at startup)
// are rejected with HTTP 400 before reaching the reverse proxy.
type proxyHandler struct {
	proxy       *httputil.ReverseProxy
	apiRouteMap map[string]*RoutingRule
	logger      *zap.Logger
}

func (h *proxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	apiName := strings.TrimPrefix(r.URL.Path, "/")
	if rule, exists := h.apiRouteMap[apiName]; exists && rule == nil {
		h.logger.Warn("Rejecting request for path with invalid routing rule", zap.String("path", apiName))
		http.Error(w, "invalid routing configuration for path: "+apiName, http.StatusBadRequest)
		return
	}
	h.proxy.ServeHTTP(w, r)
}

// consumeBody reads the body, calculates SHA-256 hash, and returns the body bytes and hash.
// v4.Signer requires a payload hash for signing.
func consumeBody(body io.ReadCloser) ([]byte, string, error) {
	// Return empty hash if body is nil
	if body == nil {
		// SHA-256 of empty string
		return nil, "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", nil
	}

	// Consume body
	buf, err := io.ReadAll(body)
	if err != nil {
		return nil, "", err
	}

	// Calculate SHA-256 hash
	h := sha256.Sum256(buf)
	payloadHash := hex.EncodeToString(h[:])

	return buf, payloadHash, nil
}

// buildRoutingMaps creates maps for routing API requests to their service
// configurations and per-role credentials providers. Invalid rules (missing
// service_name, unresolvable region, unresolvable endpoint, or failed STS
// AssumeRole at startup) are mapped to nil so the proxyHandler can reject
// them with HTTP 400 instead of forwarding unsigned requests.
func buildRoutingMaps(
	ctx context.Context,
	routes []RoutingRule,
	defaultRoleARN string,
	defaultCreds aws.CredentialsProvider,
	defaultRegion string,
	sessionCfg *awsutil.AWSSessionSettings,
	logger *zap.Logger,
) (map[string]*RoutingRule, map[string]aws.CredentialsProvider) {
	apiMap := make(map[string]*RoutingRule)
	credsByRole := make(map[string]aws.CredentialsProvider)
	if defaultRoleARN != "" {
		credsByRole[defaultRoleARN] = defaultCreds
	}

	for i := range routes {
		// Copy by value so we can mutate Region/AWSEndpoint without
		// affecting the caller's input config.
		route := routes[i]
		isValidRoute := true

		if route.ServiceName == "" {
			logger.Warn("Skipping routing rule: service_name is required",
				zap.Int("route_index", i),
				zap.Strings("paths", route.Paths))
			isValidRoute = false
		}
		if isValidRoute && route.Region == "" {
			if defaultRegion != "" {
				route.Region = defaultRegion
			} else {
				logger.Warn("Skipping routing rule: region could not be resolved",
					zap.Int("route_index", i),
					zap.Strings("paths", route.Paths))
				isValidRoute = false
			}
		}

		if isValidRoute && route.AWSEndpoint == "" {
			resolved, err := getServiceEndpoint(route.Region, route.ServiceName)
			if err != nil {
				logger.Warn("Skipping routing rule: failed to auto resolve endpoint",
					zap.Int("route_index", i),
					zap.String("service_name", route.ServiceName),
					zap.String("region", route.Region),
					zap.Error(err))
				isValidRoute = false
			} else {
				route.AWSEndpoint = resolved
			}
		}

		// Build per-role credentials provider on first encounter.
		if isValidRoute && route.RoleARN != "" {
			if _, exists := credsByRole[route.RoleARN]; !exists {
				roleSessionCfg := *sessionCfg
				roleSessionCfg.RoleARN = route.RoleARN
				roleSessionCfg.Region = route.Region
				roleAwsCfg, err := awsutil.GetAWSConfig(ctx, logger, &roleSessionCfg)
				if err != nil {
					logger.Warn("Skipping routing rule: failed to create AWS config for role",
						zap.Int("route_index", i),
						zap.String("role_arn", route.RoleARN),
						zap.Error(err))
					isValidRoute = false
				} else {
					credsByRole[route.RoleARN] = roleAwsCfg.Credentials
				}
			}
		}

		// First rule to register a path wins; later duplicates are ignored.
		for _, path := range route.Paths {
			path = strings.TrimPrefix(path, "/")
			if _, exists := apiMap[path]; !exists {
				if isValidRoute {
					ruleCopy := route
					apiMap[path] = &ruleCopy
				} else {
					apiMap[path] = nil
				}
			}
		}
	}
	return apiMap, credsByRole
}
