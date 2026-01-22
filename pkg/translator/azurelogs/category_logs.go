// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azurelogs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/azurelogs"

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	gojson "github.com/goccy/go-json"
	"go.opentelemetry.io/collector/pdata/plog"
	conventions "go.opentelemetry.io/otel/semconv/v1.38.0"
)

const (
	noError = "NoError"

	categoryAzureCdnAccessLog                  = "AzureCdnAccessLog"
	categoryFrontDoorAccessLog                 = "FrontDoorAccessLog"
	categoryFrontDoorHealthProbeLog            = "FrontDoorHealthProbeLog"
	categoryFrontdoorWebApplicationFirewallLog = "FrontDoorWebApplicationFirewallLog"
	categoryAppServiceAppLogs                  = "AppServiceAppLogs"
	categoryAppServiceAuditLogs                = "AppServiceAuditLogs"
	categoryAppServiceAuthenticationLogs       = "AppServiceAuthenticationLogs"
	categoryAppServiceConsoleLogs              = "AppServiceConsoleLogs"
	categoryAppServiceHTTPLogs                 = "AppServiceHTTPLogs"
	categoryAppServiceIPSecAuditLogs           = "AppServiceIPSecAuditLogs"
	categoryAppServicePlatformLogs             = "AppServicePlatformLogs"
	categoryAdministrative                     = "Administrative"
	categoryAlert                              = "Alert"
	categoryAutoscale                          = "Autoscale"
	categorySecurity                           = "Security"
	categoryPolicy                             = "Policy"
	categoryRecommendation                     = "Recommendation"
	categoryServiceHealth                      = "ServiceHealth"
	categoryResourceHealth                     = "ResourceHealth"

	// attributeAzureRef holds the request tracking reference, also
	// placed in the request header "X-Azure-Ref".
	attributeAzureRef = "azure.ref"

	// attributeTimeToFirstByte holds the length of time in milliseconds
	// from when Microsoft service (CDN, Front Door, etc) receives the
	// request to the time the first byte gets sent to the client.
	attributeTimeToFirstByte = "azure.time_to_first_byte"

	// attributeDuration holds the length of time from first byte of
	// request to last byte of response out, in seconds.
	attributeDuration = "duration"

	// attributeAzurePop holds the point of presence (POP) that
	// processed the request
	attributeAzurePop = "azure.pop"

	// attributeCacheStatus holds the result of the cache hit/miss
	// at the POP
	attributeCacheStatus = "azure.cache_status"

	// attributeTLSServerName holds the server name indication (SNI)
	// value
	attributeTLSServerName = "tls.server.name"

	missingPort = "missing port in address"
)

