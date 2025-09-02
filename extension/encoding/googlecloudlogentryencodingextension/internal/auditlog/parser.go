// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package auditlog // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/googlecloudlogentryencodingextension/internal/auditlog"

import (
	"errors"
	"fmt"
	"strings"

	gojson "github.com/goccy/go-json"
	"github.com/iancoleman/strcase"
	"go.opentelemetry.io/collector/pdata/pcommon"
	semconv "go.opentelemetry.io/otel/semconv/v1.34.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/googlecloudlogentryencodingextension/internal/shared"
)

const (
	ActivityLogNameSuffix    = "cloudaudit.googleapis.com%2Factivity"
	DataAccessLogNameSuffix  = "cloudaudit.googleapis.com%2Fdata_access"
	SystemEventLogNameSuffix = "cloudaudit.googleapis.com%2Fsystem_event"
	PolicyLogNameSuffix      = "cloudaudit.googleapis.com%2Fpolicy"

	auditLogType = "type.googleapis.com/google.cloud.audit.AuditLog"

	// gcpAuditServiceName holds the name of the API service performing the operation
	gcpAuditServiceName = "gcp.audit.service.name"
	// gcpAuditMethodName holds the name of the service method or operation
	gcpAuditMethodName = "gcp.audit.method.name"
	// gcpAuditResourceName holds the value for the operation target
	gcpAuditResourceName = "gcp.audit.resource.name"

	// gcpAuditResourceLocationCurrent holds the locations of a resource after the execution of the operation
	gcpAuditResourceLocationCurrent = "gcp.audit.resource.location.current"
	// gcpAuditResourceLocationOriginal holds the locations of a resource prior to the execution of the operation
	gcpAuditResourceLocationOriginal = "gcp.audit.resource.location.original"

	// gcpAuditNUmResponseItems holds the number of items returned from a List or Query API method, if applicable
	gcpAuditNumResponseItems = "gcp.audit.response.items"

	// gcpAuditAuthenticationAuthoritySelector holds the authority selector specified by the requestor, if any
	gcpAuditAuthenticationAuthoritySelector = "gcp.audit.authentication.authority_selector"
	// gcpAuditAuthenticationServiceAccountKeyName holds the name of the service account key used to create/exchange
	// credentials for authenticating the service account making the request
	gcpAuditAuthenticationServiceAccountKeyName = "gcp.audit.authentication.service_account.key.name"

	gcpAuditAuthorization = "gcp.audit.authorization"
	// gcpAuditAuthorizationResource holds the value of the resource being accessed
	gcpAuditAuthorizationResource = "resource"
	// gcpAuditAuthorizationPermission holds the required IAM permission
	gcpAuditAuthorizationPermission = "permission"
	// gcpAuditAuthorizationGranted holds the value on whether the authorization for resource and permission was granted
	gcpAuditAuthorizationGranted = "granted"

	// gcpAuditRequestCallerNetwork holds the network of the request caller
	gcpAuditRequestCallerNetwork = "gcp.audit.request.caller.network"
	// gcpAuditRequestReason holds the reason for the request.
	gcpAuditRequestReason = "gcp.audit.request.reason"
	// gcpAuditRequestTime holds the timestamp for when the destination receives the last byte of the request
	gcpAuditRequestTime = "gcp.audit.request.time"
	// httpRequestID will hold the request ID from requestMetadata.requestAttributes.id
	httpRequestID = "http.request.id"
	// gcpAuditRequestAuthPrincipal holds the principal at transport level layer
	gcpAuditRequestAuthPrincipal = "gcp.audit.request.auth.principal"
	// gcpAuditRequestAuthAudiences holds the audience at transport level layer
	gcpAuditRequestAuthAudiences = "gcp.audit.request.auth.audiences"
	// gcpAuditRequestAuthPresenter holds the presenter at transport level layer
	gcpAuditRequestAuthPresenter = "gcp.audit.request.auth.presenter"
	// gcpAuditRequestAuthAccessLevels holds the list of access level resource names that allow
	// resources to be accessed, at transport level layer
	gcpAuditRequestAuthAccessLevels = "gcp.audit.request.auth.access_levels"

	// gcpAuditDestinationLabels holds the labels in the destination attributes
	gcpAuditDestinationLabels = "gcp.audit.destination.label"
	// gcpAuditDestinationPrincipal holds the identity of the destination
	gcpAuditDestinationPrincipal = "gcp.audit.destination.principal"
	// gcpAuditDestinationRegionCode holds the region code of the destination
	gcpAuditDestinationRegionCode = "gcp.audit.destination.region_code"

	// gcpAuditPolicyViolationResourceType holds the esource type that the orgpolicy is checked against
	gcpAuditPolicyViolationResourceType = "gcp.audit.policy_violation.resource.type"
	// gcpAuditPolicyViolationResourceTags holds the tags referenced on the resource at the time of evaluation
	gcpAuditPolicyViolationResourceTags = "gcp.audit.policy_violation.resource.tags"
	// gcpAuditPolicyViolationInfo holds the policy violations list
	gcpAuditPolicyViolationInfo = "gcp.audit.policy_violation.info"
	// gcpAuditPolicyViolationInfoErrorMessage holds the error message
	gcpAuditPolicyViolationInfoErrorMessage = "error_message"
	// gcpAuditPolicyViolationInfoConstraint holds the constraint
	gcpAuditPolicyViolationInfoConstraint = "constraint"
	// gcpAuditPolicyViolationInfoCheckedValue holds the value that is being checked for the policy
	gcpAuditPolicyViolationInfoCheckedValue = "checked_value"
	// gcpAuditPolicyViolationInfoPolicyType holds the policy type
	gcpAuditPolicyViolationInfoPolicyType = "policy_type"
)

