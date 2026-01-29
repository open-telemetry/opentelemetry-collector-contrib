// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logs // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/azureencodingextension/internal/unmarshaler/logs"

import (
	"encoding/json"
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
	conventions "go.opentelemetry.io/otel/semconv/v1.38.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/azureencodingextension/internal/unmarshaler"
)

// Non-SemConv attributes that are used for common Azure Log Record fields
const (
	// OpenTelemetry attribute name for the name of the task being performed
	attributeAzureTaskName = "azure.app_service.task.name"

	// OpenTelemetry attribute name for original log level from Azure Log Record
	// as it sent by App Service
	attributeAzureOriginalLogLevel = "log.record.severity.original"

	// OpenTelemetry attribute name for Web Instance Id the application running
	attributeAzureWebInstanceID = "azure.app_service.instance.id"

	// OpenTelemetry attribute name for the authentication event details
	attributeAzureAuthEventDetails = "azure.auth.event.details"

	// OpenTelemetry attribute name for the version of App Service Authentication running
	attributeAzureModuleRuntimeVersion = "azure.auth.module.runtime.version"

	// OpenTelemetry attribute name for the runtime name of the application
	attributeAzureSiteName = "azure.app_service.site.name"

	// OpenTelemetry attribute name for HTTP sub-status code of the request,
	// in Azure App Service logs it differs from standard HTTP sub-status code semantic convention
	attributeAzureSubStatusCode = "azure.http.response.sub_status_code"

	// OpenTelemetry attribute name for the duration of the indicator of the access via
	// Virtual Network Service Endpoint communication
	attributeAzureIsServiceEndpoint = "azure.app_service.endpoint"

	// OpenTelemetry attribute name for the Deployment ID of the application deployment
	attributeAzureDeploymentID = "azure.deployment.id"

	// OpenTelemetry attribute name for Logger name
	attributeLogLogger = "log.record.logger"

	// OpenTelemetry attribute name for for "x-azure-fdid" HTTP Header value
	attributeHTTPHeaderAzureFDID = "http.request.header.x-azure-fdid"

	// OpenTelemetry attribute name for for "x-fd-healthprobe" HTTP Header value
	attributeHTTPHeaderFDHealthProbe = "http.request.header.x-fd-healthprobe"

	// OpenTelemetry attribute name for for "x-forwarded-for" HTTP Header value
	attributeHTTPHeaderForwardedFor = "http.request.header.x-forwarded-for"

	// OpenTelemetry attribute name for for "x-forwarded-host" HTTP Header value
	attributeHTTPHeaderForwardedHost = "http.request.header.x-forwarded-host"
)

// See https://learn.microsoft.com/en-us/azure/azure-monitor/reference/tables/appserviceapplogs
// There is no documentation about the structure of the logs, so we will
// implement this based on the fields from Azure Monitor table excluding generic fields
type azureAppServiceAppLog struct {
	azureLogRecordBase

	Properties struct {
		ContainerID       string `json:"containerId"`
		CustomLevel       string `json:"customLevel"`
		ExceptionClass    string `json:"exceptionClass"`
		Host              string `json:"host"`
		Logger            string `json:"logger"`
		Message           string `json:"message"`
		Method            string `json:"method"`
		Source            string `json:"source"`
		StackTrace        string `json:"stackTrace"`
		WebSiteInstanceID string `json:"webSiteInstanceId"`
	} `json:"properties"`
}

func (r *azureAppServiceAppLog) PutProperties(attrs pcommon.Map, body pcommon.Value) error {
	unmarshaler.AttrPutStrIf(attrs, string(conventions.ContainerIDKey), r.Properties.ContainerID)
	unmarshaler.AttrPutStrIf(attrs, attributeAzureOriginalLogLevel, r.Properties.CustomLevel)
	unmarshaler.AttrPutStrIf(attrs, string(conventions.ExceptionTypeKey), r.Properties.ExceptionClass)
	unmarshaler.AttrPutStrIf(attrs, string(conventions.HostNameKey), r.Properties.Host)
	unmarshaler.AttrPutStrIf(attrs, attributeLogLogger, r.Properties.Logger)
	unmarshaler.AttrPutStrIf(attrs, string(conventions.CodeFunctionNameKey), r.Properties.Method)
	unmarshaler.AttrPutStrIf(attrs, string(conventions.LogFilePathKey), r.Properties.Source)
	unmarshaler.AttrPutStrIf(attrs, string(conventions.ExceptionStacktraceKey), r.Properties.StackTrace)
	unmarshaler.AttrPutStrIf(attrs, attributeAzureWebInstanceID, r.Properties.WebSiteInstanceID)

	body.SetStr(r.Properties.Message)

	return nil
}

