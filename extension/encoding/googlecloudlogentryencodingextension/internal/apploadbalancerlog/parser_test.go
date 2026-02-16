// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package apploadbalancerlog

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func int64ptr(i int64) *int64 { return &i }

func boolptr(v bool) *bool { return &v }

func TestHandleRequestMetadata(t *testing.T) {
	tests := []struct {
		name        string
		log         *loadbalancerlog
		expected    map[string]any
		expectedErr string
	}{
		{
			name: "all fields populated",
			log: &loadbalancerlog{
				StatusDetails:              "ok",
				RemoteIP:                   "192.168.1.100",
				BackendTargetProjectNumber: "projects/12345",
				ProxyStatus:                "proxy_ok",
				OverrideResponseCode:       int64ptr(200),
				LoadBalancingScheme:        "EXTERNAL",
				ErrorService:               "error-svc",
				BackendNetworkName:         "default",
				CacheID:                    "ABC",
				CacheDecision:              []string{"CACHE_HIT", "CACHE_MISS"},
			},
			expected: map[string]any{
				"network.peer.address":                     "192.168.1.100",
				gcpLoadBalancingStatusDetails:              "ok",
				gcpLoadBalancingBackendTargetProjectNumber: "projects/12345",
				gcpLoadBalancingProxyStatus:                "proxy_ok",
				gcpLoadBalancingOverrideResponseCode:       int64(200),
				gcpLoadBalancingScheme:                     "EXTERNAL",
				gcpLoadBalancingErrorService:               "error-svc",
				gcpLoadBalancingBackendNetworkName:         "default",
				gcpLoadBalancingCacheID:                    "ABC",
				gcpLoadBalancingCacheDecision:              []any{"CACHE_HIT", "CACHE_MISS"},
			},
		},
		{
			name: "empty cache decisions",
			log: &loadbalancerlog{
				StatusDetails: "ok",
				RemoteIP:      "172.16.0.1",
				CacheDecision: []string{},
			},
			expected: map[string]any{
				"network.peer.address":        "172.16.0.1",
				gcpLoadBalancingStatusDetails: "ok",
			},
		},
		{
			name: "nil override response code",
			log: &loadbalancerlog{
				StatusDetails:        "ok",
				RemoteIP:             "1.2.3.4",
				OverrideResponseCode: nil,
			},
			expected: map[string]any{
				"network.peer.address":        "1.2.3.4",
				gcpLoadBalancingStatusDetails: "ok",
			},
		},
		{
			name: "conflicting remote IP",
			log: &loadbalancerlog{
				StatusDetails: "ok",
				RemoteIP:      "10.0.0.1",
			},
			expected:    map[string]any{},
			expectedErr: "already present with different value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attr := pcommon.NewMap()

			// For the conflicting IP test, pre-populate with a different IP
			if tt.name == "conflicting remote IP" {
				attr.PutStr("network.peer.address", "different-ip")
			}

			err := handleRequestMetadata(tt.log, attr)

			if tt.expectedErr != "" {
				require.ErrorContains(t, err, tt.expectedErr)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expected, attr.AsRaw())
		})
	}
}