const (
	// Identity specific attributes
	attributeIdentityAuthorizationScope  = "azure.identity.authorization.scope"
	attributeIdentityAuthorizationAction = "azure.identity.authorization.action"
	// Identity > authorization > evidence
	attributeIdentityAuthorizationEvidenceRole                = "azure.identity.authorization.evidence.role"
	attributeIdentityAuthorizationEvidenceRoleAssignmentScope = "azure.identity.authorization.evidence.role.assignment.scope"
	attributeIdentityAuthorizationEvidenceRoleAssignmentID    = "azure.identity.authorization.evidence.role.assignment.id"
	attributeIdentityAuthorizationEvidenceRoleDefinitionID    = "azure.identity.authorization.evidence.role.definition.id"
	attributeIdentityAuthorizationEvidencePrincipalID         = "azure.identity.authorization.evidence.principal.id"
	attributeIdentityAuthorizationEvidencePrincipalType       = "azure.identity.authorization.evidence.principal.type"
	// Identity > claims (standard JWT claims)
	attributeIdentityClaimsAudience  = "azure.identity.audience"
	attributeIdentityClaimsIssuer    = "azure.identity.issuer"
	attributeIdentityClaimsSubject   = "azure.identity.subject"
	attributeIdentityClaimsNotAfter  = "azure.identity.not_after"
	attributeIdentityClaimsNotBefore = "azure.identity.not_before"
	attributeIdentityClaimsCreated   = "azure.identity.created"
	// Identity > claims (Azure specific claims)
	attributeIdentityClaimsScope                 = "azure.identity.scope"
	attributeIdentityClaimsType                  = "azure.identity.type"
	attributeIdentityClaimsApplicationID         = "azure.identity.application.id"
	attributeIdentityClaimsAuthMethodsReferences = "azure.identity.auth.methods.references"
	attributeIdentityClaimsIdentifierObject      = "azure.identity.identifier.object"
	attributeIdentityClaimsIdentifierName        = "user.name"
	attributeIdentityClaimsProvider              = "azure.identity.provider"

	// azure front door WAF attributes

	// attributeAzureFrontDoorWAFRuleName holds the name of the WAF rule that
	// the request matched.
	attributeAzureFrontDoorWAFRuleName = "azure.frontdoor.waf.rule.name"

	// attributeAzureFrontDoorWAFPolicyName holds the name of the WAF policy
	// that processed the request.
	attributeAzureFrontDoorWAFPolicyName = "azure.frontdoor.waf.policy.name"

	// attributeAzureFrontDoorWAFPolicyMode holds the operations mode of the
	// WAF policy.
	attributeAzureFrontDoorWAFPolicyMode = "azure.frontdoor.waf.policy.mode"

	// attributeAzureFrontDoorWAFAction holds the action taken on the request.
	attributeAzureFrontDoorWAFAction = "azure.frontdoor.waf.action"

	// Administrative specific attributes
	attributeAzureAdministrativeEntity    = "azure.administrative.entity"
	attributeAzureAdministrativeMessage   = "azure.administrative.message"
	attributeAzureAdministrativeHierarchy = "azure.administrative.hierarchy"

	// Alert specific attributes
	attributeAzureAlertWebhookURI      = "azure.alert.webhook.uri"
	attributeAzureAlertRuleURI         = "azure.alert.rule.uri"
	attributeAzureAlertRuleName        = "azure.alert.rule.name"
	attributeAzureAlertRuleDescription = "azure.alert.rule.description"
	attributeAzureAlertThreshold       = "azure.alert.threshold"
	attributeAzureAlertWindowSize      = "azure.alert.window_size_minutes"
	attributeAzureAlertAggregation     = "azure.alert.aggregation"
	attributeAzureAlertOperator        = "azure.alert.operator"
	attributeAzureAlertMetricName      = "azure.alert.metric.name"
	attributeAzureAlertMetricUnit      = "azure.alert.metric.unit"

	// Autoscale specific attributes
	attributeAzureAutoscaleDescription     = "azure.autoscale.description"
	attributeAzureAutoscaleResourceName    = "azure.autoscale.resource.name"
	attributeAzureAutoscaleOldInstances    = "azure.autoscale.instances.previous_count"
	attributeAzureAutoscaleNewInstances    = "azure.autoscale.instances.count"
	attributeAzureAutoscaleLastScaleAction = "azure.autoscale.resource.last_scale"

	// Policy specific attributes
	attributeAzurePolicyIsComplianceCheck = "azure.policy.compliance_check"
	attributeAzurePolicyAncestors         = "azure.policy.ancestors"
	attributeAzurePolicyHierarchy         = "azure.policy.hierarchy"

	// Recommendation specific attributes
	attributeAzureRecommendationCategory      = "azure.recommendation.category"
	attributeAzureRecommendationImpact        = "azure.recommendation.impact"
	attributeAzureRecommendationName          = "azure.recommendation.name"
	attributeAzureRecommendationType          = "azure.recommendation.type"
	attributeAzureRecommendationSchemaVersion = "azure.recommendation.schema_version"
	attributeAzureRecommendationLink          = "azure.recommendation.link"

	// Security specific attributes
	attributeAzureSecurityAccountLogonID = "azure.security.account_logon_id"
	attributeAzureSecurityDomainName     = "azure.security.domain_name"
	attributeAzureSecurityActionTaken    = "azure.security.action_taken"
	attributeAzureSecuritySeverity       = "azure.security.severity"

	// Service Health specific attributes
	attributeAzureServiceHealthTitle                    = "azure.servicehealth.title"
	attributeAzureServiceHealthService                  = "azure.servicehealth.service"
	attributeAzureServiceHealthRegion                   = "azure.servicehealth.region"
	attributeAzureServiceHealthCommunicationID          = "azure.servicehealth.communication.id"
	attributeAzureServiceHealthCommunicationBody        = "azure.servicehealth.communication.body"
	attributeAzureServiceHealthCommunicationRouteType   = "azure.servicehealth.communication.route_type"
	attributeAzureServiceHealthIncidentType             = "azure.servicehealth.incident.type"
	attributeAzureServiceHealthTrackingID               = "azure.servicehealth.tracking.id"
	attributeAzureServiceHealthImpactStartTime          = "azure.servicehealth.impact.start"
	attributeAzureServiceHealthImpactMitigationTime     = "azure.servicehealth.impact.mitigation"
	attributeAzureServiceHealthImpactedServices         = "azure.servicehealth.impact.services"
	attributeAzureServiceHealthImpactType               = "azure.servicehealth.impact.type"
	attributeAzureServiceHealthImpactCategory           = "azure.servicehealth.impact.category"
	attributeAzureServiceHealthDefaultLanguageTitle     = "azure.servicehealth.default_language.title"
	attributeAzureServiceHealthDefaultLanguageContent   = "azure.servicehealth.default_language.content"
	attributeAzureServiceHealthState                    = "azure.servicehealth.state"
	attributeAzureServiceHealthMaintenanceID            = "azure.servicehealth.maintenance.id"
	attributeAzureServiceHealthMaintenanceType          = "azure.servicehealth.maintenance.type"
	attributeAzureServiceHealthIsHIR                    = "azure.servicehealth.is_hir"
	attributeAzureServiceHealthIsSynthetic              = "azure.servicehealth.is_synthetic"
	attributeAzureServiceHealthEmailTemplateID          = "azure.servicehealth.email.template.id"
	attributeAzureServiceHealthEmailTemplateFullVersion = "azure.servicehealth.email.template.full_version"
	attributeAzureServiceHealthEmailTemplateLocale      = "azure.servicehealth.email.template.locale"
	attributeAzureServiceHealthSMSText                  = "azure.servicehealth.sms.text"
	attributeAzureServiceHealthVersion                  = "azure.servicehealth.version"
	attributeAzureServiceHealthArgQuery                 = "azure.servicehealth.arg_query"
	attributeAzureServiceHealthRateNew                  = "azure.servicehealth.new_rate"
	attributeAzureServiceHealthRateOld                  = "azure.servicehealth.old_rate"

	// Resource Health specific attributes
	attributeAzureResourceHealthTitle                = "azure.resourcehealth.title"
	attributeAzureResourceHealthDetails              = "azure.resourcehealth.details"
	attributeAzureResourceHealthCurrentHealthStatus  = "azure.resourcehealth.state"
	attributeAzureResourceHealthPreviousHealthStatus = "azure.resourcehealth.previous_state"
	attributeAzureResourceHealthType                 = "azure.resourcehealth.type"
	attributeAzureResourceHealthCause                = "azure.resourcehealth.cause"
)

var (
	errStillToImplement    = errors.New("still to implement")
	errUnsupportedCategory = errors.New("category not supported")
)

func addRecordAttributes(category string, data []byte, record plog.LogRecord) error {
	var err error

	switch category {
	case categoryAzureCdnAccessLog:
		err = addAzureCdnAccessLogProperties(data, record)
	case categoryFrontDoorAccessLog:
		err = addFrontDoorAccessLogProperties(data, record)
	case categoryFrontDoorHealthProbeLog:
		err = addFrontDoorHealthProbeLogProperties(data, record)
	case categoryFrontdoorWebApplicationFirewallLog:
		err = addFrontDoorWAFLogProperties(data, record)
	case categoryAppServiceAppLogs:
		err = addAppServiceAppLogsProperties(data, record)
	case categoryAppServiceAuditLogs:
		err = addAppServiceAuditLogsProperties(data, record)
	case categoryAppServiceAuthenticationLogs:
		err = addAppServiceAuthenticationLogsProperties(data, record)
	case categoryAppServiceConsoleLogs:
		err = addAppServiceConsoleLogsProperties(data, record)
	case categoryAppServiceHTTPLogs:
		err = addAppServiceHTTPLogsProperties(data, record)
	case categoryAppServiceIPSecAuditLogs:
		err = addAppServiceIPSecAuditLogsProperties(data, record)
	case categoryAppServicePlatformLogs:
		err = addAppServicePlatformLogsProperties(data, record)
	case categoryAdministrative:
		err = addAdministrativeLogProperties(data, record)
	case categoryAlert:
		err = addAlertLogProperties(data, record)
	case categoryAutoscale:
		err = addAutoscaleLogProperties(data, record)
	case categorySecurity:
		err = addSecurityLogProperties(data, record)
	case categoryPolicy:
		err = addPolicyLogProperties(data, record)
	case categoryRecommendation:
		err = addRecommendationLogProperties(data, record)
	case categoryServiceHealth:
		err = addServiceHealthLogProperties(data, record)
	case categoryResourceHealth:
		err = addResourceHealthLogProperties(data, record)
	default:
		err = errUnsupportedCategory
	}

	if err != nil {
		return fmt.Errorf("failed to parse logs from category %q: %w", category, err)
	}
	return nil
}

