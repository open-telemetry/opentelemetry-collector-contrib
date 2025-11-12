// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Find more information about Armor Logs:
// https://docs.cloud.google.com/armor/docs/request-logging

package armorlog // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/googlecloudlogentryencodingextension/internal/armorlog"

import (
	"fmt"
	"strings"

	gojson "github.com/goccy/go-json"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/googlecloudlogentryencodingextension/internal/shared"
)

const (
	LoadBalancerLogSuffix = "requests"

	armorLogType = "type.googleapis.com/google.cloud.loadbalancing.type.LoadBalancerLogEntry"

	// gcpLoadBalancingStatusDetails holds a textual description of the response code
	gcpLoadBalancingStatusDetails = "gcp.load_balancing.status.details"

	// gcpLoadBalancingBackendTargetProjectNumber holds the project number of the backend target
	gcpLoadBalancingBackendTargetProjectNumber = "gcp.load_balancing.backend_target_project_number"

	// gcpLoadBalancingRemoteIP holds the remote IP address of the client
	gcpLoadBalancingRemoteIP = "gcp.load_balancing.remote_ip"

	// gcpArmorSecurityPolicyType holds security policy type
	gcpArmorSecurityPolicyType = "gcp.armor.security_policy.type"

	// Security policy base attributes
	// gcpArmorSecurityPolicyName holds the security policy rule that was enforced
	gcpArmorSecurityPolicyName = "gcp.armor.security_policy.name"
	// gcpArmorSecurityPolicyPriority holds a numerical priority of the matching rule in the security policy
	gcpArmorSecurityPolicyPriority = "gcp.armor.security_policy.priority"
	// gcpArmorSecurityPolicyConfiguredAction holds  the name of the configured action in the matching rule
	gcpArmorSecurityPolicyConfiguredAction = "gcp.armor.security_policy.configured_action"
	// gcpArmorSecurityPolicyOutcome holds the outcome of executing the configured action
	gcpArmorSecurityPolicyOutcome = "gcp.armor.security_policy.outcome"

	// Rate limit action attributes
	// gcpArmorRateLimitActionKey holds the the rate limit key value (up to 36 bytes)
	gcpArmorRateLimitActionKey = "gcp.armor.security_policy.rate_limit.action.key"
	// gcpArmorRateLimitActionOutcome holds the outcome of the rate limit action
	gcpArmorRateLimitActionOutcome = "gcp.armor.security_policy.rate_limit.action.outcome"

	// Extended attributes
	// gcpArmorPreconfiguredExprIds holds the IDs of all preconfigured WAF rule expressions that triggered the rule
	gcpArmorPreconfiguredExprIds = "gcp.armor.security_policy.preconfigured.expr_ids"
	// gcpArmorThreatIntelligenceCategories holds information about the matched IP address lists from Google Threat Intelligence
	gcpArmorThreatIntelligenceCategories = "gcp.armor.security_policy.threat_intelligence.categories"
	// gcpArmorAddressGroupNames holds the names of the matched address groups
	gcpArmorAddressGroupNames = "gcp.armor.security_policy.address_group.names"

	// Enforced policy attributes
	// gcpArmorAdaptiveProtectionAutoDeployAlertID holds the alert ID of the events that Adaptive Protection detected
	gcpArmorAdaptiveProtectionAutoDeployAlertID = "gcp.armor.security_policy.adaptive_protection.auto_deploy.alert_id"

	// Security policy request data attributes
	// gcpArmorRecaptchaActionTokenScore holds the legitimacy score embedded in a of the reCAPTCHA action-token
	gcpArmorRecaptchaActionTokenScore = "gcp.armor.request_data.recaptcha_action_token.score"
	// gcpArmorRecaptchaSessionTokenScore holds the legitimacy score embedded in a of the reCAPTCHA session-token
	gcpArmorRecaptchaSessionTokenScore = "gcp.armor.request_data.recaptcha_session_token.score"
	// gcpArmorUserIPInfoSource holds a field that is typically the header from which the user IP was resolved
	gcpArmorUserIPInfoSource = "gcp.armor.request_data.user_ip_info.source"
	// gcpArmorUserIPInfoIPAddress holds the IP address resolved from the source field
	gcpArmorUserIPInfoIPAddress = "gcp.armor.request_data.user_ip_info.ip_address"
	// gcpArmorRemoteIPInfoIPAddress holds the IP address of the remote IP
	gcpArmorRemoteIPInfoIPAddress = "gcp.armor.request_data.remote_ip.ip_address"
	// gcpArmorRemoteIPInfoRegionCode holds the two-letter country code or region code for the IP address
	gcpArmorRemoteIPInfoRegionCode = "gcp.armor.request_data.remote_ip.region_code"
	// gcpArmorRemoteIPInfoAsn holds the five-digit autonomous system number (ASN) for the IP address
	gcpArmorRemoteIPInfoAsn = "gcp.armor.request_data.remote_ip.asn"
	// gcpArmorTLSJa4Fingerprint holds a JA4 TTL/SSL fingerprint if the client connects using HTTPS, HTTP/2, or HTTP/3
	gcpArmorTLSJa4Fingerprint = "gcp.armor.request_data.tls_ja4_fingerprint"
	// gcpArmorTLSJa3Fingerprint holds a JA3 TTL/SSL fingerprint if the client connects using HTTPS, HTTP/2, or HTTP/3
	gcpArmorTLSJa3Fingerprint = "gcp.armor.request_data.tls_ja3_fingerprint"
)

