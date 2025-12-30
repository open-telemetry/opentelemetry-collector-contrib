// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package apploadbalancerlog // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/googlecloudlogentryencodingextension/internal/apploadbalancerlog"

import (
	"fmt"

	gojson "github.com/goccy/go-json"
	"go.opentelemetry.io/collector/pdata/pcommon"
	conventions "go.opentelemetry.io/otel/semconv/v1.38.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/googlecloudlogentryencodingextension/internal/shared"
)

const (
	GlobalAppLoadBalancerLogSuffix   = "requests"
	RegionalAppLoadBalancerLogSuffix = "loadbalancing.googleapis.com%2Fexternal_regional_requests"

	loadBalancerLogType = "type.googleapis.com/google.cloud.loadbalancing.type.LoadBalancerLogEntry"

	// gcpLoadBalancingStatusDetails holds a textual description of the response code
	gcpLoadBalancingStatusDetails = "gcp.load_balancing.status.details"
	// gcpLoadBalancingScheme holds a string that describes which load balancing scheme was used to route the request.
	gcpLoadBalancingScheme = "gcp.load_balancing.scheme"
	// gcpLoadBalancingBackendTargetProjectNumber holds the project number of the backend target
	gcpLoadBalancingBackendTargetProjectNumber = "gcp.load_balancing.backend_target_project_number"

	// Request Metadata fields
	// gcpLoadBalancingProxyStatus holds why the internal Application Load Balancer returned an HTTP error code.
	gcpLoadBalancingProxyStatus = "gcp.load_balancing.proxy_status"
	// gcpLoadBalancingOverrideResponseCode holds the override response code applied to the response sent to the client.
	gcpLoadBalancingOverrideResponseCode = "gcp.load_balancing.override_response_code"
	// gcpLoadBalancingErrorService holds the backend service that provided the custom error response.
	gcpLoadBalancingErrorService = "gcp.load_balancing.error_service"
	// gcpLoadBalancingCacheID holds the cache ID used by the load balancer.
	gcpLoadBalancingCacheID = "gcp.load_balancing.cache.id"
	// gcpLoadBalancingCacheDecision holds the cache decision made by the load balancer.
	gcpLoadBalancingCacheDecision = "gcp.load_balancing.cache.decision"
	// gcpLoadBalancingBackendNetworkName specifies the VPC network of the backend.
	gcpLoadBalancingBackendNetworkName = "gcp.load_balancing.backend_network_name"

	// Security/Authentication information
	// gcpLoadBalancingAuthPolicyInfo stores information of the overall authorization policy result.
	gcpLoadBalancingAuthPolicyInfoResult = "gcp.load_balancing.auth_policy_info.result"
	// gcpLoadBalancingAuthPolicyInfoPolicies holds the list of policies that match the request.
	gcpLoadBalancingAuthPolicyInfoPolicies = "gcp.load_balancing.auth_policy_info.policies"
	// gcpLoadBalancingAuthPolicyName holds the name of the authorization policy.
	gcpLoadBalancingAuthPolicyName = "name"
	// gcpLoadBalancingAuthPolicyResult holds the result of the authorization policy.
	gcpLoadBalancingAuthPolicyResult = "result"
	// gcpLoadBalancingAuthPolicyDetails holds the details of the authorization policy.
	gcpLoadBalancingAuthPolicyDetails = "details"
	// gcpLoadBalancingTLSInfo specifies the TLS metadata for the connection between the client and the load balancer
	gcpLoadBalancingTLSInfo = "gcp.load_balancing.tls_info"
	// gcpLoadBalancingTLSEarlyDataRequest specifies if the request includes early data in the TLS handshake.
	gcpLoadBalancingTLSEarlyDataRequest = "tls.early_data_request"
	// gcpLoadBalancingMtlsInfo specifies the mTLS metadata for the connection between the client and the load balancer.
	gcpLoadBalancingMtlsInfo = "gcp.load_balancing.mtls"
	// gcpLoadBalancingMtlsClientCertPresent is true if the client has provided a certificate during the TLS handshake.
	gcpLoadBalancingMtlsClientCertPresent = "mtls.client_cert.present"
	// gcpLoadBalancingMtlsClientCertChainVerified is true if the client certificate chain is verified against a configured TrustStore
	gcpLoadBalancingMtlsClientCertChainVerified = "mtls.client_cert.chain_verified"
	// gcpLoadBalancingMtlsClientCertError holds the predefined string representing the error conditions.
	gcpLoadBalancingMtlsClientCertError = "mtls.client_cert.error"
	// gcpLoadBalancingMtlsClientCertSerialNumber holds The serial number of the client certificate
	gcpLoadBalancingMtlsClientCertSerialNumber = "mtls.client_cert.serial_number"
	// gcpLoadBalancingMtlsClientCertSpiffeID holds the The SPIFFE ID from the subject alternative name (SAN) field.
	gcpLoadBalancingMtlsClientCertSpiffeID = "mtls.client_cert.spiffe_id"
	// gcpLoadBalancingMtlsClientCertURISans holds the comma-separated Base64-encoded list of the SAN extensions of type URI.
	gcpLoadBalancingMtlsClientCertURISans = "mtls.client_cert.uri_sans"
	// gcpLoadBalancingMtlsClientCertDnsnameSans holds the comma-separated Base64-encoded list of the SAN extensions of type DNSName.
	gcpLoadBalancingMtlsClientCertDnsnameSans = "mtls.client_cert.dnsname_sans"
	// gcpLoadBalancingMtlsClientCertLeaf holds the client leaf certificate for an established mTLS connection where the certificate passed validation.
	gcpLoadBalancingMtlsClientCertLeaf = "mtls.client_cert.leaf"
)