// putInt parses value as an int and puts it in the record
func putInt(field, value string, record plog.LogRecord) error {
	n, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return fmt.Errorf("failed to get number in %q for field %q: %w", value, field, err)
	}
	record.Attributes().PutInt(field, n)
	return nil
}

// putStr puts the value in the record if the value holds
// meaningful data. Meaningful data is defined as not being empty
// or "N/A".
func putStr(field, value string, record plog.LogRecord) {
	switch value {
	case "", "N/A":
		// ignore
	default:
		record.Attributes().PutStr(field, value)
	}
}

func putBool(field, value string, record plog.LogRecord) {
	if b, err := strconv.ParseBool(value); err == nil {
		record.Attributes().PutBool(field, b)
	}
}

// handleTime parses the time value and always multiplies it by
// 1e3. This is so we don't loose so much data if the time is for
// example "0.154". In that case, the output would be "154".
func handleTime(field, value string, record plog.LogRecord) error {
	n, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return fmt.Errorf("failed to get number in %q for field %q: %w", value, field, err)
	}
	ns := int64(n * 1e3)
	record.Attributes().PutInt(field, ns)
	return nil
}

// See https://github.com/MicrosoftDocs/azure-docs/blob/main/articles/cdn/monitoring-and-access-log.md
type azureCdnAccessLogProperties struct {
	TrackingReference    string `json:"trackingReference"`
	HTTPMethod           string `json:"httpMethod"`
	HTTPVersion          string `json:"httpVersion"`
	RequestURI           string `json:"requestUri"`
	SNI                  string `json:"sni"`
	RequestBytes         string `json:"requestBytes"`
	ResponseBytes        string `json:"responseBytes"`
	UserAgent            string `json:"userAgent"`
	ClientIP             string `json:"clientIp"`
	ClientPort           string `json:"clientPort"`
	SocketIP             string `json:"socketIp"`
	TimeToFirstByte      string `json:"timeToFirstByte"`
	TimeTaken            string `json:"timeTaken"`
	RequestProtocol      string `json:"requestProtocol"`
	SecurityProtocol     string `json:"securityProtocol"`
	HTTPStatusCode       string `json:"httpStatusCode"`
	Pop                  string `json:"pop"`
	CacheStatus          string `json:"cacheStatus"`
	ErrorInfo            string `json:"errorInfo"`
	ErrorInfo1           string `json:"ErrorInfo"`
	Endpoint             string `json:"endpoint"`
	IsReceivedFromClient bool   `json:"isReceivedFromClient"`
	BackendHostname      string `json:"backendHostname"`
}

// addRequestURIProperties parses the request URI and adds the
// relevant attributes to the record
func addRequestURIProperties(uri string, record plog.LogRecord) error {
	if uri == "" {
		return nil
	}

	u, errURL := url.Parse(uri)
	if errURL != nil {
		return fmt.Errorf("failed to parse request URI %q: %w", uri, errURL)
	}
	record.Attributes().PutStr(string(conventions.URLOriginalKey), uri)

	if port := u.Port(); port != "" {
		if err := putInt(string(conventions.URLPortKey), u.Port(), record); err != nil {
			return fmt.Errorf("failed to get port number from value %q: %w", port, err)
		}
	}

	putStr(string(conventions.URLSchemeKey), u.Scheme, record)
	putStr(string(conventions.URLPathKey), u.Path, record)
	putStr(string(conventions.URLQueryKey), u.RawQuery, record)
	putStr(string(conventions.URLFragmentKey), u.Fragment, record)

	return nil
}

// addSecurityProtocolProperties based on the security protocol
func addSecurityProtocolProperties(securityProtocol string, record plog.LogRecord) error {
	if securityProtocol == "" {
		return nil
	}
	name, remaining, _ := strings.Cut(securityProtocol, " ")
	if remaining == "" {
		return fmt.Errorf(`security protocol %q is missing version, expects format "<name> <version>"`, securityProtocol)
	}
	version, remaining, _ := strings.Cut(remaining, " ")
	if remaining != "" {
		return fmt.Errorf(`security protocol %q has invalid format, expects "<name> <version>"`, securityProtocol)
	}

	record.Attributes().PutStr(string(conventions.TLSProtocolNameKey), name)
	record.Attributes().PutStr(string(conventions.TLSProtocolVersionKey), version)

	return nil
}

// addErrorInfoProperties checks if there is an error and adds it
// to the record attributes as an exception in case it exists.
func addErrorInfoProperties(errorInfo string, record plog.LogRecord) {
	if errorInfo == noError {
		return
	}
	record.Attributes().PutStr(string(conventions.ExceptionTypeKey), errorInfo)
}

// handleDestination puts the value for the backend host name and endpoint
// in the expected field names. If backend hostname is empty, then the
// destination address and port depend only on the endpoint. If backend
// hostname is filled, then the destination address and port are based on
// it. If both are filled but different, then the endpoint will cover the
// network address and port.
func handleDestination(backendHostname, endpoint string, record plog.LogRecord) error {
	addFields := func(full, addressField, portField string) error {
		host, port, err := net.SplitHostPort(full)
		if err != nil && strings.HasSuffix(err.Error(), missingPort) {
			// there is no port, so let's keep using the full endpoint for the address
			host = full
		} else if err != nil {
			return err
		}
		if host != "" {
			record.Attributes().PutStr(addressField, host)
		}
		if port != "" {
			err = putInt(portField, port, record)
			if err != nil {
				return err
			}
		}
		return nil
	}

	if backendHostname == "" {
		if endpoint == "" {
			return nil
		}
		err := addFields(endpoint, string(conventions.DestinationAddressKey), string(conventions.DestinationPortKey))
		if err != nil {
			return fmt.Errorf("failed to parse endpoint %q: %w", endpoint, err)
		}
	} else {
		err := addFields(backendHostname, string(conventions.DestinationAddressKey), string(conventions.DestinationPortKey))
		if err != nil {
			return fmt.Errorf("failed to parse backend hostname %q: %w", backendHostname, err)
		}

		if endpoint != backendHostname && endpoint != "" {
			err = addFields(endpoint, string(conventions.NetworkPeerAddressKey), string(conventions.NetworkPeerPortKey))
			if err != nil {
				return fmt.Errorf("failed to parse endpoint %q: %w", endpoint, err)
			}
		}
	}

	return nil
}

