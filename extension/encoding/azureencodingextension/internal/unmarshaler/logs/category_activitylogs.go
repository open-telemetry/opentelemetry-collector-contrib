// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logs // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/azureencodingextension/internal/unmarshaler/logs"

import (
	"strconv"
	"strings"

	gojson "github.com/goccy/go-json"
	"go.opentelemetry.io/collector/pdata/pcommon"
	conventions "go.opentelemetry.io/otel/semconv/v1.38.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/azureencodingextension/internal/unmarshaler"
)

// ------------------------------------------------------------
// Activity Log Identity
// ------------------------------------------------------------

type azureIdentityActivity struct {
	azureIdentityBase

	Authorization *activityLogIdentityAuthorization `json:"authorization"`
}

func (r *azureIdentityActivity) PutIdentityAttributes(attrs pcommon.Map) {
	r.azureIdentityBase.PutIdentityAttributes(attrs)

	// Authorization
	if r.Authorization != nil {
		unmarshaler.AttrPutStrIf(attrs, attributeIdentityAuthorizationScope, r.Authorization.Scope)
		unmarshaler.AttrPutStrIf(attrs, attributeIdentityAuthorizationAction, r.Authorization.Action)

		if r.Authorization.Evidence != nil {
			unmarshaler.AttrPutStrIf(attrs, attributeIdentityAuthorizationEvidenceRole, r.Authorization.Evidence.Role)
			unmarshaler.AttrPutStrIf(attrs, attributeIdentityAuthorizationEvidenceRoleAssignmentScope, r.Authorization.Evidence.RoleAssignmentScope)
			unmarshaler.AttrPutStrIf(attrs, attributeIdentityAuthorizationEvidenceRoleAssignmentID, r.Authorization.Evidence.RoleAssignmentID)
			unmarshaler.AttrPutStrIf(attrs, attributeIdentityAuthorizationEvidenceRoleDefinitionID, r.Authorization.Evidence.RoleDefinitionID)
			unmarshaler.AttrPutStrIf(attrs, attributeIdentityAuthorizationEvidencePrincipalID, r.Authorization.Evidence.PrincipalID)
			unmarshaler.AttrPutStrIf(attrs, attributeIdentityAuthorizationEvidencePrincipalType, r.Authorization.Evidence.PrincipalType)
		}
	}
}

// activityLogIdentityEvidence describes role assignment evidence in identity authorization
type activityLogIdentityEvidence struct {
	Role                string `json:"role"`
	RoleAssignmentScope string `json:"roleAssignmentScope"`
	RoleAssignmentID    string `json:"roleAssignmentId"`
	RoleDefinitionID    string `json:"roleDefinitionId"`
	PrincipalID         string `json:"principalId"`
	PrincipalType       string `json:"principalType"`
}

// activityLogIdentityAuthorization describes identity authorization details
type activityLogIdentityAuthorization struct {
	Scope    string                       `json:"scope"`
	Action   string                       `json:"action"`
	Evidence *activityLogIdentityEvidence `json:"evidence"`
}

// activityLogRecordBase extends azureLogRecordBase with Activity Log identity parsing.
// All activity log category structs embed this instead of azureLogRecordBase directly,
// so they inherit the correct identity handling.
//
// The Identity field is parsed directly into activityLogIdentity during the
// main JSON unmarshal step - no double parsing required.
type activityLogRecordBase struct {
	azureLogRecordBase

	Identity *azureIdentityActivity `json:"identity"`
}

// PutCommonAttributes extends the base method by also extracting identity fields
// specific to Activity Logs (authorization, JWT claims, etc.)
func (r *activityLogRecordBase) PutCommonAttributes(attrs pcommon.Map, body pcommon.Value) {
	r.azureLogRecordBase.PutCommonAttributes(attrs, body)

	if r.Identity != nil {
		r.Identity.PutIdentityAttributes(attrs)
	}
}

// ------------------------------------------------------------
// Activity Log - Administrative category
// ------------------------------------------------------------

// Non-SemConv attributes for Administrative activity logs
const (
	attributeAzureAdministrativeEntity    = "azure.administrative.entity"
	attributeAzureAdministrativeMessage   = "azure.administrative.message"
	attributeAzureAdministrativeHierarchy = "azure.administrative.hierarchy"
)

// azureAdministrativeLog represents an Administrative activity log
// See: https://learn.microsoft.com/en-us/azure/azure-monitor/essentials/activity-log-schema#administrative-category
type azureAdministrativeLog struct {
	activityLogRecordBase

	Properties struct {
		Entity    string `json:"entity"`
		Message   string `json:"message"`
		Hierarchy string `json:"hierarchy"`
	} `json:"properties"`
}