type loadbalancerlog struct {
	Type string `json:"@type"`

	// Request Metadata fields
	StatusDetails              string   `json:"statusDetails"`
	RemoteIP                   string   `json:"remoteIp"`
	BackendTargetProjectNumber string   `json:"backendTargetProjectNumber"`
	ProxyStatus                string   `json:"proxyStatus"`
	OverrideResponseCode       *int64   `json:"overrideResponseCode"`
	LoadBalancingScheme        string   `json:"loadBalancingScheme"`
	ErrorService               string   `json:"errorService"`
	BackendNetworkName         string   `json:"backendNetworkName"`
	CacheID                    string   `json:"cacheId"`
	CacheDecision              []string `json:"cacheDecision"`

	// Security/Authentication information
	AuthPolicyInfo *authPolicyInfo `json:"authPolicyInfo"`
	TLSInfo        *tlsInfo        `json:"tls"`
	MtlsInfo       *mtlsInfo       `json:"mtls"`

	// ToDo: Add support for OrcaLoadReport

	// Embed Cloud Armor log fields
	armorlog
}

type authPolicyInfo struct {
	OverallResult string       `json:"result"`
	Policies      []policyInfo `json:"policies"`
}

type policyInfo struct {
	Name    string `json:"name"`
	Result  string `json:"result"`
	Details string `json:"details"`
}

type tlsInfo struct {
	EarlyDataRequest *bool  `json:"earlyDataRequest"`
	Protocol         string `json:"protocol"`
	Cipher           string `json:"cipher"`
}

type mtlsInfo struct {
	ClientCertPresent           *bool  `json:"clientCertPresent"`
	ClientCertChainVerified     *bool  `json:"clientCertChainVerified"`
	ClientCertError             string `json:"clientCertError"`
	ClientCertSha256Fingerprint string `json:"clientCertSha256Fingerprint"`
	ClientCertSerialNumber      string `json:"clientCertSerialNumber"`
	ClientCertValidStartTime    string `json:"clientCertValidStartTime"`
	ClientCertValidEndTime      string `json:"clientCertValidEndTime"`
	ClientCertSpiffeID          string `json:"clientCertSpiffeId"`
	ClientCertURISans           string `json:"clientCertURISans"`
	ClientCertDnsnameSans       string `json:"clientCertDnsnameSans"`
	ClientCertIssuerDn          string `json:"clientCertIssuerDn"`
	ClientCertSubjectDn         string `json:"clientCertSubjectDn"`
	ClientCertLeaf              string `json:"clientCertLeaf"`
	ClientCertChain             string `json:"clientCertChain"`
}

func isValid(log *loadbalancerlog) error {
	if log.Type != loadBalancerLogType {
		return fmt.Errorf("expected @type to be %s, got %s", loadBalancerLogType, log.Type)
	}
	return nil
}

func handleRequestMetadata(log *loadbalancerlog, attr pcommon.Map) error {
	if _, err := shared.PutStrIfNotPresent(string(conventions.NetworkPeerAddressKey), log.RemoteIP, attr); err != nil {
		return fmt.Errorf("error setting security policy attribute: %w", err)
	}

	shared.PutStr(gcpLoadBalancingStatusDetails, log.StatusDetails, attr)
	shared.PutStr(gcpLoadBalancingBackendTargetProjectNumber, log.BackendTargetProjectNumber, attr)
	shared.PutStr(gcpLoadBalancingProxyStatus, log.ProxyStatus, attr)
	shared.PutInt(gcpLoadBalancingOverrideResponseCode, log.OverrideResponseCode, attr)
	shared.PutStr(gcpLoadBalancingScheme, log.LoadBalancingScheme, attr)
	shared.PutStr(gcpLoadBalancingErrorService, log.ErrorService, attr)
	shared.PutStr(gcpLoadBalancingBackendNetworkName, log.BackendNetworkName, attr)
	shared.PutStr(gcpLoadBalancingCacheID, log.CacheID, attr)

	if len(log.CacheDecision) > 0 {
		cacheDecisions := attr.PutEmptySlice(gcpLoadBalancingCacheDecision)
		for _, decision := range log.CacheDecision {
			cacheDecisions.AppendEmpty().SetStr(decision)
		}
	}
	return nil
}