const (
	// Possible statusDetails string values
	statusDetailsRedirectedBySecurityPolicy = "redirected_by_security_policy"
	statusDetailsDeniedBySecurityPolicy     = "denied_by_security_policy"
	statusDetailsBodyDeniedBySecurityPolicy = "body_denied_by_security_policy"

	// Security policy type string values
	securityPolicyTypePreviewEdge  = "previewEdgeSecurityPolicy"
	securityPolicyTypeEnforcedEdge = "enforcedEdgeSecurityPolicy"
	securityPolicyTypePreview      = "previewSecurityPolicy"
	securityPolicyTypeEnforced     = "enforcedSecurityPolicy"
)

type armorlog struct {
	Type          string `json:"@type"`
	StatusDetails string `json:"statusDetails"`

	// Request metadata fields
	BackendTargetProjectNumber string                     `json:"backendTargetProjectNumber"`
	RemoteIP                   string                     `json:"remoteIp"`
	CacheDecision              []string                   `json:"cacheDecision"`
	SecurityPolicyRequestData  *securityPolicyRequestData `json:"securityPolicyRequestData"`

	// Security policy fields: exactly one must be non-nil.
	EnforcedSecurityPolicy     *enforcedSecurityPolicy `json:"enforcedSecurityPolicy"`
	PreviewSecurityPolicy      *securityPolicyExtended `json:"previewSecurityPolicy"`
	EnforcedEdgeSecurityPolicy *securityPolicyBase     `json:"enforcedEdgeSecurityPolicy"`
	PreviewEdgeSecurityPolicy  *securityPolicyBase     `json:"previewEdgeSecurityPolicy"`
}

type securityPolicyRequestData struct {
	RecaptchaActionToken  *recaptchaToken `json:"recaptchaActionToken"`
	RecaptchaSessionToken *recaptchaToken `json:"recaptchaSessionToken"`
	UserIPInfo            *userIPInfo     `json:"userIpInfo"`
	RemoteIPInfo          *remoteIPInfo   `json:"remoteIpInfo"`
	TLSJa4Fingerprint     string          `json:"tlsJa4Fingerprint"`
	TLSJa3Fingerprint     string          `json:"tlsJa3Fingerprint"`
}

type recaptchaToken struct {
	Score float64 `json:"score"`
}

type userIPInfo struct {
	Source    string `json:"source"`
	IPAddress string `json:"ipAddress"`
}

type remoteIPInfo struct {
	IPAddress  string `json:"ipAddress"`
	RegionCode string `json:"regionCode"`
	ASN        int    `json:"asn"`
}

type securityPolicyBase struct {
	Name             string `json:"name"`
	Priority         int    `json:"priority"`
	ConfiguredAction string `json:"configuredAction"`
	Outcome          string `json:"outcome"`
}

type securityPolicyExtended struct {
	securityPolicyBase
	RateLimitAction      *rateLimitAction    `json:"rateLimitAction"`
	PreconfiguredExprIds []string            `json:"preconfiguredExprIds"`
	ThreatIntelligence   *threatIntelligence `json:"threatIntelligence"`
	AddressGroup         *addressGroup       `json:"addressGroup"`
}

