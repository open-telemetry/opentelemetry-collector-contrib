// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package constants // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/azureencodingextension/internal/constants"

const (
	// FormatIdentificationTag is the attribute key used to identify
	// the encoding format of the log (e.g. for filtering and routing).
	FormatIdentificationTag = "encoding.format"

	// Detailed format values per Azure log category.
	FormatApplicationGatewayAccess   = "azure.application_gateway_access"
	FormatApplicationGatewayFirewall = "azure.application_gateway_firewall"
	FormatApplicationGatewayPerf     = "azure.application_gateway_performance"
	FormatAppServiceApp              = "azure.appservice_app"
	FormatAppServiceAudit            = "azure.appservice_audit"
	FormatAppServiceAuth             = "azure.appservice_authentication"
	FormatAppServiceConsole          = "azure.appservice_console"
	FormatAppServiceFileAudit        = "azure.appservice_file_audit"
	FormatAppServiceHTTP             = "azure.appservice_http"
	FormatAppServiceIPSecAudit       = "azure.appservice_ipsec_audit"
	FormatAppServicePlatform         = "azure.appservice_platform"
	FormatAuditEvent                 = "azure.audit_event"
	FormatCdnAccess                  = "azure.cdn_access"
	FormatDataFactoryActivityRuns    = "azure.datafactory_activity_runs"
	FormatDataFactoryPipelineRuns    = "azure.datafactory_pipeline_runs"
	FormatDataFactoryTriggerRuns     = "azure.datafactory_trigger_runs"
	FormatFrontDoorAccess            = "azure.frontdoor_access"
	FormatFrontDoorHealthProbe       = "azure.frontdoor_health_probe"
	FormatFrontDoorWAF               = "azure.frontdoor_waf"
	FormatFunctionApp                = "azure.function_app"
	FormatMessagingAppMetrics        = "azure.messaging_application_metrics"
	FormatMessagingDiagnosticError   = "azure.messaging_diagnostic_error"
	FormatMessagingOperational       = "azure.messaging_operational"
	FormatMessagingRuntimeAudit      = "azure.messaging_runtime_audit"
	FormatMessagingVNetFilter        = "azure.messaging_vnet_filter"
	FormatRecommendation             = "azure.recommendation"
	FormatStorageDelete              = "azure.storage_delete"
	FormatStorageRead                = "azure.storage_read"
	FormatStorageWrite               = "azure.storage_write"

	// AzureFormatResourceLog is the value for unknown/raw Azure resource logs.
	AzureFormatResourceLog = "azure.resource"
)

// FormatForCategory returns the encoding.format value for the given Azure log category.
// Unknown categories return AzureFormatResourceLog.
func FormatForCategory(category string) string {
	switch category {
	case "ApplicationGatewayAccessLog":
		return FormatApplicationGatewayAccess
	case "ApplicationGatewayPerformanceLog":
		return FormatApplicationGatewayPerf
	case "ApplicationGatewayFirewallLog":
		return FormatApplicationGatewayFirewall
	case "AppServiceAppLogs":
		return FormatAppServiceApp
	case "AppServiceAuditLogs":
		return FormatAppServiceAudit
	case "AppServiceAuthenticationLogs":
		return FormatAppServiceAuth
	case "AppServiceConsoleLogs":
		return FormatAppServiceConsole
	case "AppServiceFileAuditLogs":
		return FormatAppServiceFileAudit
	case "AppServiceHTTPLogs":
		return FormatAppServiceHTTP
	case "AppServiceIPSecAuditLogs":
		return FormatAppServiceIPSecAudit
	case "AppServicePlatformLogs":
		return FormatAppServicePlatform
	case "AzureCdnAccessLog":
		return FormatCdnAccess
	case "ApplicationMetricsLogs":
		return FormatMessagingAppMetrics
	case "DiagnosticErrorLogs":
		return FormatMessagingDiagnosticError
	case "OperationalLogs":
		return FormatMessagingOperational
	case "RuntimeAuditLogs":
		return FormatMessagingRuntimeAudit
	case "VNetAndIPFilteringLogs":
		return FormatMessagingVNetFilter
	case "ActivityRuns":
		return FormatDataFactoryActivityRuns
	case "PipelineRuns":
		return FormatDataFactoryPipelineRuns
	case "TriggerRuns":
		return FormatDataFactoryTriggerRuns
	case "FrontDoorAccessLog":
		return FormatFrontDoorAccess
	case "FrontDoorHealthProbeLog":
		return FormatFrontDoorHealthProbe
	case "FrontDoorWebApplicationFirewallLog":
		return FormatFrontDoorWAF
	case "FunctionAppLogs":
		return FormatFunctionApp
	case "Recommendation":
		return FormatRecommendation
	case "StorageRead":
		return FormatStorageRead
	case "StorageWrite":
		return FormatStorageWrite
	case "StorageDelete":
		return FormatStorageDelete
	case "AuditEvent":
		return FormatAuditEvent
	default:
		return AzureFormatResourceLog
	}
}
