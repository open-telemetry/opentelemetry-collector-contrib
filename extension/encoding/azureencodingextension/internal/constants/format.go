// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package constants // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/azureencodingextension/internal/constants"

const (
	// FormatIdentificationTag is the attribute key used to identify the encoding format of the log.
	FormatIdentificationTag = "encoding.format"

	// Format values at log family level
	FormatActivity           = "azure.activity"
	FormatApplicationGateway = "azure.application_gateway"
	FormatAudit              = "azure.audit"
	FormatAppService         = "azure.appservice"
	FormatCdn                = "azure.cdn"
	FormatDataFactory        = "azure.datafactory"
	FormatFrontDoor          = "azure.frontdoor"
	FormatFunctionApp        = "azure.function_app"
	FormatMessaging          = "azure.messaging"
	FormatStorage            = "azure.storage"

	// AzureFormatResourceLog is the value for unknown/raw Azure resource logs.
	AzureFormatResourceLog = "azure.resource"
)

// FormatForCategory returns the encoding.format value (log family) for the given Azure log category.
// Unknown categories return AzureFormatResourceLog.
func FormatForCategory(category string) string {
	switch category {
	case "ApplicationGatewayAccessLog", "ApplicationGatewayPerformanceLog", "ApplicationGatewayFirewallLog":
		return FormatApplicationGateway
	case "AppServiceAppLogs", "AppServiceAuditLogs", "AppServiceAuthenticationLogs", "AppServiceConsoleLogs",
		"AppServiceFileAuditLogs", "AppServiceHTTPLogs", "AppServiceIPSecAuditLogs", "AppServicePlatformLogs":
		return FormatAppService
	case "AzureCdnAccessLog":
		return FormatCdn
	case "ApplicationMetricsLogs", "DiagnosticErrorLogs", "OperationalLogs", "RuntimeAuditLogs", "VNetAndIPFilteringLogs":
		return FormatMessaging
	case "ActivityRuns", "PipelineRuns", "TriggerRuns":
		return FormatDataFactory
	case "FrontDoorAccessLog", "FrontDoorHealthProbeLog", "FrontDoorWebApplicationFirewallLog":
		return FormatFrontDoor
	case "FunctionAppLogs":
		return FormatFunctionApp
	case "StorageRead", "StorageWrite", "StorageDelete":
		return FormatStorage
	case "AuditEvent":
		return FormatAudit
	case "Administrative", "Alert", "Autoscale", "Policy", "Recommendation", "ResourceHealth", "Security", "ServiceHealth":
		return FormatActivity
	default:
		return AzureFormatResourceLog
	}
}