type enforcedSecurityPolicy struct {
	securityPolicyExtended
	AdaptiveProtection *adaptiveProtection `json:"adaptiveProtection"`
}

type rateLimitAction struct {
	Key     string `json:"key"`
	Outcome string `json:"outcome"`
}

type adaptiveProtection struct {
	AutoDeployAlertID string `json:"autoDeployAlertId"`
}

type threatIntelligence struct {
	Categories []string `json:"categories"`
}

type addressGroup struct {
	Names []string `json:"names"`
}

func isValid(log armorlog) error {
	if log.Type != armorLogType {
		return fmt.Errorf("expected @type to be %s, got %s", armorLogType, log.Type)
	}

	if log.StatusDetails != statusDetailsBodyDeniedBySecurityPolicy &&
		log.StatusDetails != statusDetailsDeniedBySecurityPolicy &&
		log.StatusDetails != statusDetailsRedirectedBySecurityPolicy {
		return fmt.Errorf("invalid statusDetails for Cloud Armor log entry: %s", log.StatusDetails)
	}

	if log.EnforcedSecurityPolicy == nil &&
		log.PreviewSecurityPolicy == nil &&
		log.EnforcedEdgeSecurityPolicy == nil &&
		log.PreviewEdgeSecurityPolicy == nil {
		return fmt.Errorf("at least one of the security policy fields must be non-nil")
	}

	return nil
}

func handleRecaptchaTokens(data *securityPolicyRequestData, attr pcommon.Map) {
	if data.RecaptchaActionToken != nil && data.RecaptchaActionToken.Score != 0 {
		attr.PutDouble(gcpArmorRecaptchaActionTokenScore, data.RecaptchaActionToken.Score)
	}

	if data.RecaptchaSessionToken != nil && data.RecaptchaSessionToken.Score != 0 {
		attr.PutDouble(gcpArmorRecaptchaSessionTokenScore, data.RecaptchaSessionToken.Score)
	}
}

func handleUserIPInfo(info *userIPInfo, attr pcommon.Map) {
	if info == nil {
		return
	}

	attr.PutStr(gcpArmorUserIPInfoSource, info.Source)
	if info.IPAddress != "" {
		attr.PutStr(gcpArmorUserIPInfoIPAddress, info.IPAddress)
	}
}

func handleRemoteIPInfo(info *remoteIPInfo, attr pcommon.Map) {
	if info == nil {
		return
	}

	if info.IPAddress != "" {
		attr.PutStr(gcpArmorRemoteIPInfoIPAddress, info.IPAddress)
	}
	attr.PutStr(gcpArmorRemoteIPInfoRegionCode, info.RegionCode)
	attr.PutInt(gcpArmorRemoteIPInfoAsn, int64(info.ASN))
}

func handleTLSFingerprints(data *securityPolicyRequestData, attr pcommon.Map) {
	if data.TLSJa4Fingerprint != "" {
		attr.PutStr(gcpArmorTLSJa4Fingerprint, data.TLSJa4Fingerprint)
	}
	if data.TLSJa3Fingerprint != "" {
		attr.PutStr(gcpArmorTLSJa3Fingerprint, data.TLSJa3Fingerprint)
	}
}

func handleSecurityPolicyRequestData(data *securityPolicyRequestData, attr pcommon.Map) {
	if data == nil {
		return
	}

	handleRecaptchaTokens(data, attr)
	handleUserIPInfo(data.UserIPInfo, attr)
	handleRemoteIPInfo(data.RemoteIPInfo, attr)
	handleTLSFingerprints(data, attr)
}

func handleRateLimitAction(rl *rateLimitAction, attr pcommon.Map) {
	if rl != nil {
		attr.PutStr(gcpArmorRateLimitActionKey, rl.Key)
		attr.PutStr(gcpArmorRateLimitActionOutcome, rl.Outcome)
	}
}

func handleAddressGroup(ag *addressGroup, attr pcommon.Map) {
	if ag != nil && len(ag.Names) > 0 {
		namesSlice := attr.PutEmptySlice(gcpArmorAddressGroupNames)
		for _, name := range ag.Names {
			namesSlice.AppendEmpty().SetStr(name)
		}
	}
}