// See https://github.com/MicrosoftDocs/azure-docs/blob/main/articles/app-service/configure-basic-auth-disable.md#monitor-for-basic-authentication-attempts
type azureAppServiceAuditLog struct {
	azureLogRecordBase

	Properties struct {
		User            string `json:"user"`
		UserDisplayName string `json:"userDisplayName"`
		UserAddress     string `json:"userAddress"`
		Protocol        string `json:"protocol"`
	} `json:"properties"`
}

func (r *azureAppServiceAuditLog) PutProperties(attrs pcommon.Map, _ pcommon.Value) error {
	unmarshaler.AttrPutStrIf(attrs, string(conventions.UserIDKey), r.Properties.User)
	unmarshaler.AttrPutStrIf(attrs, string(conventions.UserNameKey), r.Properties.UserDisplayName)
	unmarshaler.AttrPutStrIf(attrs, string(conventions.SourceAddressKey), r.Properties.UserAddress)
	unmarshaler.AttrPutStrIf(attrs, string(conventions.NetworkProtocolNameKey), strings.ToLower(r.Properties.Protocol))

	return nil
}

// See https://learn.microsoft.com/en-us/azure/azure-monitor/reference/tables/appserviceauthenticationlogs
// There is no documentation about the structure of the logs, so we will
// implement this based on the fields from Azure Monitor table excluding generic fields
type azureAppServiceAuthenticationLog struct {
	azureLogRecordBase

	Properties struct {
		Details              string      `json:"details"`
		Host                 string      `json:"hostName"`
		Message              string      `json:"message"`
		ModuleRuntimeVersion string      `json:"moduleRuntimeVersion"`
		SiteName             string      `json:"siteName"`
		StatusCode           json.Number `json:"statusCode"`    // int
		SubStatusCode        json.Number `json:"subStatusCode"` // int
		TaskName             string      `json:"taskName"`
	} `json:"properties"`
}

func (r *azureAppServiceAuthenticationLog) PutProperties(attrs pcommon.Map, body pcommon.Value) error {
	unmarshaler.AttrPutStrIf(attrs, attributeAzureAuthEventDetails, r.Properties.Details)
	unmarshaler.AttrPutStrIf(attrs, string(conventions.HostNameKey), r.Properties.Host)
	unmarshaler.AttrPutStrIf(attrs, attributeAzureModuleRuntimeVersion, r.Properties.ModuleRuntimeVersion)
	unmarshaler.AttrPutStrIf(attrs, attributeAzureSiteName, r.Properties.SiteName)
	unmarshaler.AttrPutIntNumberIf(attrs, string(conventions.HTTPResponseStatusCodeKey), r.Properties.StatusCode)
	unmarshaler.AttrPutIntNumberIf(attrs, attributeAzureSubStatusCode, r.Properties.SubStatusCode)
	unmarshaler.AttrPutStrIf(attrs, attributeAzureTaskName, r.Properties.TaskName)
	body.SetStr(r.Properties.Message)

	return nil
}

// See https://learn.microsoft.com/en-us/azure/azure-monitor/reference/tables/appserviceconsolelogs
// There is no documentation about the structure of the logs, so we will
// implement this based on the fields from Azure Monitor table excluding generic fields
type azureAppServiceConsoleLog struct {
	azureLogRecordBase

	Properties struct {
		ContainerID string `json:"containerId"`
		Host        string `json:"host"`
	} `json:"properties"`
}

func (r *azureAppServiceConsoleLog) PutProperties(attrs pcommon.Map, _ pcommon.Value) error {
	unmarshaler.AttrPutStrIf(attrs, string(conventions.ContainerIDKey), r.Properties.ContainerID)
	unmarshaler.AttrPutStrIf(attrs, string(conventions.HostNameKey), r.Properties.Host)

	return nil
}

