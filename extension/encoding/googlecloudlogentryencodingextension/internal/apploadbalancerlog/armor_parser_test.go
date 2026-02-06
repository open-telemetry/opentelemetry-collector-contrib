// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package apploadbalancerlog

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

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
				gcpArmorRecaptchaActionTokenScore:  float64(0.9),
				gcpArmorRecaptchaSessionTokenScore: float64(0),
				gcpArmorUserIPInfoSource:           "X-Forwarded-For",
				"client.address":                   "192.168.1.1",
				"network.peer.address":             "10.0.0.1",
				"geo.region.iso_code":              "US",
				gcpArmorRemoteIPInfoAsn:            int64(12345),
				gcpArmorTLSJa4Fingerprint:          "ja4_fingerprint",
				"tls.client.ja3":                   "ja3_fingerprint",
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
				"tls.client.ja3":                  "ja3_only",
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
				gcpArmorUserIPInfoSource: "X-Real-IP",
				"client.address":         "0.0.0.0",
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

func TestHandleArmorLogAttributes(t *testing.T) {
	tests := []struct {
		name     string
		log      *armorlog
		expected map[string]any
		wantErr  bool
	}{
		{
			name: "no security policy fields - only SecurityPolicyRequestData",
			log: &armorlog{
				SecurityPolicyRequestData: &securityPolicyRequestData{
					RecaptchaActionToken: &recaptchaToken{Score: 0.9},
				},
			},
			expected: map[string]any{
				gcpArmorRecaptchaActionTokenScore: float64(0.9),
			},
			wantErr: false,
		},
		{
			name: "enforced security policy only",
			log: &armorlog{
				EnforcedSecurityPolicy: &enforcedSecurityPolicy{
					securityPolicyExtended: securityPolicyExtended{
						securityPolicyBase: securityPolicyBase{
							Name:             "test-policy",
							Priority:         int64ptr(100),
							ConfiguredAction: "DENY",
							Outcome:          "DENY",
						},
					},
				},
			},
			expected: map[string]any{
				gcpArmorSecurityPolicyEnforced: map[string]any{
					gcpArmorSecurityPolicyName:             "test-policy",
					gcpArmorSecurityPolicyPriority:         int64(100),
					gcpArmorSecurityPolicyConfiguredAction: "DENY",
					gcpArmorSecurityPolicyOutcome:          "DENY",
				},
			},
			wantErr: false,
		},
		{
			name: "preview security policy only",
			log: &armorlog{
				PreviewSecurityPolicy: &securityPolicyExtended{
					securityPolicyBase: securityPolicyBase{
						Name:             "preview-policy",
						Priority:         int64ptr(50),
						ConfiguredAction: "ALLOW",
						Outcome:          "ACCEPT",
					},
				},
			},
			expected: map[string]any{
				gcpArmorSecurityPolicyPreview: map[string]any{
					gcpArmorSecurityPolicyName:             "preview-policy",
					gcpArmorSecurityPolicyPriority:         int64(50),
					gcpArmorSecurityPolicyConfiguredAction: "ALLOW",
					gcpArmorSecurityPolicyOutcome:          "ACCEPT",
				},
			},
			wantErr: false,
		},
		{
			name: "enforced edge security policy only",
			log: &armorlog{
				EnforcedEdgeSecurityPolicy: &securityPolicyBase{
					Name:             "edge-policy",
					Priority:         int64ptr(75),
					ConfiguredAction: "DENY",
					Outcome:          "DENY",
				},
			},
			expected: map[string]any{
				gcpArmorSecurityPolicyEnforcedEdge: map[string]any{
					gcpArmorSecurityPolicyName:             "edge-policy",
					gcpArmorSecurityPolicyPriority:         int64(75),
					gcpArmorSecurityPolicyConfiguredAction: "DENY",
					gcpArmorSecurityPolicyOutcome:          "DENY",
				},
			},
			wantErr: false,
		},
		{
			name: "preview edge security policy only",
			log: &armorlog{
				PreviewEdgeSecurityPolicy: &securityPolicyBase{
					Name:             "preview-edge-policy",
					Priority:         int64ptr(25),
					ConfiguredAction: "ALLOW",
					Outcome:          "ACCEPT",
				},
			},
			expected: map[string]any{
				gcpArmorSecurityPolicyPreviewEdge: map[string]any{
					gcpArmorSecurityPolicyName:             "preview-edge-policy",
					gcpArmorSecurityPolicyPriority:         int64(25),
					gcpArmorSecurityPolicyConfiguredAction: "ALLOW",
					gcpArmorSecurityPolicyOutcome:          "ACCEPT",
				},
			},
			wantErr: false,
		},
		{
			name: "enforced policy with user ip data",
			log: &armorlog{
				EnforcedSecurityPolicy: &enforcedSecurityPolicy{
					securityPolicyExtended: securityPolicyExtended{
						securityPolicyBase: securityPolicyBase{
							Name:             "test-policy",
							Priority:         int64ptr(100),
							ConfiguredAction: "DENY",
							Outcome:          "DENY",
						},
					},
					AdaptiveProtection: &adaptiveProtection{
						AutoDeployAlertID: "alert-456",
					},
				},
				SecurityPolicyRequestData: &securityPolicyRequestData{
					RecaptchaActionToken: &recaptchaToken{Score: 0.8},
					UserIPInfo: &userIPInfo{
						Source:    "X-Forwarded-For",
						IPAddress: "1.2.3.4",
					},
				},
			},
			expected: map[string]any{
				gcpArmorSecurityPolicyEnforced: map[string]any{
					gcpArmorSecurityPolicyName:                  "test-policy",
					gcpArmorSecurityPolicyPriority:              int64(100),
					gcpArmorSecurityPolicyConfiguredAction:      "DENY",
					gcpArmorSecurityPolicyOutcome:               "DENY",
					gcpArmorAdaptiveProtectionAutoDeployAlertID: "alert-456",
				},
				gcpArmorRecaptchaActionTokenScore: float64(0.8),
				gcpArmorUserIPInfoSource:          "X-Forwarded-For",
				"client.address":                  "1.2.3.4",
			},
			wantErr: false,
		},
		{
			name: "preview policy with all extended fields",
			log: &armorlog{
				PreviewSecurityPolicy: &securityPolicyExtended{
					securityPolicyBase: securityPolicyBase{
						Name:             "full-preview",
						Priority:         int64ptr(10),
						ConfiguredAction: "DENY",
						Outcome:          "DENY",
					},
					RateLimitAction: &rateLimitAction{
						Key:     "rl-key",
						Outcome: "RATE_LIMITED",
					},
					PreconfiguredExprIDs: []string{"expr1", "expr2"},
					ThreatIntelligence: &threatIntelligence{
						Categories: []string{"botnet"},
					},
					AddressGroup: &addressGroup{
						Names: []string{"addr-group-1"},
					},
				},
				SecurityPolicyRequestData: &securityPolicyRequestData{
					TLSJa3Fingerprint: "ja3-hash",
					TLSJa4Fingerprint: "ja4-hash",
				},
			},
			expected: map[string]any{
				gcpArmorSecurityPolicyPreview: map[string]any{
					gcpArmorSecurityPolicyName:             "full-preview",
					gcpArmorSecurityPolicyPriority:         int64(10),
					gcpArmorSecurityPolicyConfiguredAction: "DENY",
					gcpArmorSecurityPolicyOutcome:          "DENY",
					gcpArmorRateLimitActionKey:             "rl-key",
					gcpArmorRateLimitActionOutcome:         "RATE_LIMITED",
					gcpArmorWAFRuleExpressionIDs:           []any{"expr1", "expr2"},
					gcpArmorThreatIntelligenceCategories:   []any{"botnet"},
					gcpArmorAddressGroupNames:              []any{"addr-group-1"},
				},
				"tls.client.ja3":          "ja3-hash",
				gcpArmorTLSJa4Fingerprint: "ja4-hash",
			},
			wantErr: false,
		},
		{
			name: "multiple policies - enforced and preview",
			log: &armorlog{
				EnforcedSecurityPolicy: &enforcedSecurityPolicy{
					securityPolicyExtended: securityPolicyExtended{
						securityPolicyBase: securityPolicyBase{
							Name:             "enforced-policy",
							Priority:         int64ptr(100),
							ConfiguredAction: "DENY",
							Outcome:          "DENY",
						},
					},
				},
				PreviewSecurityPolicy: &securityPolicyExtended{
					securityPolicyBase: securityPolicyBase{
						Name:             "preview-policy",
						Priority:         int64ptr(50),
						ConfiguredAction: "ALLOW",
						Outcome:          "ACCEPT",
					},
				},
			},
			expected: map[string]any{
				gcpArmorSecurityPolicyEnforced: map[string]any{
					gcpArmorSecurityPolicyName:             "enforced-policy",
					gcpArmorSecurityPolicyPriority:         int64(100),
					gcpArmorSecurityPolicyConfiguredAction: "DENY",
					gcpArmorSecurityPolicyOutcome:          "DENY",
				},
				gcpArmorSecurityPolicyPreview: map[string]any{
					gcpArmorSecurityPolicyName:             "preview-policy",
					gcpArmorSecurityPolicyPriority:         int64(50),
					gcpArmorSecurityPolicyConfiguredAction: "ALLOW",
					gcpArmorSecurityPolicyOutcome:          "ACCEPT",
				},
			},
			wantErr: false,
		},
		{
			name: "all four policy types with request data",
			log: &armorlog{
				EnforcedSecurityPolicy: &enforcedSecurityPolicy{
					securityPolicyExtended: securityPolicyExtended{
						securityPolicyBase: securityPolicyBase{
							Name:             "enforced-policy",
							Priority:         int64ptr(100),
							ConfiguredAction: "DENY",
							Outcome:          "DENY",
						},
					},
					AdaptiveProtection: &adaptiveProtection{
						AutoDeployAlertID: "alert-123",
					},
				},
				PreviewSecurityPolicy: &securityPolicyExtended{
					securityPolicyBase: securityPolicyBase{
						Name:             "preview-policy",
						Priority:         int64ptr(50),
						ConfiguredAction: "ALLOW",
						Outcome:          "ACCEPT",
					},
				},
				EnforcedEdgeSecurityPolicy: &securityPolicyBase{
					Name:             "enforced-edge-policy",
					Priority:         int64ptr(75),
					ConfiguredAction: "DENY",
					Outcome:          "DENY",
				},
				PreviewEdgeSecurityPolicy: &securityPolicyBase{
					Name:             "preview-edge-policy",
					Priority:         int64ptr(25),
					ConfiguredAction: "ALLOW",
					Outcome:          "ACCEPT",
				},
				SecurityPolicyRequestData: &securityPolicyRequestData{
					RecaptchaActionToken: &recaptchaToken{Score: 0.95},
					TLSJa3Fingerprint:    "ja3-fingerprint",
				},
			},
			expected: map[string]any{
				gcpArmorSecurityPolicyEnforced: map[string]any{
					gcpArmorSecurityPolicyName:                  "enforced-policy",
					gcpArmorSecurityPolicyPriority:              int64(100),
					gcpArmorSecurityPolicyConfiguredAction:      "DENY",
					gcpArmorSecurityPolicyOutcome:               "DENY",
					gcpArmorAdaptiveProtectionAutoDeployAlertID: "alert-123",
				},
				gcpArmorSecurityPolicyPreview: map[string]any{
					gcpArmorSecurityPolicyName:             "preview-policy",
					gcpArmorSecurityPolicyPriority:         int64(50),
					gcpArmorSecurityPolicyConfiguredAction: "ALLOW",
					gcpArmorSecurityPolicyOutcome:          "ACCEPT",
				},
				gcpArmorSecurityPolicyEnforcedEdge: map[string]any{
					gcpArmorSecurityPolicyName:             "enforced-edge-policy",
					gcpArmorSecurityPolicyPriority:         int64(75),
					gcpArmorSecurityPolicyConfiguredAction: "DENY",
					gcpArmorSecurityPolicyOutcome:          "DENY",
				},
				gcpArmorSecurityPolicyPreviewEdge: map[string]any{
					gcpArmorSecurityPolicyName:             "preview-edge-policy",
					gcpArmorSecurityPolicyPriority:         int64(25),
					gcpArmorSecurityPolicyConfiguredAction: "ALLOW",
					gcpArmorSecurityPolicyOutcome:          "ACCEPT",
				},
				gcpArmorRecaptchaActionTokenScore: float64(0.95),
				"tls.client.ja3":                  "ja3-fingerprint",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attr := pcommon.NewMap()
			err := handleArmorLogAttributes(tt.log, attr)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expected, attr.AsRaw())
			}
		})
	}
}