func (r *azureAdministrativeLog) PutProperties(attrs pcommon.Map, _ pcommon.Value) error {
	unmarshaler.AttrPutStrIf(attrs, attributeAzureAdministrativeEntity, r.Properties.Entity)
	unmarshaler.AttrPutStrIf(attrs, attributeAzureAdministrativeMessage, r.Properties.Message)
	unmarshaler.AttrPutStrIf(attrs, attributeAzureAdministrativeHierarchy, r.Properties.Hierarchy)

	return nil
}

// ------------------------------------------------------------
// Activity Log - Alert category
// ------------------------------------------------------------

// Non-SemConv attributes for Alert activity logs
const (
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
)

// azureAlertLog represents an Alert activity log
// See: https://learn.microsoft.com/en-us/azure/azure-monitor/essentials/activity-log-schema#alert-category
type azureAlertLog struct {
	activityLogRecordBase

	Properties struct {
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
	} `json:"properties"`
}

func (r *azureAlertLog) PutProperties(attrs pcommon.Map, _ pcommon.Value) error {
	unmarshaler.AttrPutStrIf(attrs, attributeAzureAlertWebhookURI, r.Properties.WebHookURI)
	unmarshaler.AttrPutStrIf(attrs, attributeAzureAlertRuleURI, r.Properties.RuleURI)
	unmarshaler.AttrPutStrIf(attrs, attributeAzureAlertRuleName, r.Properties.RuleName)
	unmarshaler.AttrPutStrIf(attrs, attributeAzureAlertRuleDescription, r.Properties.RuleDescription)
	unmarshaler.AttrPutStrIf(attrs, attributeAzureAlertThreshold, r.Properties.Threshold)
	unmarshaler.AttrPutStrIf(attrs, attributeAzureAlertWindowSize, r.Properties.WindowSizeInMinutes)
	unmarshaler.AttrPutStrIf(attrs, attributeAzureAlertAggregation, r.Properties.Aggregation)
	unmarshaler.AttrPutStrIf(attrs, attributeAzureAlertOperator, r.Properties.Operator)
	unmarshaler.AttrPutStrIf(attrs, attributeAzureAlertMetricName, r.Properties.MetricName)
	unmarshaler.AttrPutStrIf(attrs, attributeAzureAlertMetricUnit, r.Properties.MetricUnit)

	return nil
}

// ------------------------------------------------------------
// Activity Log - Autoscale category
// ------------------------------------------------------------

// Non-SemConv attributes for Autoscale activity logs
const (
	attributeAzureAutoscaleDescription     = "azure.autoscale.description"
	attributeAzureAutoscaleResourceName    = "azure.autoscale.resource.name"
	attributeAzureAutoscaleOldInstances    = "azure.autoscale.instances.previous_count"
	attributeAzureAutoscaleNewInstances    = "azure.autoscale.instances.count"
	attributeAzureAutoscaleLastScaleAction = "azure.autoscale.resource.last_scale"
)

// azureAutoscaleLog represents an Autoscale activity log
// See: https://learn.microsoft.com/en-us/azure/azure-monitor/essentials/activity-log-schema#autoscale-category
type azureAutoscaleLog struct {
	activityLogRecordBase

	Properties struct {
		Description         string `json:"Description"`
		ResourceName        string `json:"ResourceName"`
		OldInstancesCount   string `json:"OldInstancesCount"`
		NewInstancesCount   string `json:"NewInstancesCount"`
		LastScaleActionTime string `json:"LastScaleActionTime"`
	} `json:"properties"`
}

func (r *azureAutoscaleLog) PutProperties(attrs pcommon.Map, _ pcommon.Value) error {
	unmarshaler.AttrPutStrIf(attrs, attributeAzureAutoscaleDescription, r.Properties.Description)
	unmarshaler.AttrPutStrIf(attrs, attributeAzureAutoscaleResourceName, r.Properties.ResourceName)
	unmarshaler.AttrPutStrIf(attrs, attributeAzureAutoscaleOldInstances, r.Properties.OldInstancesCount)
	unmarshaler.AttrPutStrIf(attrs, attributeAzureAutoscaleNewInstances, r.Properties.NewInstancesCount)
	unmarshaler.AttrPutStrIf(attrs, attributeAzureAutoscaleLastScaleAction, r.Properties.LastScaleActionTime)

	return nil
}