type auditLog struct {
	Type        string `json:"@type"`
	ServiceName string `json:"serviceName"`
	MethodName  string `json:"methodName"`

	ResourceName     string            `json:"resourceName"`
	ResourceLocation *resourceLocation `json:"resourceLocation"`
	// TODO Add support for resourceOriginalState
	// We can deduct the format of this struct by the @type field. We should use it
	// to unmarshal this struct without affecting performance and following the right
	// semantic conventions.

	NumResponseItems string `json:"numResponseItems"`

	Status *status `json:"status"`

	AuthenticationInfo *authenticationInfo `json:"authenticationInfo"`

	AuthorizationInfo []authorizationInfo `json:"authorizationInfo"`

	PolicyViolationInfo *policyViolationInfo `json:"policyViolationInfo"`

	RequestMetadata *requestMetadata `json:"requestMetadata"`
}

type resourceLocation struct {
	CurrentLocations  []string `json:"currentLocations"`
	OriginalLocations []string `json:"originalLocations"`
}

type status struct {
	Code    *int64 `json:"code"`
	Message string `json:"message"`
	// TODO Add support for details
}

type authenticationInfo struct {
	PrincipalEmail        string `json:"principalEmail"`
	PrincipalSubject      string `json:"principalSubject"`
	AuthoritySelector     string `json:"authoritySelector"`
	ServiceAccountKeyName string `json:"serviceAccountKeyName"`

	// TODO Add support for thirdPartyPrincipal
	// TODO Add support for serviceAccountDelegationInfo
}

type authorizationInfo struct {
	Resource   string `json:"resource"`
	Permission string `json:"permission"`
	Granted    *bool  `json:"granted"`

	// TODO Add support for resourceAttributes
}

type policyViolationInfo struct {
	OrgPolicyViolationInfo *orgPolicyViolationInfo `json:"orgPolicyViolationInfo"`
}

type orgPolicyViolationInfo struct {
	ResourceType  string            `json:"resourceType"`
	ResourceTags  map[string]string `json:"resourceTags"`
	ViolationInfo []violationInfo   `json:"violationInfo"`

	// TODO Add support for payload
}

type violationInfo struct {
	Constraint   string `json:"constraint"`
	ErrorMessage string `json:"errorMessage"`
	CheckedValue string `json:"checkedValue"`
	PolicyType   string `json:"policyType"`
}

type requestMetadata struct {
	CallerIP                string                 `json:"callerIp"`
	CallerSuppliedUserAgent string                 `json:"callerSuppliedUserAgent"`
	CallerNetwork           string                 `json:"callerNetwork"`
	RequestAttributes       *requestAttributes     `json:"requestAttributes"`
	DestinationAttributes   *destinationAttributes `json:"destinationAttributes"`
}

type requestAttributes struct {
	ID       string            `json:"id"`
	Method   string            `json:"method"`
	Headers  map[string]string `json:"headers"`
	Path     string            `json:"path"`
	Host     string            `json:"host"`
	Scheme   string            `json:"scheme"`
	Query    string            `json:"query"`
	Time     string            `json:"time"`
	Size     string            `json:"size"`
	Protocol string            `json:"protocol"`
	Reason   string            `json:"reason"`
	Auth     auth              `json:"auth"`
}

type auth struct {
	Principal    string   `json:"principal"`
	Audiences    []string `json:"audiences"`
	Presenter    string   `json:"presenter"`
	AccessLevels []string `json:"accessLevels"`
	// TODO Add support for claims
}

type destinationAttributes struct {
	IP         string            `json:"ip"`
	Port       string            `json:"port"`
	Labels     map[string]string `json:"labels"`
	Principal  string            `json:"principal"`
	RegionCode string            `json:"regionCode"`
}