func TestHandleAuthPolicyInfo(t *testing.T) {
	tests := []struct {
		name     string
		info     *authPolicyInfo
		expected map[string]any
	}{
		{
			name:     "nil auth policy info",
			info:     nil,
			expected: map[string]any{},
		},
		{
			name: "auth policy with result only",
			info: &authPolicyInfo{
				OverallResult: "ALLOW",
				Policies:      []policyInfo{},
			},
			expected: map[string]any{
				gcpLoadBalancingAuthPolicyInfoResult: "ALLOW",
			},
		},
		{
			name: "auth policy with single policy",
			info: &authPolicyInfo{
				OverallResult: "DENY",
				Policies: []policyInfo{
					{
						Name:    "policy-1",
						Result:  "DENY",
						Details: "blocked by rule",
					},
				},
			},
			expected: map[string]any{
				gcpLoadBalancingAuthPolicyInfoResult: "DENY",
				gcpLoadBalancingAuthPolicyInfoPolicies: []any{
					map[string]any{
						gcpLoadBalancingAuthPolicyName:    "policy-1",
						gcpLoadBalancingAuthPolicyResult:  "DENY",
						gcpLoadBalancingAuthPolicyDetails: "blocked by rule",
					},
				},
			},
		},
		{
			name: "auth policy with multiple policies",
			info: &authPolicyInfo{
				OverallResult: "ALLOW",
				Policies: []policyInfo{
					{
						Name:    "policy-1",
						Result:  "DENY",
						Details: "denied",
					},
					{
						Name:    "policy-2",
						Result:  "ALLOW",
						Details: "allowed",
					},
				},
			},
			expected: map[string]any{
				gcpLoadBalancingAuthPolicyInfoResult: "ALLOW",
				gcpLoadBalancingAuthPolicyInfoPolicies: []any{
					map[string]any{
						gcpLoadBalancingAuthPolicyName:    "policy-1",
						gcpLoadBalancingAuthPolicyResult:  "DENY",
						gcpLoadBalancingAuthPolicyDetails: "denied",
					},
					map[string]any{
						gcpLoadBalancingAuthPolicyName:    "policy-2",
						gcpLoadBalancingAuthPolicyResult:  "ALLOW",
						gcpLoadBalancingAuthPolicyDetails: "allowed",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attr := pcommon.NewMap()
			handleAuthPolicyInfo(tt.info, attr)
			assert.Equal(t, tt.expected, attr.AsRaw())
		})
	}
}

func TestHandleTLSInfo(t *testing.T) {
	tests := []struct {
		name     string
		info     *tlsInfo
		expected map[string]any
	}{
		{
			name:     "nil tls info",
			info:     nil,
			expected: map[string]any{},
		},
		{
			name: "all fields populated",
			info: &tlsInfo{
				EarlyDataRequest: boolptr(true),
				Protocol:         "TLSv1.3",
				Cipher:           "TLS_AES_128_GCM_SHA256",
			},
			expected: map[string]any{
				gcpLoadBalancingTLSInfo: map[string]any{
					gcpLoadBalancingTLSEarlyDataRequest: true,
					"tls.protocol.name":                 "TLSv1.3",
					"tls.cipher":                        "TLS_AES_128_GCM_SHA256",
				},
			},
		},
		{
			name: "early data request false",
			info: &tlsInfo{
				EarlyDataRequest: boolptr(false),
				Protocol:         "TLSv1.2",
				Cipher:           "ECDHE-RSA-AES128-GCM-SHA256",
			},
			expected: map[string]any{
				gcpLoadBalancingTLSInfo: map[string]any{
					gcpLoadBalancingTLSEarlyDataRequest: false,
					"tls.protocol.name":                 "TLSv1.2",
					"tls.cipher":                        "ECDHE-RSA-AES128-GCM-SHA256",
				},
			},
		},
		{
			name: "empty protocol and cipher",
			info: &tlsInfo{
				EarlyDataRequest: boolptr(false),
			},
			expected: map[string]any{
				gcpLoadBalancingTLSInfo: map[string]any{
					gcpLoadBalancingTLSEarlyDataRequest: false,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attr := pcommon.NewMap()
			handleTLSInfo(tt.info, attr)
			assert.Equal(t, tt.expected, attr.AsRaw())
		})
	}
}

func TestHandleMtlsInfo(t *testing.T) {
	tests := []struct {
		name     string
		info     *mtlsInfo
		expected map[string]any
	}{
		{
			name:     "nil mtls info",
			info:     nil,
			expected: map[string]any{},
		},
		{
			name: "all fields populated",
			info: &mtlsInfo{
				ClientCertPresent:           boolptr(true),
				ClientCertChainVerified:     boolptr(true),
				ClientCertError:             "",
				ClientCertSha256Fingerprint: "abc123",
				ClientCertSerialNumber:      "12345",
				ClientCertValidStartTime:    "2024-01-01T00:00:00Z",
				ClientCertValidEndTime:      "2025-01-01T00:00:00Z",
				ClientCertSpiffeID:          "spiffe://example.com/service",
				ClientCertURISans:           "uri:example.com",
				ClientCertDnsnameSans:       "dns:example.com",
				ClientCertIssuerDn:          "CN=CA",
				ClientCertSubjectDn:         "CN=client",
				ClientCertLeaf:              "-----BEGIN CERTIFICATE-----",
				ClientCertChain:             "-----BEGIN CERTIFICATE----- chain",
			},
			expected: map[string]any{
				gcpLoadBalancingMtlsInfo: map[string]any{
					gcpLoadBalancingMtlsClientCertPresent:       true,
					gcpLoadBalancingMtlsClientCertChainVerified: true,
					"tls.client.hash.sha256":                    "abc123",
					gcpLoadBalancingMtlsClientCertSerialNumber:  "12345",
					"tls.client.not_before":                     "2024-01-01T00:00:00Z",
					"tls.client.not_after":                      "2025-01-01T00:00:00Z",
					gcpLoadBalancingMtlsClientCertSpiffeID:      "spiffe://example.com/service",
					gcpLoadBalancingMtlsClientCertURISans:       "uri:example.com",
					gcpLoadBalancingMtlsClientCertDnsnameSans:   "dns:example.com",
					"tls.client.issuer":                         "CN=CA",
					"tls.client.subject":                        "CN=client",
					gcpLoadBalancingMtlsClientCertLeaf:          "-----BEGIN CERTIFICATE-----",
					"tls.client.certificate_chain":              "-----BEGIN CERTIFICATE----- chain",
				},
			},
		},
		{
			name: "cert present but not verified",
			info: &mtlsInfo{
				ClientCertPresent:       boolptr(true),
				ClientCertChainVerified: boolptr(false),
				ClientCertError:         "cert not verified",
			},
			expected: map[string]any{
				gcpLoadBalancingMtlsInfo: map[string]any{
					gcpLoadBalancingMtlsClientCertPresent:       true,
					gcpLoadBalancingMtlsClientCertChainVerified: false,
					gcpLoadBalancingMtlsClientCertError:         "cert not verified",
				},
			},
		},
		{
			name: "cert not present",
			info: &mtlsInfo{
				ClientCertPresent: boolptr(false),
			},
			expected: map[string]any{
				gcpLoadBalancingMtlsInfo: map[string]any{
					gcpLoadBalancingMtlsClientCertPresent: false,
				},
			},
		},
		{
			name: "nil boolean fields",
			info: &mtlsInfo{
				ClientCertSha256Fingerprint: "fingerprint",
			},
			expected: map[string]any{
				gcpLoadBalancingMtlsInfo: map[string]any{
					"tls.client.hash.sha256": "fingerprint",
				},
			},
		},
		{
			name: "partial fields",
			info: &mtlsInfo{
				ClientCertPresent:      boolptr(true),
				ClientCertSerialNumber: "67890",
				ClientCertSpiffeID:     "spiffe://test.com/app",
			},
			expected: map[string]any{
				gcpLoadBalancingMtlsInfo: map[string]any{
					gcpLoadBalancingMtlsClientCertPresent:      true,
					gcpLoadBalancingMtlsClientCertSerialNumber: "67890",
					gcpLoadBalancingMtlsClientCertSpiffeID:     "spiffe://test.com/app",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attr := pcommon.NewMap()
			handleMtlsInfo(tt.info, attr)
			assert.Equal(t, tt.expected, attr.AsRaw())
		})
	}
}

func TestIsValid(t *testing.T) {
	tests := []struct {
		name    string
		log     *loadbalancerlog
		wantErr bool
	}{
		{
			name: "valid log type",
			log: &loadbalancerlog{
				Type: loadBalancerLogType,
			},
			wantErr: false,
		},
		{
			name: "invalid log type",
			log: &loadbalancerlog{
				Type: "type.googleapis.com/invalid.type",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := isValid(tt.log)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestParsePayloadIntoAttributes(t *testing.T) {
	tests := []struct {
		name       string
		payload    string
		expected   map[string]any
		expectsErr string
	}{
		{
			name:       "invalid json",
			payload:    `{invalid json}`,
			expectsErr: "failed to unmarshal Load Balancer log",
		},
		{
			name: "invalid log type",
			payload: `{
				"@type": "invalid-type",
				"statusDetails": "ok"
			}`,
			expectsErr: "expected @type to be",
		},
		{
			name: "minimal load balancer log without armor fields",
			payload: `{
				"@type": "type.googleapis.com/google.cloud.loadbalancing.type.LoadBalancerLogEntry",
				"statusDetails": "ok",
				"remoteIp": "192.168.1.1"
			}`,
			expected: map[string]any{
				gcpLoadBalancingStatusDetails: "ok",
				"network.peer.address":        "192.168.1.1",
			},
		},
		{
			name: "load balancer log with request metadata",
			payload: `{
				"@type": "type.googleapis.com/google.cloud.loadbalancing.type.LoadBalancerLogEntry",
				"statusDetails": "ok",
				"remoteIp": "10.0.0.1",
				"backendTargetProjectNumber": "projects/12345",
				"proxyStatus": "proxy_ok",
				"overrideResponseCode": 301,
				"loadBalancingScheme": "EXTERNAL",
				"errorService": "error-svc",
				"backendNetworkName": "vpc-network",
				"cacheDecision": ["CACHE_HIT", "CACHE_REVALIDATED"]
			}`,
			expected: map[string]any{
				gcpLoadBalancingStatusDetails:              "ok",
				"network.peer.address":                     "10.0.0.1",
				gcpLoadBalancingBackendTargetProjectNumber: "projects/12345",
				gcpLoadBalancingProxyStatus:                "proxy_ok",
				gcpLoadBalancingOverrideResponseCode:       int64(301),
				gcpLoadBalancingScheme:                     "EXTERNAL",
				gcpLoadBalancingErrorService:               "error-svc",
				gcpLoadBalancingBackendNetworkName:         "vpc-network",
				gcpLoadBalancingCacheDecision:              []any{"CACHE_HIT", "CACHE_REVALIDATED"},
			},
		},
		{
			name: "load balancer log with auth policy info",
			payload: `{
				"@type": "type.googleapis.com/google.cloud.loadbalancing.type.LoadBalancerLogEntry",
				"statusDetails": "ok",
				"remoteIp": "172.16.0.1",
				"authPolicyInfo": {
					"result": "DENY",
					"policies": [
						{
							"name": "policy-1",
							"result": "DENY",
							"details": "blocked"
						}
					]
				}
			}`,
			expected: map[string]any{
				gcpLoadBalancingStatusDetails:        "ok",
				"network.peer.address":               "172.16.0.1",
				gcpLoadBalancingAuthPolicyInfoResult: "DENY",
				gcpLoadBalancingAuthPolicyInfoPolicies: []any{
					map[string]any{
						gcpLoadBalancingAuthPolicyName:    "policy-1",
						gcpLoadBalancingAuthPolicyResult:  "DENY",
						gcpLoadBalancingAuthPolicyDetails: "blocked",
					},
				},
			},
		},
		{
			name: "load balancer log with tls info",
			payload: `{
				"@type": "type.googleapis.com/google.cloud.loadbalancing.type.LoadBalancerLogEntry",
				"statusDetails": "ok",
				"remoteIp": "1.2.3.4",
				"tls": {
					"earlyDataRequest": true,
					"protocol": "TLSv1.3",
					"cipher": "TLS_AES_128_GCM_SHA256"
				}
			}`,
			expected: map[string]any{
				gcpLoadBalancingStatusDetails: "ok",
				"network.peer.address":        "1.2.3.4",
				gcpLoadBalancingTLSInfo: map[string]any{
					gcpLoadBalancingTLSEarlyDataRequest: true,
					"tls.protocol.name":                 "TLSv1.3",
					"tls.cipher":                        "TLS_AES_128_GCM_SHA256",
				},
			},
		},
		{
			name: "load balancer log with mtls info",
			payload: `{
				"@type": "type.googleapis.com/google.cloud.loadbalancing.type.LoadBalancerLogEntry",
				"statusDetails": "ok",
				"remoteIp": "5.6.7.8",
				"mtls": {
					"clientCertPresent": true,
					"clientCertChainVerified": true,
					"clientCertSha256Fingerprint": "abc123",
					"clientCertSerialNumber": "12345"
				}
			}`,
			expected: map[string]any{
				gcpLoadBalancingStatusDetails: "ok",
				"network.peer.address":        "5.6.7.8",
				gcpLoadBalancingMtlsInfo: map[string]any{
					gcpLoadBalancingMtlsClientCertPresent:       true,
					gcpLoadBalancingMtlsClientCertChainVerified: true,
					"tls.client.hash.sha256":                    "abc123",
					gcpLoadBalancingMtlsClientCertSerialNumber:  "12345",
				},
			},
		},
		{
			name: "conflicting remote IPs between metadata and armor fields",
			payload: `{
				"@type": "type.googleapis.com/google.cloud.loadbalancing.type.LoadBalancerLogEntry",
				"statusDetails": "denied_by_security_policy",
				"remoteIp": "192.168.1.1",
				"enforcedSecurityPolicy": {
					"name": "test-policy",
					"priority": 1,
					"configuredAction": "DENY",
					"outcome": "DENY"
				},
				"securityPolicyRequestData": {
					"remoteIpInfo": {
						"ipAddress": "10.0.0.1"
					}
				}
			}`,
			expectsErr: "already present with different value",
		},
		{
			name: "preview edge security policy",
			payload: `{
				"@type": "type.googleapis.com/google.cloud.loadbalancing.type.LoadBalancerLogEntry",
				"statusDetails": "denied_by_security_policy",
				"remoteIp": "1.1.1.1",
				"previewEdgeSecurityPolicy": {
					"name": "edge-policy",
					"priority": 5,
					"configuredAction": "ALLOW",
					"outcome": "ACCEPT"
				}
			}`,
			expected: map[string]any{
				gcpLoadBalancingStatusDetails: "denied_by_security_policy",
				"network.peer.address":        "1.1.1.1",
				gcpArmorSecurityPolicyPreviewEdge: map[string]any{
					gcpArmorSecurityPolicyName:             "edge-policy",
					gcpArmorSecurityPolicyPriority:         int64(5),
					gcpArmorSecurityPolicyConfiguredAction: "ALLOW",
					gcpArmorSecurityPolicyOutcome:          "ACCEPT",
				},
			},
		},
		{
			name: "enforced edge security policy",
			payload: `{
				"@type": "type.googleapis.com/google.cloud.loadbalancing.type.LoadBalancerLogEntry",
				"statusDetails": "denied_by_security_policy",
				"remoteIp": "2.2.2.2",
				"enforcedEdgeSecurityPolicy": {
					"name": "edge-policy-enforced",
					"priority": 10,
					"configuredAction": "DENY",
					"outcome": "DENY"
				}
			}`,
			expected: map[string]any{
				gcpLoadBalancingStatusDetails: "denied_by_security_policy",
				"network.peer.address":        "2.2.2.2",
				gcpArmorSecurityPolicyEnforcedEdge: map[string]any{
					gcpArmorSecurityPolicyName:             "edge-policy-enforced",
					gcpArmorSecurityPolicyPriority:         int64(10),
					gcpArmorSecurityPolicyConfiguredAction: "DENY",
					gcpArmorSecurityPolicyOutcome:          "DENY",
				},
			},
		},
		{
			name: "preview security policy with extended fields",
			payload: `{
				"@type": "type.googleapis.com/google.cloud.loadbalancing.type.LoadBalancerLogEntry",
				"statusDetails": "denied_by_security_policy",
				"remoteIp": "3.3.3.3",
				"previewSecurityPolicy": {
					"name": "preview-policy",
					"priority": 20,
					"configuredAction": "DENY",
					"outcome": "DENY",
					"preconfiguredExprIds": ["expr1", "expr2"],
					"threatIntelligence": {
						"categories": ["botnet"]
					}
				}
			}`,
			expected: map[string]any{
				gcpLoadBalancingStatusDetails: "denied_by_security_policy",
				"network.peer.address":        "3.3.3.3",
				gcpArmorSecurityPolicyPreview: map[string]any{
					gcpArmorSecurityPolicyName:             "preview-policy",
					gcpArmorSecurityPolicyPriority:         int64(20),
					gcpArmorSecurityPolicyConfiguredAction: "DENY",
					gcpArmorSecurityPolicyOutcome:          "DENY",
					gcpArmorWAFRuleExpressionIDs:           []any{"expr1", "expr2"},
					gcpArmorThreatIntelligenceCategories:   []any{"botnet"},
				},
			},
		},
		{
			name: "enforced security policy with all fields",
			payload: `{
				"@type": "type.googleapis.com/google.cloud.loadbalancing.type.LoadBalancerLogEntry",
				"statusDetails": "denied_by_security_policy",
				"remoteIp": "4.4.4.4",
				"enforcedSecurityPolicy": {
					"name": "enforced-policy",
					"priority": 30,
					"configuredAction": "DENY",
					"outcome": "DENY",
					"rateLimitAction": {
						"key": "rate-key",
						"outcome": "RATE_LIMIT_THRESHOLD_EXCEED"
					},
					"preconfiguredExprIds": ["expr1"],
					"threatIntelligence": {
						"categories": ["malware", "phishing"]
					},
					"addressGroup": {
						"names": ["group1", "group2"]
					},
					"adaptiveProtection": {
						"autoDeployAlertId": "alert-123"
					}
				}
			}`,
			expected: map[string]any{
				gcpLoadBalancingStatusDetails: "denied_by_security_policy",
				"network.peer.address":        "4.4.4.4",
				gcpArmorSecurityPolicyEnforced: map[string]any{
					gcpArmorSecurityPolicyName:                  "enforced-policy",
					gcpArmorSecurityPolicyPriority:              int64(30),
					gcpArmorSecurityPolicyConfiguredAction:      "DENY",
					gcpArmorSecurityPolicyOutcome:               "DENY",
					gcpArmorRateLimitActionKey:                  "rate-key",
					gcpArmorRateLimitActionOutcome:              "RATE_LIMIT_THRESHOLD_EXCEED",
					gcpArmorWAFRuleExpressionIDs:                []any{"expr1"},
					gcpArmorThreatIntelligenceCategories:        []any{"malware", "phishing"},
					gcpArmorAddressGroupNames:                   []any{"group1", "group2"},
					gcpArmorAdaptiveProtectionAutoDeployAlertID: "alert-123",
				},
			},
		},
		{
			name: "complete load balancer log with armor and security policy request data",
			payload: `{
				"@type": "type.googleapis.com/google.cloud.loadbalancing.type.LoadBalancerLogEntry",
				"statusDetails": "denied_by_security_policy",
				"remoteIp": "1.2.3.4",
				"backendTargetProjectNumber": "projects/987654",
				"loadBalancingScheme": "EXTERNAL",
				"cacheDecision": ["CACHE_MISS"],
				"tls": {
					"earlyDataRequest": false,
					"protocol": "TLSv1.2",
					"cipher": "ECDHE-RSA-AES128-GCM-SHA256"
				},
				"securityPolicyRequestData": {
					"recaptchaActionToken": { "score": 0.9 },
					"userIpInfo": { "source": "X-Forwarded-For", "ipAddress": "5.6.7.8" },
					"tlsJa3Fingerprint": "ja3-fingerprint",
					"tlsJa4Fingerprint": "ja4-fingerprint"
				},
				"enforcedSecurityPolicy": {
					"name": "complete-policy",
					"priority": 100,
					"configuredAction": "DENY",
					"outcome": "DENY"
				}
			}`,
			expected: map[string]any{
				gcpLoadBalancingStatusDetails:              "denied_by_security_policy",
				"network.peer.address":                     "1.2.3.4",
				gcpLoadBalancingBackendTargetProjectNumber: "projects/987654",
				gcpLoadBalancingScheme:                     "EXTERNAL",
				gcpLoadBalancingCacheDecision:              []any{"CACHE_MISS"},
				gcpLoadBalancingTLSInfo: map[string]any{
					gcpLoadBalancingTLSEarlyDataRequest: false,
					"tls.protocol.name":                 "TLSv1.2",
					"tls.cipher":                        "ECDHE-RSA-AES128-GCM-SHA256",
				},
				gcpArmorRecaptchaActionTokenScore: float64(0.9),
				gcpArmorUserIPInfoSource:          "X-Forwarded-For",
				"client.address":                  "5.6.7.8",
				"tls.client.ja3":                  "ja3-fingerprint",
				gcpArmorTLSJa4Fingerprint:         "ja4-fingerprint",
				gcpArmorSecurityPolicyEnforced: map[string]any{
					gcpArmorSecurityPolicyName:             "complete-policy",
					gcpArmorSecurityPolicyPriority:         int64(100),
					gcpArmorSecurityPolicyConfiguredAction: "DENY",
					gcpArmorSecurityPolicyOutcome:          "DENY",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attr := pcommon.NewMap()
			err := ParsePayloadIntoAttributes([]byte(tt.payload), attr)

			if tt.expectsErr != "" {
				require.ErrorContains(t, err, tt.expectsErr)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.expected, attr.AsRaw())
		})
	}
}