// ------------------------------------------------------------
// Activity Log - Security category
// ------------------------------------------------------------

// Non-SemConv attributes for Security activity logs
const (
	attributeAzureSecurityAccountLogonID = "azure.security.account_logon_id"
	attributeAzureSecurityDomainName     = "azure.security.domain_name"
	attributeAzureSecurityActionTaken    = "azure.security.action_taken"
	attributeAzureSecuritySeverity       = "azure.security.severity"
)

// azureSecurityLog represents a Security activity log
// See: https://learn.microsoft.com/en-us/azure/azure-monitor/essentials/activity-log-schema#security-category
type azureSecurityLog struct {
	activityLogRecordBase

	Properties struct {
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
	} `json:"properties"`
}

func (r *azureSecurityLog) PutProperties(attrs pcommon.Map, _ pcommon.Value) error {
	// Map to OTel process semantic conventions
	unmarshaler.AttrPutStrIf(attrs, string(conventions.ProcessCommandLineKey), r.Properties.CommandLine)
	unmarshaler.AttrPutStrIf(attrs, string(conventions.ProcessExecutablePathKey), r.Properties.ProcessName)
	unmarshaler.AttrPutStrIf(attrs, string(conventions.ProcessOwnerKey), r.Properties.UserName)
	unmarshaler.AttrPutStrIf(attrs, string(conventions.EnduserIDKey), r.Properties.UserSID)

	// Parse and set process.pid if present
	if r.Properties.ProcessID != "" {
		if pid, err := strconv.ParseInt(r.Properties.ProcessID, 10, 64); err == nil {
			attrs.PutInt(string(conventions.ProcessPIDKey), pid)
		}
	}

	// Parse and set process.parent_pid if present
	if r.Properties.ParentProcessID != "" {
		if ppid, err := strconv.ParseInt(r.Properties.ParentProcessID, 10, 64); err == nil {
			attrs.PutInt(string(conventions.ProcessParentPIDKey), ppid)
		}
	}

	// Azure-specific fields that don't have OTel equivalents
	unmarshaler.AttrPutStrIf(attrs, attributeAzureSecurityAccountLogonID, r.Properties.AccountLogonID)
	unmarshaler.AttrPutStrIf(attrs, attributeAzureSecurityDomainName, r.Properties.DomainName)
	unmarshaler.AttrPutStrIf(attrs, attributeAzureSecurityActionTaken, r.Properties.ActionTaken)
	unmarshaler.AttrPutStrIf(attrs, attributeAzureSecuritySeverity, r.Properties.Severity)

	return nil
}

// ------------------------------------------------------------
// Activity Log - Policy category
// ------------------------------------------------------------

// Non-SemConv attributes for Policy activity logs
const (
	attributeAzurePolicyIsComplianceCheck = "azure.policy.compliance_check"
	attributeAzureLocation                = "azure.location"
	attributeAzurePolicyAncestors         = "azure.policy.ancestors"
	attributeAzurePolicyHierarchy         = "azure.policy.hierarchy"
	attributeAzurePolicyPolicies          = "azure.policy.policies"
)

// policyElement represents a single policy in the policies array
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

// azurePolicyLog represents a Policy activity log
// See: https://learn.microsoft.com/en-us/azure/azure-monitor/essentials/activity-log-schema#policy-category
type azurePolicyLog struct {
	activityLogRecordBase

	Properties struct {
		IsComplianceCheck string `json:"isComplianceCheck"`
		ResourceLocation  string `json:"resourceLocation"`
		Ancestors         string `json:"ancestors"`
		Policies          string `json:"policies"`
		Hierarchy         string `json:"hierarchy"`
	} `json:"properties"`
}