// addAzureCdnAccessLogProperties parses the Azure CDN access log, and adds
// the relevant attributes to the record
func addAzureCdnAccessLogProperties(data []byte, record plog.LogRecord) error {
	var properties azureCdnAccessLogProperties
	if err := gojson.Unmarshal(data, &properties); err != nil {
		return fmt.Errorf("failed to parse AzureCdnAccessLog properties: %w", err)
	}

	if err := putInt(string(conventions.HTTPRequestSizeKey), properties.RequestBytes, record); err != nil {
		return err
	}
	if err := putInt(string(conventions.HTTPResponseSizeKey), properties.ResponseBytes, record); err != nil {
		return err
	}
	if err := putInt(string(conventions.ClientPortKey), properties.ClientPort, record); err != nil {
		return err
	}
	if err := putInt(string(conventions.HTTPResponseStatusCodeKey), properties.HTTPStatusCode, record); err != nil {
		return err
	}

	if err := handleTime(attributeTimeToFirstByte, properties.TimeToFirstByte, record); err != nil {
		return err
	}
	if err := handleTime(attributeDuration, properties.TimeTaken, record); err != nil {
		return err
	}

	if err := addRequestURIProperties(properties.RequestURI, record); err != nil {
		return fmt.Errorf(`failed to handle "requestUri" field: %w`, err)
	}
	if err := addSecurityProtocolProperties(properties.SecurityProtocol, record); err != nil {
		return err
	}
	if err := handleDestination(properties.BackendHostname, properties.Endpoint, record); err != nil {
		return err
	}

	if properties.ErrorInfo != properties.ErrorInfo1 && properties.ErrorInfo != "" && properties.ErrorInfo1 != "" {
		return errors.New(`unexpected: "errorInfo" and "ErrorInfo" JSON fields have different values`)
	}
	if properties.ErrorInfo1 != "" {
		addErrorInfoProperties(properties.ErrorInfo1, record)
	} else if properties.ErrorInfo != "" {
		addErrorInfoProperties(properties.ErrorInfo, record)
	}

	putStr(attributeAzureRef, properties.TrackingReference, record)
	putStr(string(conventions.HTTPRequestMethodKey), properties.HTTPMethod, record)
	putStr(string(conventions.NetworkProtocolVersionKey), properties.HTTPVersion, record)
	putStr(string(conventions.NetworkProtocolNameKey), properties.RequestProtocol, record)
	putStr(attributeTLSServerName, properties.SNI, record)
	putStr(string(conventions.UserAgentOriginalKey), properties.UserAgent, record)
	putStr(string(conventions.ClientAddressKey), properties.ClientIP, record)
	putStr(string(conventions.SourceAddressKey), properties.SocketIP, record)

	putStr(attributeAzurePop, properties.Pop, record)
	putStr(attributeCacheStatus, properties.CacheStatus, record)

	if properties.IsReceivedFromClient {
		record.Attributes().PutStr(string(conventions.NetworkIODirectionKey), "receive")
	} else {
		record.Attributes().PutStr(string(conventions.NetworkIODirectionKey), "transmit")
	}

	return nil
}

// See https://learn.microsoft.com/en-us/azure/frontdoor/monitor-front-door?pivots=front-door-standard-premium#access-log.
type frontDoorAccessLog struct {
	TrackingReference string `json:"trackingReference"`
	HTTPMethod        string `json:"httpMethod"`
	HTTPVersion       string `json:"httpVersion"`
	RequestURI        string `json:"requestUri"`
	SNI               string `json:"sni"`
	RequestBytes      string `json:"requestBytes"`
	ResponseBytes     string `json:"responseBytes"`
	UserAgent         string `json:"userAgent"`
	ClientIP          string `json:"clientIp"`
	ClientPort        string `json:"clientPort"`
	SocketIP          string `json:"socketIp"`
	TimeToFirstByte   string `json:"timeToFirstByte"`
	TimeTaken         string `json:"timeTaken"`
	RequestProtocol   string `json:"requestProtocol"`
	SecurityProtocol  string `json:"securityProtocol"`
	HTTPStatusCode    string `json:"httpStatusCode"`
	Pop               string `json:"pop"`
	CacheStatus       string `json:"cacheStatus"`
	ErrorInfo         string `json:"errorInfo"`
	ErrorInfo1        string `json:"ErrorInfo"`
	Result            string `json:"result"`
	Endpoint          string `json:"endpoint"`
	HostName          string `json:"hostName"`
	SecurityCipher    string `json:"securityCipher"`
	SecurityCurves    string `json:"securityCurves"`
	OriginIP          string `json:"originIp"`
}