func handleThreatIntelligence(ti *threatIntelligence, attr pcommon.Map) {
	if ti != nil && len(ti.Categories) > 0 {
		categoriesSlice := attr.PutEmptySlice(gcpArmorThreatIntelligenceCategories)
		for _, category := range ti.Categories {
			categoriesSlice.AppendEmpty().SetStr(category)
		}
	}
}

func handleSecurityPolicyBase(sp *securityPolicyBase, attr pcommon.Map) {
	attr.PutStr(gcpArmorSecurityPolicyName, sp.Name)
	attr.PutInt(gcpArmorSecurityPolicyPriority, int64(sp.Priority))
	attr.PutStr(gcpArmorSecurityPolicyConfiguredAction, sp.ConfiguredAction)
	attr.PutStr(gcpArmorSecurityPolicyOutcome, sp.Outcome)
}

func handleSecurityPolicyExtended(sp *securityPolicyExtended, attr pcommon.Map) {
	handleSecurityPolicyBase(&sp.securityPolicyBase, attr)

	handleRateLimitAction(sp.RateLimitAction, attr)
	handleThreatIntelligence(sp.ThreatIntelligence, attr)
	handleAddressGroup(sp.AddressGroup, attr)

	if len(sp.PreconfiguredExprIds) > 0 {
		exprIdsSlice := attr.PutEmptySlice(gcpArmorPreconfiguredExprIds)
		for _, id := range sp.PreconfiguredExprIds {
			exprIdsSlice.AppendEmpty().SetStr(id)
		}
	}
}

func handleEnforcedSecurityPolicy(sp *enforcedSecurityPolicy, attr pcommon.Map) {
	handleSecurityPolicyExtended(&sp.securityPolicyExtended, attr)

	if sp.AdaptiveProtection != nil {
		attr.PutStr(gcpArmorAdaptiveProtectionAutoDeployAlertID, sp.AdaptiveProtection.AutoDeployAlertID)
	}
}

func ContainsSecurityPolicyFields(jsonPayload gojson.RawMessage) bool {
	// Check for the presence of key armor log field names in the raw JSON without unmarshaling
	// to avoid unnecessary overhead for non-armor load balancer logs.
	s := string(jsonPayload)
	return strings.Contains(s, `"enforcedSecurityPolicy"`) ||
		strings.Contains(s, `"previewSecurityPolicy"`) ||
		strings.Contains(s, `"enforcedEdgeSecurityPolicy"`) ||
		strings.Contains(s, `"previewEdgeSecurityPolicy"`)
}

func ParsePayloadIntoAttributes(payload []byte, attr pcommon.Map) error {
	var log armorlog
	if err := gojson.Unmarshal(payload, &log); err != nil {
		return fmt.Errorf("failed to unmarshal Armor log: %w", err)
	}

	if err := isValid(log); err != nil {
		return fmt.Errorf("invalid Armor log: %w", err)
	}

	attr.PutStr(gcpLoadBalancingStatusDetails, log.StatusDetails)

	// Handle request metadata fields
	shared.PutStr(gcpLoadBalancingBackendTargetProjectNumber, log.BackendTargetProjectNumber, attr)
	shared.PutStr(gcpLoadBalancingRemoteIP, log.RemoteIP, attr)

	// Handle security policy request data (all nested fields)
	handleSecurityPolicyRequestData(log.SecurityPolicyRequestData, attr)

	if log.PreviewEdgeSecurityPolicy != nil {
		attr.PutStr(gcpArmorSecurityPolicyType, securityPolicyTypePreviewEdge)
		handleSecurityPolicyBase(log.PreviewEdgeSecurityPolicy, attr)
	} else if log.EnforcedEdgeSecurityPolicy != nil {
		attr.PutStr(gcpArmorSecurityPolicyType, securityPolicyTypeEnforcedEdge)
		handleSecurityPolicyBase(log.EnforcedEdgeSecurityPolicy, attr)
	} else if log.PreviewSecurityPolicy != nil {
		attr.PutStr(gcpArmorSecurityPolicyType, securityPolicyTypePreview)
		handleSecurityPolicyExtended(log.PreviewSecurityPolicy, attr)
	} else if log.EnforcedSecurityPolicy != nil {
		attr.PutStr(gcpArmorSecurityPolicyType, securityPolicyTypeEnforced)
		handleEnforcedSecurityPolicy(log.EnforcedSecurityPolicy, attr)
	}

	return nil
}