func (r *azurePolicyLog) PutProperties(attrs pcommon.Map, _ pcommon.Value) error {
	// Parse isComplianceCheck as boolean
	if r.Properties.IsComplianceCheck != "" {
		isComplianceCheck := strings.EqualFold(r.Properties.IsComplianceCheck, "true")
		attrs.PutBool(attributeAzurePolicyIsComplianceCheck, isComplianceCheck)
	}

	unmarshaler.AttrPutStrIf(attrs, attributeAzureLocation, r.Properties.ResourceLocation)
	unmarshaler.AttrPutStrIf(attrs, attributeAzurePolicyAncestors, r.Properties.Ancestors)
	unmarshaler.AttrPutStrIf(attrs, attributeAzurePolicyHierarchy, r.Properties.Hierarchy)

	// Parse the embedded policies JSON string
	if r.Properties.Policies != "" {
		var policies []policyElement
		if err := gojson.Unmarshal([]byte(r.Properties.Policies), &policies); err == nil && len(policies) > 0 {
			policiesSlice := attrs.PutEmptySlice(attributeAzurePolicyPolicies)
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
	}

	return nil
}

// ------------------------------------------------------------
// Activity Log - Service Health category
// ------------------------------------------------------------

// Non-SemConv attributes for Service Health activity logs
const (
	attributeAzureServiceHealthTitle                  = "azure.servicehealth.title"
	attributeAzureServiceHealthService                = "azure.servicehealth.service"
	attributeAzureServiceHealthRegion                 = "azure.servicehealth.region"
	attributeAzureServiceHealthCommunicationID        = "azure.servicehealth.communication.id"
	attributeAzureServiceHealthCommunicationBody      = "azure.servicehealth.communication.body"
	attributeAzureServiceHealthIncidentType           = "azure.servicehealth.incident.type"
	attributeAzureServiceHealthTrackingID             = "azure.servicehealth.tracking.id"
	attributeAzureServiceHealthImpactStartTime        = "azure.servicehealth.impact.start"
	attributeAzureServiceHealthImpactMitigationTime   = "azure.servicehealth.impact.mitigation"
	attributeAzureServiceHealthImpactedServices       = "azure.servicehealth.impact.services"
	attributeAzureServiceHealthImpactType             = "azure.servicehealth.impact.type"
	attributeAzureServiceHealthImpactCategory         = "azure.servicehealth.impact.category"
	attributeAzureServiceHealthDefaultLanguageTitle   = "azure.servicehealth.default_language.title"
	attributeAzureServiceHealthDefaultLanguageContent = "azure.servicehealth.default_language.content"
	attributeAzureServiceHealthState                  = "azure.servicehealth.state"
	attributeAzureServiceHealthMaintenanceID          = "azure.servicehealth.maintenance.id"
	attributeAzureServiceHealthMaintenanceType        = "azure.servicehealth.maintenance.type"
	attributeAzureServiceHealthIsHIR                  = "azure.servicehealth.is_hir"
	attributeAzureServiceHealthIsSynthetic            = "azure.servicehealth.is_synthetic"
)

// impactedService represents an impacted service in Service Health logs
type impactedService struct {
	Name    string `json:"ServiceName"`
	ID      string `json:"ServiceId"`
	GUID    string `json:"ServiceGuid"`
	Regions []struct {
		Name string `json:"RegionName"`
		ID   string `json:"RegionId"`
	} `json:"ImpactedRegions"`
}

// azureServiceHealthLog represents a Service Health activity log
// See: https://learn.microsoft.com/en-us/azure/azure-monitor/essentials/activity-log-schema#service-health-category
type azureServiceHealthLog struct {
	activityLogRecordBase

	Properties struct {
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
	} `json:"properties"`
}

func (r *azureServiceHealthLog) PutProperties(attrs pcommon.Map, _ pcommon.Value) error {
	unmarshaler.AttrPutStrIf(attrs, attributeAzureServiceHealthTitle, r.Properties.Title)
	unmarshaler.AttrPutStrIf(attrs, attributeAzureServiceHealthService, r.Properties.Service)
	unmarshaler.AttrPutStrIf(attrs, attributeAzureServiceHealthRegion, r.Properties.Region)
	unmarshaler.AttrPutStrIf(attrs, attributeAzureServiceHealthCommunicationBody, r.Properties.CommunicationText)
	unmarshaler.AttrPutStrIf(attrs, attributeAzureServiceHealthIncidentType, r.Properties.IncidentType)
	unmarshaler.AttrPutStrIf(attrs, attributeAzureServiceHealthTrackingID, r.Properties.TrackingID)
	unmarshaler.AttrPutStrIf(attrs, attributeAzureServiceHealthImpactStartTime, r.Properties.ImpactStartTime)
	unmarshaler.AttrPutStrIf(attrs, attributeAzureServiceHealthImpactMitigationTime, r.Properties.ImpactMitigationTime)
	unmarshaler.AttrPutStrIf(attrs, attributeAzureServiceHealthDefaultLanguageTitle, r.Properties.DefaultLanguageTitle)
	unmarshaler.AttrPutStrIf(attrs, attributeAzureServiceHealthDefaultLanguageContent, r.Properties.DefaultLanguageContent)
	unmarshaler.AttrPutStrIf(attrs, attributeAzureServiceHealthState, r.Properties.Stage)
	unmarshaler.AttrPutStrIf(attrs, attributeAzureServiceHealthCommunicationID, r.Properties.CommunicationID)
	unmarshaler.AttrPutStrIf(attrs, attributeAzureServiceHealthMaintenanceID, r.Properties.MaintenanceID)
	unmarshaler.AttrPutStrIf(attrs, attributeAzureServiceHealthMaintenanceType, r.Properties.MaintenanceType)
	unmarshaler.AttrPutStrIf(attrs, attributeAzureServiceHealthImpactType, r.Properties.ImpactType)
	unmarshaler.AttrPutStrIf(attrs, attributeAzureServiceHealthImpactCategory, r.Properties.ImpactCategory)

	// Handle isHIR - only set if true
	if r.Properties.IsHIR {
		attrs.PutBool(attributeAzureServiceHealthIsHIR, r.Properties.IsHIR)
	}

	// Parse isSynthetic as boolean
	if r.Properties.IsSynthetic != "" {
		isSynthetic := r.Properties.IsSynthetic == "True" || r.Properties.IsSynthetic == "true"
		attrs.PutBool(attributeAzureServiceHealthIsSynthetic, isSynthetic)
	}

	// Parse the embedded impactedServices JSON string
	if r.Properties.ImpactedServices != "" {
		var impactedServices []impactedService
		if err := gojson.Unmarshal([]byte(r.Properties.ImpactedServices), &impactedServices); err == nil && len(impactedServices) > 0 {
			impactedServicesSlice := attrs.PutEmptySlice(attributeAzureServiceHealthImpactedServices)
			for _, s := range impactedServices {
				impactedServiceMap := impactedServicesSlice.AppendEmpty().SetEmptyMap()
				impactedServiceMap.PutStr("name", s.Name)
				impactedServiceMap.PutStr("id", s.ID)
				impactedServiceMap.PutStr("guid", s.GUID)

				if len(s.Regions) > 0 {
					regionsSlice := impactedServiceMap.PutEmptySlice("regions")
					for _, region := range s.Regions {
						regionMap := regionsSlice.AppendEmpty().SetEmptyMap()
						regionMap.PutStr("name", region.Name)
						regionMap.PutStr("id", region.ID)
					}
				}
			}
		}
	}

	return nil
}

// ------------------------------------------------------------
// Activity Log - Resource Health category
// ------------------------------------------------------------

// Non-SemConv attributes for Resource Health activity logs
const (
	attributeAzureResourceHealthTitle                = "azure.resourcehealth.title"
	attributeAzureResourceHealthDetails              = "azure.resourcehealth.details"
	attributeAzureResourceHealthCurrentHealthStatus  = "azure.resourcehealth.state"
	attributeAzureResourceHealthPreviousHealthStatus = "azure.resourcehealth.previous_state"
	attributeAzureResourceHealthType                 = "azure.resourcehealth.type"
	attributeAzureResourceHealthCause                = "azure.resourcehealth.cause"
)

// azureResourceHealthLog represents a Resource Health activity log
// See: https://learn.microsoft.com/en-us/azure/azure-monitor/essentials/activity-log-schema#resource-health-category
type azureResourceHealthLog struct {
	activityLogRecordBase

	Properties struct {
		Title                string `json:"title"`
		Details              string `json:"details"`
		CurrentHealthStatus  string `json:"currentHealthStatus"`
		PreviousHealthStatus string `json:"previousHealthStatus"`
		Type                 string `json:"type"`
		Cause                string `json:"cause"`
	} `json:"properties"`
}

func (r *azureResourceHealthLog) PutProperties(attrs pcommon.Map, _ pcommon.Value) error {
	unmarshaler.AttrPutStrIf(attrs, attributeAzureResourceHealthTitle, r.Properties.Title)
	unmarshaler.AttrPutStrIf(attrs, attributeAzureResourceHealthDetails, r.Properties.Details)
	unmarshaler.AttrPutStrIf(attrs, attributeAzureResourceHealthCurrentHealthStatus, r.Properties.CurrentHealthStatus)
	unmarshaler.AttrPutStrIf(attrs, attributeAzureResourceHealthPreviousHealthStatus, r.Properties.PreviousHealthStatus)
	unmarshaler.AttrPutStrIf(attrs, attributeAzureResourceHealthType, r.Properties.Type)
	unmarshaler.AttrPutStrIf(attrs, attributeAzureResourceHealthCause, r.Properties.Cause)

	return nil
}