// See https://learn.microsoft.com/en-us/azure/azure-monitor/reference/tables/appservicehttplogs
// There is no documentation about the structure of the logs, so we will
// implement this based on the fields from Azure Monitor table excluding generic fields
type azureAppServiceHTTPLog struct {
	azureLogRecordBase

	Properties struct {
		ClientIP       string      `json:"CIp"`
		Host           string      `json:"ComputerName"`
		Cookie         string      `json:"Cookie"`
		RequestBytes   json.Number `json:"CsBytes"` // int
		HostHeader     string      `json:"CsHost"`
		RequestMethod  string      `json:"CsMethod"`
		URIQuery       string      `json:"CsUriQuery"`
		RequestPath    string      `json:"CsUriStem"`
		UserName       string      `json:"CsUsername"`
		Referer        string      `json:"Referer"`
		Result         string      `json:"Result"`
		ResponseBytes  json.Number `json:"ScBytes"`  // int
		HTTPStatusCode json.Number `json:"ScStatus"` // int
		HTTPSubStatus  string      `json:"ScSubStatus"`
		ServerPort     json.Number `json:"SPort"`     // int
		TimeTaken      json.Number `json:"TimeTaken"` // int
		UserAgent      string      `json:"UserAgent"`
	} `json:"properties"`
}

func (r *azureAppServiceHTTPLog) PutProperties(attrs pcommon.Map, body pcommon.Value) error {
	// In general it's unsafe to put Cookie values in Log Attributes as it may contain sensitive information,
	// and there is no generic masking available for it as well
	// So we will skip "Cookie" field here

	unmarshaler.AttrPutHostPortIf(attrs, string(conventions.ClientAddressKey), string(conventions.ClientPortKey), r.Properties.ClientIP)
	unmarshaler.AttrPutStrIf(attrs, string(conventions.ServerAddressKey), r.Properties.Host)
	unmarshaler.AttrPutIntNumberIf(attrs, string(conventions.ServerPortKey), r.Properties.ServerPort)
	unmarshaler.AttrPutIntNumberIf(attrs, string(conventions.HTTPRequestSizeKey), r.Properties.RequestBytes)
	unmarshaler.AttrPutStrIf(attrs, attributeHTTPHeaderHost, r.Properties.HostHeader)
	unmarshaler.AttrPutStrIf(attrs, string(conventions.HTTPRequestMethodKey), r.Properties.RequestMethod)
	unmarshaler.AttrPutStrIf(attrs, string(conventions.URLQueryKey), r.Properties.URIQuery)
	unmarshaler.AttrPutStrIf(attrs, string(conventions.URLPathKey), r.Properties.RequestPath)
	unmarshaler.AttrPutStrIf(attrs, string(conventions.UserNameKey), r.Properties.UserName)
	unmarshaler.AttrPutStrIf(attrs, attributeHTTPHeaderReferer, r.Properties.Referer)
	unmarshaler.AttrPutIntNumberIf(attrs, string(conventions.HTTPResponseSizeKey), r.Properties.ResponseBytes)
	unmarshaler.AttrPutIntNumberIf(attrs, string(conventions.HTTPResponseStatusCodeKey), r.Properties.HTTPStatusCode)
	unmarshaler.AttrPutStrIf(attrs, attributeAzureSubStatusCode, r.Properties.HTTPSubStatus)
	unmarshaler.AttrPutFloatNumberIf(attrs, attributeAzureRequestDuration, r.Properties.TimeTaken)
	unmarshaler.AttrPutStrIf(attrs, string(conventions.UserAgentOriginalKey), r.Properties.UserAgent)

	body.SetStr(r.Properties.Result)

	return nil
}

