// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package auditlog // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/googlecloudlogentryencodingextension/internal/auditlog"

import (
	"errors"
	"fmt"

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
	CallerIP                string `json:"callerIp"`
	CallerSuppliedUserAgent string `json:"callerSuppliedUserAgent"`
	CallerNetwork           string `json:"callerNetwork"`
	// TODO Add requestAttributes
	// TODO Add destinationAttributes
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
	for _, auth := range info {
		m := infoList.AppendEmpty().SetEmptyMap()
		shared.PutStr(gcpAuditAuthorizationPermission, auth.Permission, m)
		shared.PutBool(gcpAuditAuthorizationGranted, auth.Granted, m)
		shared.PutStr(gcpAuditAuthorizationResource, auth.Resource, m)
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

func handleRequestMetadata(metadata *requestMetadata, attr pcommon.Map) {
	if metadata == nil {
		return
	}

	shared.PutStr(string(semconv.ClientAddressKey), metadata.CallerIP, attr)
	shared.PutStr(string(semconv.UserAgentOriginalKey), metadata.CallerSuppliedUserAgent, attr)
	shared.PutStr(gcpAuditRequestCallerNetwork, metadata.CallerNetwork, attr)
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

	shared.PutStr(gcpAuditResourceName, log.ResourceName, attr)
	handleResourceLocation(log.ResourceLocation, attr)

	handleStatus(log.Status, attr)
	handleAuthenticationInfo(log.AuthenticationInfo, attr)
	handleAuthorizationInfo(log.AuthorizationInfo, attr)
	handlePolicyViolationInfo(log.PolicyViolationInfo, attr)
	handleRequestMetadata(log.RequestMetadata, attr)
	handlePolicyViolationInfo(log.PolicyViolationInfo, attr)

	return nil
}