// isValid checks that the log meets requirements
// See: https://cloud.google.com/logging/docs/audit/understanding-audit-logs#interpreting_the_sample_audit_log_entry
func isValid(log auditLog) error {
	if log.Type != auditLogType {
		return fmt.Errorf("expected @type to be %q, got %q", auditLogType, log.Type)
	}
	if log.ServiceName == "" {
		return errors.New("missing service name")
	}
	if log.MethodName == "" {
		return errors.New("missing method name")
	}
	return nil
}

func handleResourceLocation(loc *resourceLocation, attr pcommon.Map) {
	if loc == nil {
		return
	}

	if len(loc.CurrentLocations) > 0 {
		curr := attr.PutEmptySlice(gcpAuditResourceLocationCurrent)
		for _, c := range loc.CurrentLocations {
			v := curr.AppendEmpty()
			v.SetStr(c)
		}
	}

	if len(loc.OriginalLocations) > 0 {
		original := attr.PutEmptySlice(gcpAuditResourceLocationOriginal)
		for _, c := range loc.OriginalLocations {
			v := original.AppendEmpty()
			v.SetStr(c)
		}
	}
}

func handleStatus(s *status, attr pcommon.Map) {
	if s == nil {
		return
	}

	shared.PutInt(string(semconv.RPCJSONRPCErrorCodeKey), s.Code, attr)
	shared.PutStr(string(semconv.RPCJSONRPCErrorMessageKey), s.Message, attr)
}

func handleAuthenticationInfo(info *authenticationInfo, attr pcommon.Map) {
	if info == nil {
		return
	}

	shared.PutStr(string(semconv.UserIDKey), info.PrincipalSubject, attr)
	shared.PutStr(string(semconv.UserEmailKey), info.PrincipalEmail, attr)
	shared.PutStr(gcpAuditAuthenticationAuthoritySelector, info.AuthoritySelector, attr)
	shared.PutStr(gcpAuditAuthenticationServiceAccountKeyName, info.ServiceAccountKeyName, attr)
}

func handleAuthorizationInfo(info []authorizationInfo, attr pcommon.Map) {
	if len(info) == 0 {
		return
	}

	infoList := attr.PutEmptySlice(gcpAuditAuthorization)
	for _, authI := range info {
		m := infoList.AppendEmpty().SetEmptyMap()
		shared.PutStr(gcpAuditAuthorizationPermission, authI.Permission, m)
		shared.PutBool(gcpAuditAuthorizationGranted, authI.Granted, m)
		shared.PutStr(gcpAuditAuthorizationResource, authI.Resource, m)
	}
}

func handlePolicyViolationInfo(info *policyViolationInfo, attr pcommon.Map) {
	if info == nil {
		return
	}

	// Some logs have:
	// 		policyViolationInfo: {
	// 			orgPolicyViolationInfo: {}
	// 		}
	// We should ignore those cases.
	if info.OrgPolicyViolationInfo == nil {
		return
	}

	shared.PutStr(gcpAuditPolicyViolationResourceType, info.OrgPolicyViolationInfo.ResourceType, attr)
	if len(info.OrgPolicyViolationInfo.ResourceTags) > 0 {
		tags := attr.PutEmptyMap(gcpAuditPolicyViolationResourceTags)
		for name, value := range info.OrgPolicyViolationInfo.ResourceTags {
			shared.PutStr(strcase.ToSnakeWithIgnore(name, "."), value, tags)
		}
	}

	if len(info.OrgPolicyViolationInfo.ViolationInfo) == 0 {
		return
	}
	allInfo := attr.PutEmptySlice(gcpAuditPolicyViolationInfo)
	for _, i := range info.OrgPolicyViolationInfo.ViolationInfo {
		m := allInfo.AppendEmpty().SetEmptyMap()
		m.PutStr(gcpAuditPolicyViolationInfoConstraint, i.Constraint)
		m.PutStr(gcpAuditPolicyViolationInfoErrorMessage, i.ErrorMessage)
		m.PutStr(gcpAuditPolicyViolationInfoPolicyType, i.PolicyType)
		m.PutStr(gcpAuditPolicyViolationInfoCheckedValue, i.CheckedValue)
	}
}