// addFrontDoorAccessLogProperties parses the Front Door access log, and adds
// the relevant attributes to the record
func addFrontDoorAccessLogProperties(data []byte, record plog.LogRecord) error {
	var properties frontDoorAccessLog
	if err := gojson.Unmarshal(data, &properties); err != nil {
		return fmt.Errorf("failed to parse FrontDoorAccessLog properties: %w", err)
	}

	if err := putInt(string(conventions.HTTPRequestSizeKey), properties.RequestBytes, record); err != nil {
		return err
	}
	if err := putInt(string(conventions.HTTPResponseSizeKey), properties.ResponseBytes, record); err != nil {
		return err
	}
	if err := putInt(string(conventions.ClientPortKey), properties.ClientPort, record); err != nil {
		return err
	}
	if err := putInt(string(conventions.HTTPResponseStatusCodeKey), properties.HTTPStatusCode, record); err != nil {
		return err
	}

	if err := handleTime(attributeTimeToFirstByte, properties.TimeToFirstByte, record); err != nil {
		return err
	}
	if err := handleTime(attributeDuration, properties.TimeTaken, record); err != nil {
		return err
	}

	if err := addRequestURIProperties(properties.RequestURI, record); err != nil {
		return fmt.Errorf(`failed to handle "requestUri" field: %w`, err)
	}
	if err := addSecurityProtocolProperties(properties.SecurityProtocol, record); err != nil {
		return err
	}
	if err := handleDestination(properties.HostName, properties.Endpoint, record); err != nil {
		return err
	}

	if properties.ErrorInfo != properties.ErrorInfo1 && properties.ErrorInfo != "" && properties.ErrorInfo1 != "" {
		return errors.New(`unexpected: "errorInfo" and "ErrorInfo" JSON fields have different values`)
	}
	if properties.ErrorInfo1 != "" {
		addErrorInfoProperties(properties.ErrorInfo1, record)
	} else if properties.ErrorInfo != "" {
		addErrorInfoProperties(properties.ErrorInfo, record)
	}

	if properties.OriginIP != "" && properties.OriginIP != "N/A" {
		address, port, _ := strings.Cut(properties.OriginIP, ":")
		putStr(string(conventions.ServerAddressKey), address, record)
		if port != "" {
			if err := putInt(string(conventions.ServerPortKey), port, record); err != nil {
				return err
			}
		}
	}

	putStr(attributeAzureRef, properties.TrackingReference, record)
	putStr(string(conventions.HTTPRequestMethodKey), properties.HTTPMethod, record)
	putStr(string(conventions.NetworkProtocolVersionKey), properties.HTTPVersion, record)
	putStr(string(conventions.NetworkProtocolNameKey), properties.RequestProtocol, record)
	putStr(attributeTLSServerName, properties.SNI, record)
	putStr(string(conventions.UserAgentOriginalKey), properties.UserAgent, record)
	putStr(string(conventions.ClientAddressKey), properties.ClientIP, record)
	putStr(string(conventions.SourceAddressKey), properties.SocketIP, record)

	putStr(attributeAzurePop, properties.Pop, record)
	putStr(attributeCacheStatus, properties.CacheStatus, record)

	putStr(string(conventions.TLSCurveKey), properties.SecurityCurves, record)
	putStr(string(conventions.TLSCipherKey), properties.SecurityCipher, record)

	return nil
}

// addFrontDoorHealthProbeLogProperties parses the Front Door access log, and adds
// the relevant attributes to the record
func addFrontDoorHealthProbeLogProperties(_ []byte, _ plog.LogRecord) error {
	// TODO @constanca-m implement this the same way as addAzureCdnAccessLogProperties
	return errStillToImplement
}

// See https://learn.microsoft.com/en-us/azure/web-application-firewall/afds/waf-front-door-monitor?pivots=front-door-standard-premium#waf-logs
type frontDoorWAFLogProperties struct {
	ClientIP          string `json:"clientIP"`
	ClientPort        string `json:"clientPort"`
	SocketIP          string `json:"socketIP"`
	RequestURI        string `json:"requestUri"`
	RuleName          string `json:"ruleName"`
	Policy            string `json:"policy"`
	Action            string `json:"action"`
	Host              string `json:"host"`
	TrackingReference string `json:"trackingReference"`
	PolicyMode        string `json:"policyMode"`
}

// addFrontDoorWAFLogProperties parses the Front Door access log, and adds
// the relevant attributes to the record
func addFrontDoorWAFLogProperties(data []byte, record plog.LogRecord) error {
	var properties frontDoorWAFLogProperties
	if err := gojson.Unmarshal(data, &properties); err != nil {
		return fmt.Errorf("failed to parse AzureCdnAccessLog properties: %w", err)
	}

	if err := putInt(string(conventions.ClientPortKey), properties.ClientPort, record); err != nil {
		return err
	}

	if err := addRequestURIProperties(properties.RequestURI, record); err != nil {
		return fmt.Errorf(`failed to handle "requestUri" field: %w`, err)
	}

	putStr(string(conventions.ClientAddressKey), properties.ClientIP, record)
	putStr(string(conventions.SourceAddressKey), properties.SocketIP, record)
	putStr(attributeAzureRef, properties.TrackingReference, record)
	putStr("http.request.header.host", properties.Host, record)
	putStr(attributeAzureFrontDoorWAFPolicyName, properties.Policy, record)
	putStr(attributeAzureFrontDoorWAFPolicyMode, properties.PolicyMode, record)
	putStr(attributeAzureFrontDoorWAFRuleName, properties.RuleName, record)
	putStr(attributeAzureFrontDoorWAFAction, properties.Action, record)

	return nil
}

// addAppServiceAppLogsProperties parses the App Service access log, and adds
// the relevant attributes to the record
func addAppServiceAppLogsProperties(_ []byte, _ plog.LogRecord) error {
	// TODO @constanca-m implement this the same way as addAzureCdnAccessLogProperties
	return errStillToImplement
}

// addAppServiceAuditLogsProperties parses the App Service access log, and adds
// the relevant attributes to the record
func addAppServiceAuditLogsProperties(_ []byte, _ plog.LogRecord) error {
	// TODO @constanca-m implement this the same way as addAzureCdnAccessLogProperties
	return errStillToImplement
}

// addAppServiceAuthenticationLogsProperties parses the App Service access log, and adds
// the relevant attributes to the record
func addAppServiceAuthenticationLogsProperties(_ []byte, _ plog.LogRecord) error {
	// TODO @constanca-m implement this the same way as addAzureCdnAccessLogProperties
	return errStillToImplement
}

// addAppServiceConsoleLogsProperties parses the App Service access log, and adds
// the relevant attributes to the record
func addAppServiceConsoleLogsProperties(_ []byte, _ plog.LogRecord) error {
	// TODO @constanca-m implement this the same way as addAzureCdnAccessLogProperties
	return errStillToImplement
}

// addAppServiceHTTPLogsProperties parses the App Service access log, and adds
// the relevant attributes to the record
func addAppServiceHTTPLogsProperties(_ []byte, _ plog.LogRecord) error {
	// TODO @constanca-m implement this the same way as addAzureCdnAccessLogProperties
	return errStillToImplement
}

// addAppServiceIPSecAuditLogsProperties parses the App Service access log, and adds
// the relevant attributes to the record
func addAppServiceIPSecAuditLogsProperties(_ []byte, _ plog.LogRecord) error {
	// TODO @constanca-m implement this the same way as addAzureCdnAccessLogProperties
	return errStillToImplement
}

// addAppServicePlatformLogsProperties parses the App Service access log, and adds
// the relevant attributes to the record
func addAppServicePlatformLogsProperties(_ []byte, _ plog.LogRecord) error {
	// TODO @constanca-m implement this the same way as addAzureCdnAccessLogProperties
	return errStillToImplement
}

// ------------------------------------------------------------
// Activity Log - Administrative category
// ------------------------------------------------------------

