// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package constants // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/azureencodingextension/internal/constants"

// Format is the encoding format for Azure logs.
type Format string

const (
	// FormatIdentificationTag is the attribute key used to identify the encoding format of the log.
	FormatIdentificationTag = "encoding.format"

	// Format values at log family level
	FormatActivity           Format = "azure.activity"
	FormatApplicationGateway Format = "azure.application_gateway"
	FormatAudit              Format = "azure.audit"
	FormatAppService         Format = "azure.appservice"
	FormatCdn                Format = "azure.cdn"
	FormatDataFactory        Format = "azure.datafactory"
	FormatFrontDoor          Format = "azure.frontdoor"
	FormatFunctionApp        Format = "azure.function_app"
	FormatMessaging          Format = "azure.messaging"
	FormatStorage            Format = "azure.storage"

	// FormatGeneric is the value when a generic parser is applied: the log has an Azure category
	// but no dedicated parser yet.
	FormatGeneric Format = "azure.generic"
)

// FormatForCategory returns the encoding.format value (log family) for the given Azure log category.
// Unsupported categories return FormatGeneric (generic parser applied).
func FormatForCategory(category string) Format {
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
	case "Administrative", "Alert", "Autoscale", "Policy", "Recommendation", "ResourceHealth", "Security", "ServiceHealth":
		return FormatActivity
	default:
		return FormatGeneric
	}
}
