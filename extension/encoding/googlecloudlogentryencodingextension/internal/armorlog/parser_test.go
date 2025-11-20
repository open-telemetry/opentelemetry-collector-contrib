// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package armorlog

import (
	"testing"

	gojson "github.com/goccy/go-json"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
)

func int64ptr(i int64) *int64 { return &i }

func TestContainsSecurityPolicyFields(t *testing.T) {
	tests := []struct {
		name     string
		payload  string
		expected bool
	}{
		{
			name:     "contains enforcedSecurityPolicy",
			payload:  `{"@type":"type.googleapis.com/google.cloud.loadbalancing.type.LoadBalancerLogEntry","enforcedSecurityPolicy":{"name":"test"}}`,
			expected: true,
		},
		{
			name:     "contains previewSecurityPolicy",
			payload:  `{"@type":"type.googleapis.com/google.cloud.loadbalancing.type.LoadBalancerLogEntry","previewSecurityPolicy":{"name":"test"}}`,
			expected: true,
		},
		{
			name:     "contains enforcedEdgeSecurityPolicy",
			payload:  `{"@type":"type.googleapis.com/google.cloud.loadbalancing.type.LoadBalancerLogEntry","enforcedEdgeSecurityPolicy":{"name":"test"}}`,
			expected: true,
		},
		{
			name:     "contains previewEdgeSecurityPolicy",
			payload:  `{"@type":"type.googleapis.com/google.cloud.loadbalancing.type.LoadBalancerLogEntry","previewEdgeSecurityPolicy":{"name":"test"}}`,
			expected: true,
		},
		{
			name:     "no security policy fields",
			payload:  `{"@type":"type.googleapis.com/google.cloud.loadbalancing.type.LoadBalancerLogEntry","someOtherField":"value"}`,
			expected: false,
		},
		{
			name:     "empty JSON",
			payload:  `{}`,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ContainsSecurityPolicyFields(gojson.RawMessage(tt.payload))
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestIsValid(t *testing.T) {
	tests := []struct {
		name       string
		log        armorlog
		expectsErr string
	}{
		{
			name: "invalid type",
			log: armorlog{
				Type:                   "invalid-type",
				EnforcedSecurityPolicy: &enforcedSecurityPolicy{},
			},
			expectsErr: "expected @type to be type.googleapis.com/google.cloud.loadbalancing.type.LoadBalancerLogEntry, got invalid-type",
		},
		{
			name: "all security policies nil",
			log: armorlog{
				Type: armorLogType,
			},
			expectsErr: "at least one of the security policy fields must be non-nil",
		},
		{
			name: "valid log with enforced security policy",
			log: armorlog{
				Type:                   armorLogType,
				EnforcedSecurityPolicy: &enforcedSecurityPolicy{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := isValid(tt.log)
			if tt.expectsErr != "" {
				require.ErrorContains(t, err, tt.expectsErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestHandleSecurityPolicyRequestData(t *testing.T) {
	tests := []struct {
		name     string
		data     *securityPolicyRequestData
		expected map[string]any
	}{
		{
			name:     "nil data",
			data:     nil,
			expected: map[string]any{},
		},
		{
			name: "all fields populated",
			data: &securityPolicyRequestData{
				RecaptchaActionToken:  &recaptchaToken{Score: 0.9},
				RecaptchaSessionToken: &recaptchaToken{Score: 0.0},
				UserIPInfo: &userIPInfo{
					Source:    "X-Forwarded-For",
					IPAddress: "192.168.1.1",
				},
				RemoteIPInfo: &remoteIPInfo{
					IPAddress:  "10.0.0.1",
					RegionCode: "US",
					ASN:        int64ptr(12345),
				},
				TLSJa4Fingerprint: "ja4_fingerprint",
				TLSJa3Fingerprint: "ja3_fingerprint",
			},
			expected: map[string]any{
				gcpArmorRecaptchaActionTokenScore:     float64(0.9),
				gcpArmorRecaptchaSessionTokenScore:    float64(0),
				gcpArmorUserIPInfoSource:              "X-Forwarded-For",
				string(semconv.ClientAddressKey):      "192.168.1.1",
				string(semconv.NetworkPeerAddressKey): "10.0.0.1",
				string(semconv.GeoRegionISOCodeKey):   "US",
				gcpArmorRemoteIPInfoAsn:               int64(12345),
				gcpArmorTLSJa4Fingerprint:             "ja4_fingerprint",
				string(semconv.TLSClientJa3Key):       "ja3_fingerprint",
			},
		},
		{
			name: "partial fields - recaptcha and tls",
			data: &securityPolicyRequestData{
				RecaptchaActionToken: &recaptchaToken{Score: 0.5},
				TLSJa3Fingerprint:    "ja3_only",
			},
			expected: map[string]any{
				gcpArmorRecaptchaActionTokenScore: float64(0.5),
				string(semconv.TLSClientJa3Key):   "ja3_only",
			},
		},
		{
			name: "user ip Info",
			data: &securityPolicyRequestData{
				UserIPInfo: &userIPInfo{
					Source:    "X-Real-IP",
					IPAddress: "0.0.0.0",
				},
			},
			expected: map[string]any{
				gcpArmorUserIPInfoSource:         "X-Real-IP",
				string(semconv.ClientAddressKey): "0.0.0.0",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attr := pcommon.NewMap()
			require.NoError(t, handleSecurityPolicyRequestData(tt.data, attr))
			require.Equal(t, tt.expected, attr.AsRaw())
		})
	}
}

func TestHandleSecurityPolicyBase(t *testing.T) {
	tests := []struct {
		name     string
		policy   *securityPolicyBase
		expected map[string]any
	}{
		{
			name: "all fields populated",
			policy: &securityPolicyBase{
				Name:             "test-policy",
				Priority:         int64ptr(100),
				ConfiguredAction: "DENY",
				Outcome:          "DENY",
			},
			expected: map[string]any{
				gcpArmorSecurityPolicyName:             "test-policy",
				gcpArmorSecurityPolicyPriority:         int64(100),
				gcpArmorSecurityPolicyConfiguredAction: "DENY",
				gcpArmorSecurityPolicyOutcome:          "DENY",
			},
		},
		{
			name: "zero priority",
			policy: &securityPolicyBase{
				Name:             "policy-zero",
				Priority:         int64ptr(0),
				ConfiguredAction: "allow",
				Outcome:          "accepted",
			},
			expected: map[string]any{
				gcpArmorSecurityPolicyName:             "policy-zero",
				gcpArmorSecurityPolicyPriority:         int64(0),
				gcpArmorSecurityPolicyConfiguredAction: "allow",
				gcpArmorSecurityPolicyOutcome:          "accepted",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attr := pcommon.NewMap()
			handleSecurityPolicyBase(tt.policy, attr)

			require.Equal(t, tt.expected, attr.AsRaw())
		})
	}
}

func TestHandleThreatIntelligence(t *testing.T) {
	tests := []struct {
		name     string
		ti       *threatIntelligence
		expected map[string]any
	}{
		{
			name:     "nil threat intelligence",
			ti:       nil,
			expected: map[string]any{},
		},
		{
			name: "empty categories",
			ti: &threatIntelligence{
				Categories: []string{},
			},
			expected: map[string]any{},
		},
		{
			name: "single category",
			ti: &threatIntelligence{
				Categories: []string{"malware"},
			},
			expected: map[string]any{
				gcpArmorThreatIntelligenceCategories: []any{"malware"},
			},
		},
		{
			name: "multiple categories",
			ti: &threatIntelligence{
				Categories: []string{"malware", "phishing", "botnet"},
			},
			expected: map[string]any{
				gcpArmorThreatIntelligenceCategories: []any{"malware", "phishing", "botnet"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attr := pcommon.NewMap()
			handleThreatIntelligence(tt.ti, attr)

			require.Equal(t, tt.expected, attr.AsRaw())
		})
	}
}

func TestHandleAddressGroup(t *testing.T) {
	tests := []struct {
		name     string
		ag       *addressGroup
		expected map[string]any
	}{
		{
			name:     "nil address group",
			ag:       nil,
			expected: map[string]any{},
		},
		{
			name: "empty names",
			ag: &addressGroup{
				Names: []string{},
			},
			expected: map[string]any{},
		},
		{
			name: "single name",
			ag: &addressGroup{
				Names: []string{"group1"},
			},
			expected: map[string]any{
				gcpArmorAddressGroupNames: []any{"group1"},
			},
		},
		{
			name: "multiple names",
			ag: &addressGroup{
				Names: []string{"group1", "group2", "group3"},
			},
			expected: map[string]any{
				gcpArmorAddressGroupNames: []any{"group1", "group2", "group3"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attr := pcommon.NewMap()
			handleAddressGroup(tt.ag, attr)

			require.Equal(t, tt.expected, attr.AsRaw())
		})
	}
}

func TestHandleRateLimitAction(t *testing.T) {
	tests := []struct {
		name     string
		rl       *rateLimitAction
		expected map[string]any
	}{
		{
			name:     "nil rate limit action",
			rl:       nil,
			expected: map[string]any{},
		},
		{
			name: "with key and outcome",
			rl: &rateLimitAction{
				Key:     "test-key",
				Outcome: "rate_limited",
			},
			expected: map[string]any{
				gcpArmorRateLimitActionKey:     "test-key",
				gcpArmorRateLimitActionOutcome: "rate_limited",
			},
		},
		{
			name: "empty key and outcome",
			rl: &rateLimitAction{
				Key:     "",
				Outcome: "",
			},
			expected: map[string]any{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attr := pcommon.NewMap()
			handleRateLimitAction(tt.rl, attr)

			require.Equal(t, tt.expected, attr.AsRaw())
		})
	}
}

func TestHandleSecurityPolicyExtended(t *testing.T) {
	tests := []struct {
		name     string
		policy   *securityPolicyExtended
		expected map[string]any
	}{
		{
			name: "base fields only",
			policy: &securityPolicyExtended{
				securityPolicyBase: securityPolicyBase{
					Name:             "test-policy",
					Priority:         int64ptr(10),
					ConfiguredAction: "DENY",
					Outcome:          "DENY",
				},
			},
			expected: map[string]any{
				gcpArmorSecurityPolicyName:             "test-policy",
				gcpArmorSecurityPolicyPriority:         int64(10),
				gcpArmorSecurityPolicyConfiguredAction: "DENY",
				gcpArmorSecurityPolicyOutcome:          "DENY",
			},
		},
		{
			name: "with rate limit action",
			policy: &securityPolicyExtended{
				securityPolicyBase: securityPolicyBase{
					Name:             "test-policy",
					Priority:         int64ptr(10),
					ConfiguredAction: "DENY",
					Outcome:          "DENY",
				},
				RateLimitAction: &rateLimitAction{
					Key:     "rate-key",
					Outcome: "rate_limited",
				},
			},
			expected: map[string]any{
				gcpArmorSecurityPolicyName:             "test-policy",
				gcpArmorSecurityPolicyPriority:         int64(10),
				gcpArmorSecurityPolicyConfiguredAction: "DENY",
				gcpArmorSecurityPolicyOutcome:          "DENY",
				gcpArmorRateLimitActionKey:             "rate-key",
				gcpArmorRateLimitActionOutcome:         "rate_limited",
			},
		},
		{
			name: "with preconfigured expr ids",
			policy: &securityPolicyExtended{
				securityPolicyBase: securityPolicyBase{
					Name:             "test-policy",
					Priority:         int64ptr(10),
					ConfiguredAction: "DENY",
					Outcome:          "DENY",
				},
				PreconfiguredExprIDs: []string{"expr1", "expr2"},
			},
			expected: map[string]any{
				gcpArmorSecurityPolicyName:             "test-policy",
				gcpArmorSecurityPolicyPriority:         int64(10),
				gcpArmorSecurityPolicyConfiguredAction: "DENY",
				gcpArmorSecurityPolicyOutcome:          "DENY",
				gcpArmorWAFRuleExpressionIDs:           []any{"expr1", "expr2"},
			},
		},
		{
			name: "with threat intelligence",
			policy: &securityPolicyExtended{
				securityPolicyBase: securityPolicyBase{
					Name:             "test-policy",
					Priority:         int64ptr(10),
					ConfiguredAction: "DENY",
					Outcome:          "DENY",
				},
				ThreatIntelligence: &threatIntelligence{
					Categories: []string{"malware", "phishing"},
				},
			},
			expected: map[string]any{
				gcpArmorSecurityPolicyName:             "test-policy",
				gcpArmorSecurityPolicyPriority:         int64(10),
				gcpArmorSecurityPolicyConfiguredAction: "DENY",
				gcpArmorSecurityPolicyOutcome:          "DENY",
				gcpArmorThreatIntelligenceCategories:   []any{"malware", "phishing"},
			},
		},
		{
			name: "with address group",
			policy: &securityPolicyExtended{
				securityPolicyBase: securityPolicyBase{
					Name:             "test-policy",
					Priority:         int64ptr(10),
					ConfiguredAction: "DENY",
					Outcome:          "DENY",
				},
				AddressGroup: &addressGroup{
					Names: []string{"group1", "group2"},
				},
			},
			expected: map[string]any{
				gcpArmorSecurityPolicyName:             "test-policy",
				gcpArmorSecurityPolicyPriority:         int64(10),
				gcpArmorSecurityPolicyConfiguredAction: "DENY",
				gcpArmorSecurityPolicyOutcome:          "DENY",
				gcpArmorAddressGroupNames:              []any{"group1", "group2"},
			},
		},
		{
			name: "all fields populated",
			policy: &securityPolicyExtended{
				securityPolicyBase: securityPolicyBase{
					Name:             "test-policy",
					Priority:         int64ptr(10),
					ConfiguredAction: "DENY",
					Outcome:          "DENY",
				},
				RateLimitAction: &rateLimitAction{
					Key:     "rate-key",
					Outcome: "rate_limited",
				},
				PreconfiguredExprIDs: []string{"expr1"},
				ThreatIntelligence: &threatIntelligence{
					Categories: []string{"malware"},
				},
				AddressGroup: &addressGroup{
					Names: []string{"group1"},
				},
			},
			expected: map[string]any{
				gcpArmorSecurityPolicyName:             "test-policy",
				gcpArmorSecurityPolicyPriority:         int64(10),
				gcpArmorSecurityPolicyConfiguredAction: "DENY",
				gcpArmorSecurityPolicyOutcome:          "DENY",
				gcpArmorRateLimitActionKey:             "rate-key",
				gcpArmorRateLimitActionOutcome:         "rate_limited",
				gcpArmorWAFRuleExpressionIDs:           []any{"expr1"},
				gcpArmorThreatIntelligenceCategories:   []any{"malware"},
				gcpArmorAddressGroupNames:              []any{"group1"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attr := pcommon.NewMap()
			handleSecurityPolicyExtended(tt.policy, attr)

			require.Equal(t, tt.expected, attr.AsRaw())
		})
	}
}

func TestHandleEnforcedSecurityPolicy(t *testing.T) {
	tests := []struct {
		name     string
		policy   *enforcedSecurityPolicy
		expected map[string]any
	}{
		{
			name: "without adaptive protection",
			policy: &enforcedSecurityPolicy{
				securityPolicyExtended: securityPolicyExtended{
					securityPolicyBase: securityPolicyBase{
						Name:             "test-policy",
						Priority:         int64ptr(10),
						ConfiguredAction: "DENY",
						Outcome:          "DENY",
					},
				},
			},
			expected: map[string]any{
				gcpArmorSecurityPolicyName:             "test-policy",
				gcpArmorSecurityPolicyPriority:         int64(10),
				gcpArmorSecurityPolicyConfiguredAction: "DENY",
				gcpArmorSecurityPolicyOutcome:          "DENY",
			},
		},
		{
			name: "with adaptive protection",
			policy: &enforcedSecurityPolicy{
				securityPolicyExtended: securityPolicyExtended{
					securityPolicyBase: securityPolicyBase{
						Name:             "test-policy",
						Priority:         int64ptr(10),
						ConfiguredAction: "DENY",
						Outcome:          "DENY",
					},
				},
				AdaptiveProtection: &adaptiveProtection{
					AutoDeployAlertID: "alert-123",
				},
			},
			expected: map[string]any{
				gcpArmorSecurityPolicyName:                  "test-policy",
				gcpArmorSecurityPolicyPriority:              int64(10),
				gcpArmorSecurityPolicyConfiguredAction:      "DENY",
				gcpArmorSecurityPolicyOutcome:               "DENY",
				gcpArmorAdaptiveProtectionAutoDeployAlertID: "alert-123",
			},
		},
		{
			name: "with all extended fields and adaptive protection",
			policy: &enforcedSecurityPolicy{
				securityPolicyExtended: securityPolicyExtended{
					securityPolicyBase: securityPolicyBase{
						Name:             "test-policy",
						Priority:         int64ptr(10),
						ConfiguredAction: "DENY",
						Outcome:          "DENY",
					},
					RateLimitAction: &rateLimitAction{
						Key:     "rate-key",
						Outcome: "rate_limited",
					},
					PreconfiguredExprIDs: []string{"expr1"},
					ThreatIntelligence: &threatIntelligence{
						Categories: []string{"malware"},
					},
					AddressGroup: &addressGroup{
						Names: []string{"group1"},
					},
				},
				AdaptiveProtection: &adaptiveProtection{
					AutoDeployAlertID: "alert-123",
				},
			},
			expected: map[string]any{
				gcpArmorSecurityPolicyName:                  "test-policy",
				gcpArmorSecurityPolicyPriority:              int64(10),
				gcpArmorSecurityPolicyConfiguredAction:      "DENY",
				gcpArmorSecurityPolicyOutcome:               "DENY",
				gcpArmorRateLimitActionKey:                  "rate-key",
				gcpArmorRateLimitActionOutcome:              "rate_limited",
				gcpArmorWAFRuleExpressionIDs:                []any{"expr1"},
				gcpArmorThreatIntelligenceCategories:        []any{"malware"},
				gcpArmorAddressGroupNames:                   []any{"group1"},
				gcpArmorAdaptiveProtectionAutoDeployAlertID: "alert-123",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attr := pcommon.NewMap()
			handleEnforcedSecurityPolicy(tt.policy, attr)

			require.Equal(t, tt.expected, attr.AsRaw())
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
			expectsErr: "failed to unmarshal Armor log",
		},
		{
			name: "invalid armor log type",
			payload: `{
				"@type": "invalid-type",
				"statusDetails": "denied_by_security_policy"
			}`,
			expectsErr: "invalid Armor log",
		},
		{
			name: "different remote ips",
			payload: `{
				"@type": "type.googleapis.com/google.cloud.loadbalancing.type.LoadBalancerLogEntry",
				"statusDetails": "denied_by_security_policy",
				"remoteIp": "192.168.1.1",
				"enforcedSecurityPolicy": {},
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
				"previewEdgeSecurityPolicy": {
					"name": "edge-policy",
					"priority": 5,
					"configuredAction": "allow",
					"outcome": "accepted"
				}
			}`,
			expected: map[string]any{
				gcpLoadBalancingStatusDetails:          "denied_by_security_policy",
				gcpArmorSecurityPolicyType:             securityPolicyTypePreviewEdge,
				gcpArmorSecurityPolicyName:             "edge-policy",
				gcpArmorSecurityPolicyPriority:         int64(5),
				gcpArmorSecurityPolicyConfiguredAction: "allow",
				gcpArmorSecurityPolicyOutcome:          "accepted",
			},
		},
		{
			name: "enforced edge security policy",
			payload: `{
				"@type": "type.googleapis.com/google.cloud.loadbalancing.type.LoadBalancerLogEntry",
				"statusDetails": "denied_by_security_policy",
				"enforcedEdgeSecurityPolicy": {
					"name": "edge-policy-enforced",
					"priority": 10,
					"configuredAction": "DENY",
					"outcome": "DENY"
				}
			}`,
			expected: map[string]any{
				gcpLoadBalancingStatusDetails:          "denied_by_security_policy",
				gcpArmorSecurityPolicyType:             securityPolicyTypeEnforcedEdge,
				gcpArmorSecurityPolicyName:             "edge-policy-enforced",
				gcpArmorSecurityPolicyPriority:         int64(10),
				gcpArmorSecurityPolicyConfiguredAction: "DENY",
				gcpArmorSecurityPolicyOutcome:          "DENY",
			},
		},
		{
			name: "preview security policy",
			payload: `{
				"@type": "type.googleapis.com/google.cloud.loadbalancing.type.LoadBalancerLogEntry",
				"statusDetails": "denied_by_security_policy",
				"previewSecurityPolicy": {
					"name": "preview-policy",
					"priority": 20,
					"configuredAction": "DENY",
					"outcome": "DENY",
					"preconfiguredExprIds": ["expr1", "expr2"]
				}
			}`,
			expected: map[string]any{
				gcpLoadBalancingStatusDetails:          "denied_by_security_policy",
				gcpArmorSecurityPolicyType:             securityPolicyTypePreview,
				gcpArmorSecurityPolicyName:             "preview-policy",
				gcpArmorSecurityPolicyPriority:         int64(20),
				gcpArmorSecurityPolicyConfiguredAction: "DENY",
				gcpArmorSecurityPolicyOutcome:          "DENY",
				gcpArmorWAFRuleExpressionIDs:           []any{"expr1", "expr2"},
			},
		},
		{
			name: "enforced security policy with all fields",
			payload: `{
				"@type": "type.googleapis.com/google.cloud.loadbalancing.type.LoadBalancerLogEntry",
				"statusDetails": "denied_by_security_policy",
				"enforcedSecurityPolicy": {
					"name": "enforced-policy",
					"priority": 30,
					"configuredAction": "DENY",
					"outcome": "DENY",
					"rateLimitAction": {
						"key": "rate-key",
						"outcome": "rate_limited"
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
				gcpLoadBalancingStatusDetails:               "denied_by_security_policy",
				gcpArmorSecurityPolicyType:                  securityPolicyTypeEnforced,
				gcpArmorSecurityPolicyName:                  "enforced-policy",
				gcpArmorSecurityPolicyPriority:              int64(30),
				gcpArmorSecurityPolicyConfiguredAction:      "DENY",
				gcpArmorSecurityPolicyOutcome:               "DENY",
				gcpArmorRateLimitActionKey:                  "rate-key",
				gcpArmorRateLimitActionOutcome:              "rate_limited",
				gcpArmorWAFRuleExpressionIDs:                []any{"expr1"},
				gcpArmorThreatIntelligenceCategories:        []any{"malware", "phishing"},
				gcpArmorAddressGroupNames:                   []any{"group1", "group2"},
				gcpArmorAdaptiveProtectionAutoDeployAlertID: "alert-123",
			},
		},
		{
			name: "with security policy request data",
			payload: `{
				"@type": "type.googleapis.com/google.cloud.loadbalancing.type.LoadBalancerLogEntry",
				"statusDetails": "denied_by_security_policy",
				"remoteIp": "1.2.3.4",
				"backendTargetProjectNumber": "123456789",
				"securityPolicyRequestData": {
					"recaptchaActionToken": { "score": 0.9 },
					"userIpInfo": { "source": "X-Forwarded-For", "ipAddress": "5.6.7.8" },
					"tlsJa3Fingerprint": "ja3-fingerprint"
				},
				"enforcedSecurityPolicy": {
					"name": "enforced-policy",
					"priority": 30,
					"configuredAction": "DENY",
					"outcome": "DENY"
				}
			}`,
			expected: map[string]any{
				gcpLoadBalancingStatusDetails:              "denied_by_security_policy",
				string(semconv.NetworkPeerAddressKey):      "1.2.3.4",
				gcpLoadBalancingBackendTargetProjectNumber: "123456789",
				gcpArmorRecaptchaActionTokenScore:          float64(0.9),
				gcpArmorUserIPInfoSource:                   "X-Forwarded-For",
				string(semconv.ClientAddressKey):           "5.6.7.8",
				string(semconv.TLSClientJa3Key):            "ja3-fingerprint",
				gcpArmorSecurityPolicyType:                 securityPolicyTypeEnforced,
				gcpArmorSecurityPolicyName:                 "enforced-policy",
				gcpArmorSecurityPolicyPriority:             int64(30),
				gcpArmorSecurityPolicyConfiguredAction:     "DENY",
				gcpArmorSecurityPolicyOutcome:              "DENY",
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