func handleRequestMetadata(metadata *requestMetadata, attr pcommon.Map) error {
	if metadata == nil {
		return nil
	}

	shared.PutStr(string(semconv.ClientAddressKey), metadata.CallerIP, attr)
	shared.PutStr(string(semconv.UserAgentOriginalKey), metadata.CallerSuppliedUserAgent, attr)
	shared.PutStr(gcpAuditRequestCallerNetwork, metadata.CallerNetwork, attr)

	if metadata.RequestAttributes != nil {
		if err := shared.AddStrAsInt(string(semconv.HTTPRequestSizeKey), metadata.RequestAttributes.Size, attr); err != nil {
			return fmt.Errorf("failed to add http request size %s: %w", metadata.RequestAttributes.Size, err)
		}
		shared.PutStr(string(semconv.HTTPRequestMethodKey), metadata.RequestAttributes.Method, attr)
		shared.PutStr(string(semconv.URLQueryKey), metadata.RequestAttributes.Query, attr)
		shared.PutStr(string(semconv.URLPathKey), metadata.RequestAttributes.Path, attr)
		shared.PutStr(string(semconv.URLSchemeKey), metadata.RequestAttributes.Scheme, attr)
		shared.PutStr(gcpAuditRequestTime, metadata.RequestAttributes.Time, attr)
		shared.PutStr("http.request.header.host", metadata.RequestAttributes.Host, attr)
		for h, v := range metadata.RequestAttributes.Headers {
			shared.PutStr("http.request.header."+strings.ToLower(h), v, attr)
		}
		shared.PutStr(string(semconv.NetworkProtocolNameKey), strings.ToLower(metadata.RequestAttributes.Protocol), attr)
		shared.PutStr(gcpAuditRequestReason, metadata.RequestAttributes.Reason, attr)
		shared.PutStr(httpRequestID, metadata.RequestAttributes.ID, attr)
		shared.PutStr(gcpAuditRequestAuthPrincipal, metadata.RequestAttributes.Auth.Principal, attr)
		shared.PutStr(gcpAuditRequestAuthPresenter, metadata.RequestAttributes.Auth.Presenter, attr)
		if len(metadata.RequestAttributes.Auth.AccessLevels) > 0 {
			sl := attr.PutEmptySlice(gcpAuditRequestAuthAccessLevels)
			for _, level := range metadata.RequestAttributes.Auth.AccessLevels {
				v := sl.AppendEmpty()
				v.SetStr(level)
			}
		}
		if len(metadata.RequestAttributes.Auth.Audiences) > 0 {
			sl := attr.PutEmptySlice(gcpAuditRequestAuthAudiences)
			for _, audience := range metadata.RequestAttributes.Auth.Audiences {
				v := sl.AppendEmpty()
				v.SetStr(audience)
			}
		}
	}

	if metadata.DestinationAttributes != nil {
		if err := shared.AddStrAsInt(string(semconv.ServerPortKey), metadata.DestinationAttributes.Port, attr); err != nil {
			return fmt.Errorf("failed to add destination port %s: %w", metadata.DestinationAttributes.Port, err)
		}
		shared.PutStr(string(semconv.ServerAddressKey), metadata.DestinationAttributes.IP, attr)
		shared.PutStr(gcpAuditDestinationPrincipal, metadata.DestinationAttributes.Principal, attr)
		shared.PutStr(gcpAuditDestinationRegionCode, metadata.DestinationAttributes.RegionCode, attr)
		if len(metadata.DestinationAttributes.Labels) > 0 {
			m := attr.PutEmptyMap(gcpAuditDestinationLabels)
			for l, v := range metadata.DestinationAttributes.Labels {
				shared.PutStr(strcase.ToSnakeWithIgnore(l, "."), v, m)
			}
		}
	}

	return nil
}

func ParsePayloadIntoAttributes(payload []byte, attr pcommon.Map) error {
	var log auditLog
	if err := gojson.Unmarshal(payload, &log); err != nil {
		return fmt.Errorf("failed to unmarshal audit log payload: %w", err)
	}

	if err := isValid(log); err != nil {
		return fmt.Errorf("failed to validate audit log payload: %w", err)
	}
	attr.PutStr(gcpAuditServiceName, log.ServiceName)
	attr.PutStr(gcpAuditMethodName, log.MethodName)

	if err := shared.AddStrAsInt(gcpAuditNumResponseItems, log.NumResponseItems, attr); err != nil {
		return fmt.Errorf("failed to add number of response items: %w", err)
	}

	if err := handleRequestMetadata(log.RequestMetadata, attr); err != nil {
		return fmt.Errorf("failed to add request metadata: %w", err)
	}

	shared.PutStr(gcpAuditResourceName, log.ResourceName, attr)
	handleResourceLocation(log.ResourceLocation, attr)

	handleStatus(log.Status, attr)
	handleAuthenticationInfo(log.AuthenticationInfo, attr)
	handleAuthorizationInfo(log.AuthorizationInfo, attr)
	handlePolicyViolationInfo(log.PolicyViolationInfo, attr)
	handlePolicyViolationInfo(log.PolicyViolationInfo, attr)

	return nil
}