// administrativeLogProperties represents the properties field of an Administrative activity log.
// See: https://learn.microsoft.com/en-us/azure/azure-monitor/essentials/activity-log-schema#administrative-category
type administrativeLogProperties struct {
	Entity    string `json:"entity"`
	Message   string `json:"message"`
	Hierarchy string `json:"hierarchy"`
}

// addAdministrativeLogProperties parses Administrative activity logs
// and maps them to OpenTelemetry semantic conventions.
// See: https://learn.microsoft.com/en-us/azure/azure-monitor/essentials/activity-log-schema#administrative-category
func addAdministrativeLogProperties(data []byte, record plog.LogRecord) error {
	var properties administrativeLogProperties
	if err := gojson.Unmarshal(data, &properties); err != nil {
		return fmt.Errorf("failed to parse Administrative properties: %w", err)
	}

	putStr(attributeAzureAdministrativeEntity, properties.Entity, record)
	putStr(attributeAzureAdministrativeMessage, properties.Message, record)
	putStr(attributeAzureAdministrativeHierarchy, properties.Hierarchy, record)

	return nil
}

// ------------------------------------------------------------
// Activity Log - Alert category
// ------------------------------------------------------------

// alertLogProperties represents the properties field of an Alert activity log.
// See: https://learn.microsoft.com/en-us/azure/azure-monitor/essentials/activity-log-schema#alert-category
type alertLogProperties struct {
	WebHookURI          string `json:"webHookUri"`
	RuleURI             string `json:"RuleUri"`
	RuleName            string `json:"RuleName"`
	RuleDescription     string `json:"RuleDescription"`
	Threshold           string `json:"Threshold"`
	WindowSizeInMinutes string `json:"WindowSizeInMinutes"`
	Aggregation         string `json:"Aggregation"`
	Operator            string `json:"Operator"`
	MetricName          string `json:"MetricName"`
	MetricUnit          string `json:"MetricUnit"`
}

// addAlertLogProperties parses Alert activity logs
// and maps them to OpenTelemetry semantic conventions.
// See: https://learn.microsoft.com/en-us/azure/azure-monitor/essentials/activity-log-schema#alert-category
func addAlertLogProperties(data []byte, record plog.LogRecord) error {
	var properties alertLogProperties
	if err := gojson.Unmarshal(data, &properties); err != nil {
		return fmt.Errorf("failed to parse Alert properties: %w", err)
	}

	putStr(attributeAzureAlertWebhookURI, properties.WebHookURI, record)
	putStr(attributeAzureAlertRuleURI, properties.RuleURI, record)
	putStr(attributeAzureAlertRuleName, properties.RuleName, record)
	putStr(attributeAzureAlertRuleDescription, properties.RuleDescription, record)
	putStr(attributeAzureAlertThreshold, properties.Threshold, record)
	putStr(attributeAzureAlertWindowSize, properties.WindowSizeInMinutes, record)
	putStr(attributeAzureAlertAggregation, properties.Aggregation, record)
	putStr(attributeAzureAlertOperator, properties.Operator, record)
	putStr(attributeAzureAlertMetricName, properties.MetricName, record)
	putStr(attributeAzureAlertMetricUnit, properties.MetricUnit, record)

	return nil
}

// ------------------------------------------------------------
// Activity Log - Autoscale category
// ------------------------------------------------------------

// autoscaleLogProperties represents the properties field of an Autoscale activity log.
// See: https://learn.microsoft.com/en-us/azure/azure-monitor/essentials/activity-log-schema#autoscale-category
type autoscaleLogProperties struct {
	Description         string `json:"Description"`
	ResourceName        string `json:"ResourceName"`
	OldInstancesCount   string `json:"OldInstancesCount"`
	NewInstancesCount   string `json:"NewInstancesCount"`
	LastScaleActionTime string `json:"LastScaleActionTime"`
}

// addAutoscaleLogProperties parses Autoscale activity logs
// and maps them to OpenTelemetry semantic conventions.
// See: https://learn.microsoft.com/en-us/azure/azure-monitor/essentials/activity-log-schema#autoscale-category
func addAutoscaleLogProperties(data []byte, record plog.LogRecord) error {
	var properties autoscaleLogProperties
	if err := gojson.Unmarshal(data, &properties); err != nil {
		return fmt.Errorf("failed to parse Autoscale properties: %w", err)
	}

	putStr(attributeAzureAutoscaleDescription, properties.Description, record)
	putStr(attributeAzureAutoscaleResourceName, properties.ResourceName, record)
	putStr(attributeAzureAutoscaleOldInstances, properties.OldInstancesCount, record)
	putStr(attributeAzureAutoscaleNewInstances, properties.NewInstancesCount, record)
	putStr(attributeAzureAutoscaleLastScaleAction, properties.LastScaleActionTime, record)

	return nil
}

// ------------------------------------------------------------
// Activity Log - Policy category
// ------------------------------------------------------------

type policyElement struct {
	DefinitionID             string   `json:"policyDefinitionId"`
	SetDefinitionID          string   `json:"policySetDefinitionId"`
	ReferenceID              string   `json:"policyDefinitionReferenceId"`
	SetDefinitionName        string   `json:"policySetDefinitionName"`
	SetDefinitionDisplayName string   `json:"policySetDefinitionDisplayName"`
	SetDefinitionVersion     string   `json:"policySetDefinitionVersion"`
	DefinitionName           string   `json:"policyDefinitionName"`
	DefinitionDisplayName    string   `json:"policyDefinitionDisplayName"`
	DefinitionVersion        string   `json:"policyDefinitionVersion"`
	DefinitionEffect         string   `json:"policyDefinitionEffect"`
	AssignmentID             string   `json:"policyAssignmentId"`
	AssignmentName           string   `json:"policyAssignmentName"`
	AssignmentDisplayName    string   `json:"policyAssignmentDisplayName"`
	AssignmentScope          string   `json:"policyAssignmentScope"`
	ExemptionIDs             []string `json:"policyExemptionIds"`
	AssignmentIDs            []string `json:"policyAssignmentIds"`
}

type policyLogProperties struct {
	IsComplianceCheck string `json:"isComplianceCheck"`
	ResourceLocation  string `json:"resourceLocation"`
	Ancestors         string `json:"ancestors"`
	Policies          string `json:"policies"`
	Hierarchy         string `json:"hierarchy"`
}