// See https://learn.microsoft.com/en-us/azure/azure-monitor/reference/tables/appserviceipsecauditlogs
// There is no documentation about the structure of the logs, so we will
// implement this based on the fields from Azure Monitor table excluding generic fields
type azureAppServiceIPSecAuditLog struct {
	azureLogRecordBase

	Properties struct {
		ClientIP          string `json:"CIp"`
		HostHeader        string `json:"CsHost"`
		Details           string `json:"details"`
		Result            string `json:"Result"`
		IsServiceEndpoint string `json:"ServiceEndpoint"`
		XAzureFDID        string `json:"XAzureFDID"`     // X-Azure-FDID header
		XFDHealthProbe    string `json:"XFDHealthProbe"` // X-FD-HealthProbe header
		XForwardedFor     string `json:"XForwardedFor"`  // X-Forwarded-For header
		XForwardedHost    string `json:"XForwardedHost"` // X-Forwarded-Host header
	} `json:"properties"`
}

func (r *azureAppServiceIPSecAuditLog) PutProperties(attrs pcommon.Map, body pcommon.Value) error {
	unmarshaler.AttrPutHostPortIf(attrs, string(conventions.SourceAddressKey), string(conventions.SourcePortKey), r.Properties.ClientIP)
	unmarshaler.AttrPutStrIf(attrs, attributeHTTPHeaderHost, r.Properties.HostHeader)
	unmarshaler.AttrPutStrIf(attrs, attributeAzureAuthEventDetails, r.Properties.Details)
	unmarshaler.AttrPutStrIf(attrs, attributeAzureIsServiceEndpoint, r.Properties.IsServiceEndpoint)
	unmarshaler.AttrPutStrIf(attrs, attributeHTTPHeaderAzureFDID, r.Properties.XAzureFDID)
	unmarshaler.AttrPutStrIf(attrs, attributeHTTPHeaderFDHealthProbe, r.Properties.XFDHealthProbe)
	unmarshaler.AttrPutStrIf(attrs, attributeHTTPHeaderForwardedFor, r.Properties.XForwardedFor)
	unmarshaler.AttrPutStrIf(attrs, attributeHTTPHeaderForwardedHost, r.Properties.XForwardedHost)

	body.SetStr(r.Properties.Result)

	return nil
}

// See https://learn.microsoft.com/en-us/azure/azure-monitor/reference/tables/appserviceplatformlogs
// There is no documentation about the structure of the logs, so we will
// implement this based on the fields from Azure Monitor table excluding generic fields
type azureAppServicePlatformLog struct {
	azureLogRecordBase

	Properties struct {
		ContainerID  string `json:"containerId"`
		DeploymentID string `json:"deploymentId"`
		Exception    string `json:"Exception"`
		Host         string `json:"host"`
		Message      string `json:"message"`
		StackTrace   string `json:"stackTrace"`
	} `json:"properties"`
}

func (r *azureAppServicePlatformLog) PutProperties(attrs pcommon.Map, body pcommon.Value) error {
	unmarshaler.AttrPutStrIf(attrs, string(conventions.ContainerIDKey), r.Properties.ContainerID)
	unmarshaler.AttrPutStrIf(attrs, attributeAzureDeploymentID, r.Properties.DeploymentID)
	unmarshaler.AttrPutStrIf(attrs, string(conventions.ExceptionMessageKey), r.Properties.Exception)
	unmarshaler.AttrPutStrIf(attrs, string(conventions.ExceptionStacktraceKey), r.Properties.StackTrace)
	unmarshaler.AttrPutStrIf(attrs, string(conventions.HostNameKey), r.Properties.Host)

	body.SetStr(r.Properties.Message)

	return nil
}

// See https://learn.microsoft.com/en-us/azure/azure-monitor/reference/tables/appservicefileauditlogs
// There is no documentation about the structure of the logs, so we will
// implement this based on the fields from Azure Monitor table excluding generic fields
type azureAppServiceFileAuditLog struct {
	azureLogRecordBase

	Properties struct {
		Path    string `json:"path"`
		Process string `json:"process"`
	} `json:"properties"`
}

func (r *azureAppServiceFileAuditLog) PutProperties(attrs pcommon.Map, _ pcommon.Value) error {
	unmarshaler.AttrPutStrIf(attrs, string(conventions.FilePathKey), r.Properties.Path)
	unmarshaler.AttrPutStrIf(attrs, string(conventions.ProcessTitleKey), r.Properties.Process)

	return nil
}