func handleAuthPolicyInfo(authPolicyInfo *authPolicyInfo, attr pcommon.Map) {
	if authPolicyInfo == nil {
		return
	}
	shared.PutStr(gcpLoadBalancingAuthPolicyInfoResult, authPolicyInfo.OverallResult, attr)

	if len(authPolicyInfo.Policies) > 0 {
		policiesSlice := attr.PutEmptySlice(gcpLoadBalancingAuthPolicyInfoPolicies)
		for _, policy := range authPolicyInfo.Policies {
			policyMap := policiesSlice.AppendEmpty().SetEmptyMap()
			shared.PutStr(gcpLoadBalancingAuthPolicyName, policy.Name, policyMap)
			shared.PutStr(gcpLoadBalancingAuthPolicyResult, policy.Result, policyMap)
			shared.PutStr(gcpLoadBalancingAuthPolicyDetails, policy.Details, policyMap)
		}
	}
}

func handleTLSInfo(tlsInfo *tlsInfo, attr pcommon.Map) {
	if tlsInfo == nil {
		return
	}
	tlsMap := attr.PutEmptyMap(gcpLoadBalancingTLSInfo)
	shared.PutBool(gcpLoadBalancingTLSEarlyDataRequest, tlsInfo.EarlyDataRequest, tlsMap)
	shared.PutStr(string(conventions.TLSProtocolNameKey), tlsInfo.Protocol, tlsMap)
	shared.PutStr(string(conventions.TLSCipherKey), tlsInfo.Cipher, tlsMap)
}

func handleMtlsInfo(mtlsInfo *mtlsInfo, attr pcommon.Map) {
	if mtlsInfo == nil {
		return
	}

	mtlsMap := attr.PutEmptyMap(gcpLoadBalancingMtlsInfo)
	shared.PutBool(gcpLoadBalancingMtlsClientCertPresent, mtlsInfo.ClientCertPresent, mtlsMap)
	shared.PutBool(gcpLoadBalancingMtlsClientCertChainVerified, mtlsInfo.ClientCertChainVerified, mtlsMap)
	shared.PutStr(gcpLoadBalancingMtlsClientCertError, mtlsInfo.ClientCertError, mtlsMap)
	shared.PutStr(string(conventions.TLSClientHashSha256Key), mtlsInfo.ClientCertSha256Fingerprint, mtlsMap)
	shared.PutStr(gcpLoadBalancingMtlsClientCertSerialNumber, mtlsInfo.ClientCertSerialNumber, mtlsMap)
	shared.PutStr(string(conventions.TLSClientNotBeforeKey), mtlsInfo.ClientCertValidStartTime, mtlsMap)
	shared.PutStr(string(conventions.TLSClientNotAfterKey), mtlsInfo.ClientCertValidEndTime, mtlsMap)
	shared.PutStr(gcpLoadBalancingMtlsClientCertSpiffeID, mtlsInfo.ClientCertSpiffeID, mtlsMap)
	shared.PutStr(gcpLoadBalancingMtlsClientCertURISans, mtlsInfo.ClientCertURISans, mtlsMap)
	shared.PutStr(gcpLoadBalancingMtlsClientCertDnsnameSans, mtlsInfo.ClientCertDnsnameSans, mtlsMap)
	shared.PutStr(string(conventions.TLSClientIssuerKey), mtlsInfo.ClientCertIssuerDn, mtlsMap)
	shared.PutStr(string(conventions.TLSClientSubjectKey), mtlsInfo.ClientCertSubjectDn, mtlsMap)
	shared.PutStr(gcpLoadBalancingMtlsClientCertLeaf, mtlsInfo.ClientCertLeaf, mtlsMap)
	shared.PutStr(string(conventions.TLSClientCertificateChainKey), mtlsInfo.ClientCertChain, mtlsMap)
}

func ParsePayloadIntoAttributes(payload []byte, attr pcommon.Map) error {
	var log loadbalancerlog
	if err := gojson.Unmarshal(payload, &log); err != nil {
		return fmt.Errorf("failed to unmarshal Load Balancer log: %w", err)
	}

	if err := isValid(&log); err != nil {
		return err
	}

	if err := handleRequestMetadata(&log, attr); err != nil {
		return fmt.Errorf("error handling request metadata: %w", err)
	}

	handleAuthPolicyInfo(log.AuthPolicyInfo, attr)
	handleTLSInfo(log.TLSInfo, attr)
	handleMtlsInfo(log.MtlsInfo, attr)

	// Handle embedded Armor log fields
	if err := handleArmorLogAttributes(&log.armorlog, attr); err != nil {
		return fmt.Errorf("error handling embedded Armor log fields: %w", err)
	}

	return nil
}