func addPolicyLogProperties(data []byte, record plog.LogRecord) error {
	var properties policyLogProperties
	if err := gojson.Unmarshal(data, &properties); err != nil {
		return fmt.Errorf("failed to parse Policy properties: %w", err)
	}

	// check if Policies is a string and unmarshal the embedded JSON
	// object in the `policyElement` struct
	var policies []policyElement
	if err := gojson.Unmarshal([]byte(properties.Policies), &policies); err != nil {
		return fmt.Errorf("failed to parse Policy properties: %w", err)
	}

	putBool(attributeAzurePolicyIsComplianceCheck, properties.IsComplianceCheck, record)
	putStr(attributeAzureLocation, properties.ResourceLocation, record)
	putStr(attributeAzurePolicyAncestors, properties.Ancestors, record)
	putStr(attributeAzurePolicyHierarchy, properties.Hierarchy, record)

	// Add policies as a slice of maps
	if len(policies) > 0 {
		policiesSlice := record.Attributes().PutEmptySlice("azure.policy.policies")
		for i := range policies {
			policyMap := policiesSlice.AppendEmpty().SetEmptyMap()
			policyMap.PutStr("definition.id", policies[i].DefinitionID)
			policyMap.PutStr("definition.name", policies[i].DefinitionName)
			policyMap.PutStr("definition.display_name", policies[i].DefinitionDisplayName)
			policyMap.PutStr("definition.version", policies[i].DefinitionVersion)
			policyMap.PutStr("definition.effect", policies[i].DefinitionEffect)
			policyMap.PutStr("definition.reference_id", policies[i].ReferenceID)
			policyMap.PutStr("set_definition.id", policies[i].SetDefinitionID)
			policyMap.PutStr("set_definition.name", policies[i].SetDefinitionName)
			policyMap.PutStr("set_definition.display_name", policies[i].SetDefinitionDisplayName)
			policyMap.PutStr("set_definition.version", policies[i].SetDefinitionVersion)
			policyMap.PutStr("assignment.id", policies[i].AssignmentID)
			policyMap.PutStr("assignment.name", policies[i].AssignmentName)
			policyMap.PutStr("assignment.display_name", policies[i].AssignmentDisplayName)
			policyMap.PutStr("assignment.scope", policies[i].AssignmentScope)
		}
	}

	return nil
}

// ------------------------------------------------------------
// Activity Log - Recommendation category
// ------------------------------------------------------------

// recommendationLogProperties represents the properties field of a Recommendation activity log.
// See: https://learn.microsoft.com/en-us/azure/azure-monitor/essentials/activity-log-schema#recommendation-category
type recommendationLogProperties struct {
	RecommendationSchemaVersion string `json:"recommendationSchemaVersion"`
	RecommendationCategory      string `json:"recommendationCategory"`
	RecommendationImpact        string `json:"recommendationImpact"`
	RecommendationName          string `json:"recommendationName"`
	RecommendationResourceLink  string `json:"recommendationResourceLink"`
	RecommendationType          string `json:"recommendationType"`
}

// addRecommendationLogProperties parses Recommendation activity logs from Azure Advisor
// and maps them to OpenTelemetry semantic conventions.
// See: https://learn.microsoft.com/en-us/azure/azure-monitor/essentials/activity-log-schema#recommendation-category
func addRecommendationLogProperties(data []byte, record plog.LogRecord) error {
	var properties recommendationLogProperties
	if err := gojson.Unmarshal(data, &properties); err != nil {
		return fmt.Errorf("failed to parse Recommendation properties: %w", err)
	}

	putStr(attributeAzureRecommendationCategory, properties.RecommendationCategory, record)
	putStr(attributeAzureRecommendationImpact, properties.RecommendationImpact, record)
	putStr(attributeAzureRecommendationName, properties.RecommendationName, record)
	putStr(attributeAzureRecommendationType, properties.RecommendationType, record)
	putStr(attributeAzureRecommendationSchemaVersion, properties.RecommendationSchemaVersion, record)
	putStr(attributeAzureRecommendationLink, properties.RecommendationResourceLink, record)

	return nil
}

// ------------------------------------------------------------
// Activity Log - Security category
// ------------------------------------------------------------

// securityLogProperties represents the properties field of a Security activity log.
// See: https://learn.microsoft.com/en-us/azure/azure-monitor/essentials/activity-log-schema#security-category
type securityLogProperties struct {
	AccountLogonID  string `json:"accountLogonId"`
	CommandLine     string `json:"commandLine"`
	DomainName      string `json:"domainName"`
	ParentProcess   string `json:"parentProcess"`
	ParentProcessID string `json:"parentProcess id"`
	ProcessID       string `json:"processId"`
	ProcessName     string `json:"processName"`
	UserName        string `json:"userName"`
	UserSID         string `json:"UserSID"`
	ActionTaken     string `json:"ActionTaken"`
	Severity        string `json:"Severity"`
}

// addSecurityLogProperties parses Security activity logs
// and maps them to OpenTelemetry semantic conventions.
// See: https://learn.microsoft.com/en-us/azure/azure-monitor/essentials/activity-log-schema#security-category
func addSecurityLogProperties(data []byte, record plog.LogRecord) error {
	var properties securityLogProperties
	if err := gojson.Unmarshal(data, &properties); err != nil {
		return fmt.Errorf("failed to parse Security properties: %w", err)
	}

	// Map to OTel process semantic conventions
	putStr(string(conventions.ProcessCommandLineKey), properties.CommandLine, record)
	if err := putInt(string(conventions.ProcessPIDKey), properties.ProcessID, record); err != nil {
		return err
	}
	if err := putInt(string(conventions.ProcessParentPIDKey), properties.ParentProcessID, record); err != nil {
		return err
	}
	putStr(string(conventions.ProcessExecutablePathKey), properties.ProcessName, record)
	putStr(string(conventions.ProcessOwnerKey), properties.UserName, record)
	putStr(string(conventions.EnduserIDKey), properties.UserSID, record)

	// Azure-specific fields that don't have OTel equivalents
	putStr(attributeAzureSecurityAccountLogonID, properties.AccountLogonID, record)
	putStr(attributeAzureSecurityDomainName, properties.DomainName, record)
	putStr(attributeAzureSecurityActionTaken, properties.ActionTaken, record)
	putStr(attributeAzureSecuritySeverity, properties.Severity, record)

	return nil
}

// ------------------------------------------------------------
// Activity Log - Service Health category
// ------------------------------------------------------------

// serviceHealthLogProperties represents the properties field of a Service Health activity log.
// See: https://learn.microsoft.com/en-us/azure/azure-monitor/essentials/activity-log-schema#service-health-category
type serviceHealthLogProperties struct {
	Title                  string `json:"title"`
	Service                string `json:"service"`
	Region                 string `json:"region"`
	CommunicationText      string `json:"communication"`
	CommunicationID        string `json:"communicationId"`
	IncidentType           string `json:"incidentType"`
	TrackingID             string `json:"trackingId"`
	ImpactStartTime        string `json:"impactStartTime"`
	ImpactMitigationTime   string `json:"impactMitigationTime"`
	ImpactedServices       string `json:"impactedServices"`
	DefaultLanguageTitle   string `json:"defaultLanguageTitle"`
	DefaultLanguageContent string `json:"defaultLanguageContent"`
	Stage                  string `json:"stage"`
	MaintenanceID          string `json:"maintenanceId"`
	MaintenanceType        string `json:"maintenanceType"`
	IsHIR                  bool   `json:"isHIR"`
	IsSynthetic            string `json:"IsSynthetic"`
	ImpactType             string `json:"impactType"`
	ImpactCategory         string `json:"impactCategory"`
}

type impactedService struct {
	Name    string `json:"ServiceName"`
	ID      string `json:"ServiceId"`
	GUID    string `json:"ServiceGuid"`
	Regions []struct {
		Name string `json:"RegionName"`
		ID   string `json:"RegionId"`
	} `json:"ImpactedRegions"`
}

// addServiceHealthLogProperties parses Service Health activity logs
// and maps them to OpenTelemetry semantic conventions.
// See: https://learn.microsoft.com/en-us/azure/azure-monitor/essentials/activity-log-schema#service-health-category
func addServiceHealthLogProperties(data []byte, record plog.LogRecord) error {
	var properties serviceHealthLogProperties
	if err := gojson.Unmarshal(data, &properties); err != nil {
		return fmt.Errorf("failed to parse Service Health properties: %w", err)
	}

	// check if Policies is a string and unmarshal the embedded JSON
	// object in the `policyElement` struct
	var impactedServices []impactedService
	if err := gojson.Unmarshal([]byte(properties.ImpactedServices), &impactedServices); err != nil {
		return fmt.Errorf("failed to parse Impacted Services properties: %w", err)
	}

	// Add impacted services as a slice of maps
	if len(impactedServices) > 0 {
		impactedServicesSlice := record.Attributes().PutEmptySlice(attributeAzureServiceHealthImpactedServices)
		for _, s := range impactedServices {
			impactedServiceMap := impactedServicesSlice.AppendEmpty().SetEmptyMap()
			impactedServiceMap.PutStr("name", s.Name)
			impactedServiceMap.PutStr("id", s.ID)
			impactedServiceMap.PutStr("guid", s.GUID)

			if len(s.Regions) > 0 {
				regionsSlice := impactedServiceMap.PutEmptySlice("regions")
				for _, r := range s.Regions {
					regionMap := regionsSlice.AppendEmpty().SetEmptyMap()
					regionMap.PutStr("name", r.Name)
					regionMap.PutStr("id", r.ID)
				}
			}
		}
	}

	putStr(attributeAzureServiceHealthTitle, properties.Title, record)
	putStr(attributeAzureServiceHealthService, properties.Service, record)
	putStr(attributeAzureServiceHealthRegion, properties.Region, record)
	putStr(attributeAzureServiceHealthCommunicationBody, properties.CommunicationText, record)
	putStr(attributeAzureServiceHealthCommunicationID, properties.CommunicationID, record)
	putStr(attributeAzureServiceHealthIncidentType, properties.IncidentType, record)
	putStr(attributeAzureServiceHealthTrackingID, properties.TrackingID, record)
	putStr(attributeAzureServiceHealthImpactStartTime, properties.ImpactStartTime, record)
	putStr(attributeAzureServiceHealthImpactMitigationTime, properties.ImpactMitigationTime, record)
	putStr(attributeAzureServiceHealthDefaultLanguageTitle, properties.DefaultLanguageTitle, record)
	putStr(attributeAzureServiceHealthDefaultLanguageContent, properties.DefaultLanguageContent, record)
	putStr(attributeAzureServiceHealthState, properties.Stage, record)
	putStr(attributeAzureServiceHealthMaintenanceID, properties.MaintenanceID, record)
	putStr(attributeAzureServiceHealthMaintenanceType, properties.MaintenanceType, record)
	if properties.IsHIR {
		record.Attributes().PutBool(attributeAzureServiceHealthIsHIR, properties.IsHIR)
	}
	putBool(attributeAzureServiceHealthIsSynthetic, properties.IsSynthetic, record)
	putStr(attributeAzureServiceHealthImpactType, properties.ImpactType, record)
	putStr(attributeAzureServiceHealthImpactCategory, properties.ImpactCategory, record)
	return nil
}

// ------------------------------------------------------------
// Activity Log - Resource Health category
// ------------------------------------------------------------

// resourceHealthLogProperties represents the properties field of a Resource Health activity log.
// See: https://learn.microsoft.com/en-us/azure/azure-monitor/essentials/activity-log-schema#resource-health-category
type resourceHealthLogProperties struct {
	Title                string `json:"title"`
	Details              string `json:"details"`
	CurrentHealthStatus  string `json:"currentHealthStatus"`
	PreviousHealthStatus string `json:"previousHealthStatus"`
	Type                 string `json:"type"`
	Cause                string `json:"cause"`
}

// addResourceHealthLogProperties parses Resource Health activity logs
// and maps them to OpenTelemetry semantic conventions.
// See: https://learn.microsoft.com/en-us/azure/azure-monitor/essentials/activity-log-schema#resource-health-category
func addResourceHealthLogProperties(data []byte, record plog.LogRecord) error {
	var properties resourceHealthLogProperties
	if err := gojson.Unmarshal(data, &properties); err != nil {
		return fmt.Errorf("failed to parse Resource Health properties: %w", err)
	}

	putStr(attributeAzureResourceHealthTitle, properties.Title, record)
	putStr(attributeAzureResourceHealthDetails, properties.Details, record)
	putStr(attributeAzureResourceHealthCurrentHealthStatus, properties.CurrentHealthStatus, record)
	putStr(attributeAzureResourceHealthPreviousHealthStatus, properties.PreviousHealthStatus, record)
	putStr(attributeAzureResourceHealthType, properties.Type, record)
	putStr(attributeAzureResourceHealthCause, properties.Cause, record)

	return nil
}
