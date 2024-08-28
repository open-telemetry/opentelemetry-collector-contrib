// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azurelogs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/azurelogs"

var mappings = map[string]map[string]string{
	"common": {},
	"AzureCdnAccessLog": {
		"BackendHostname":       "destination.address",       // If the request is being forwarded to a backend, this field represents the hostname of the backend. This field is blank if the request gets redirected or forwarded to a regional cache (when caching gets enabled for the routing rule).
		"CacheStatus":           "",                          // For caching scenarios, this field defines the cache hit/miss at the POP
		"ClientIp":              "client.address",            // The IP address of the client that made the request. If there was an X-Forwarded-For header in the request, then the Client IP is picked from the same.
		"ClientPort":            "client.port",               // The IP port of the client that made the request.
		"HttpMethod":            "http.request.method",       // HTTP method used by the request.
		"HttpStatusCode":        "http.response.status_code", // The HTTP status code returned from the proxy. If a request to the origin timeouts the value for HttpStatusCode is set to 0.
		"HttpStatusDetails":     "",                          // Resulting status on the request. Meaning of this string value can be found at a Status reference table.
		"HttpVersion":           "network.protocol.version",  // Type of the request or connection.
		"POP":                   "",                          // Short name of the edge where the request landed.
		"RequestBytes":          "http.request.size",         // The size of the HTTP request message in bytes, including the request headers and the request body.
		"RequestUri":            "url.full",                  // URI of the received request.
		"ResponseBytes":         "http.response.size",        // Bytes sent by the backend server as the response.
		"RoutingRuleName":       "",                          // The name of the routing rule that the request matched.
		"RulesEngineMatchNames": "",                          // The names of the rules that the request matched.
		"SecurityProtocol":      "",                          // handled by complex_conversions
		"isReceivedFromClient":  "",                          // If true, it means that the request came from the client. If false, the request is a miss in the edge (child POP) and is responded from origin shield (parent POP).
		"TimeTaken":             "",                          // The length of time from first byte of request into Azure Front Door to last byte of response out, in seconds.
		"TrackingReference":     "az.service_request_id",     // The unique reference string that identifies a request served by Azure Front Door, also sent as X-Azure-Ref header to the client. Required for searching details in the access logs for a specific request.
		"UserAgent":             "user_agent.original",       // The browser type that the client used.
		"ErrorInfo":             "error.type",                // This field contains the specific type of error to narrow down troubleshooting area.
		"TimeToFirstByte":       "",                          // The length of time in milliseconds from when Microsoft CDN receives the request to the time the first byte gets sent to the client. The time is measured only from the Microsoft side. Client-side data isn't measured.
		"Result":                "",                          // SSLMismatchedSNI is a status code that signifies a successful request with a mismatch warning between the Server Name Indication (SNI) and the host header. This status code implies domain fronting, a technique that violates Azure Front Door's terms of service. Requests with SSLMismatchedSNI will be rejected after January 22, 2024.
		"SNI":                   "",                          // This field specifies the Server Name Indication (SNI) that is sent during the TLS/SSL handshake. It can be used to identify the exact SNI value if there was a SSLMismatchedSNI status code. Additionally, it can be compared with the host value in the requestUri field to detect and resolve the mismatch issue.
	},
	"FrontDoorAccessLog": {
		"trackingReference":   "az.service_request_id",
		"httpMethod":          "http.request.method",
		"httpVersion":         "network.protocol.version",
		"requestUri":          "url.full",
		"hostName":            "server.address",
		"requestBytes":        "http.request.size",
		"responseBytes":       "http.response.size",
		"userAgent":           "user_agent.original",
		"clientIp":            "client.address",
		"clientPort":          "client.port",
		"socketIp":            "network.peer.address",
		"timeTaken":           "http.server.request.duration",
		"requestProtocol":     "network.protocol.name",
		"securityProtocol":    "", // handled by complex_conversions
		"securityCipher":      "tls.cipher",
		"securityCurves":      "tls.curve",
		"endpoint":            "",
		"httpStatusCode":      "http.response.status_code",
		"pop":                 "",
		"cacheStatus":         "",
		"matchedRulesSetName": "",
		"routeName":           "http.route",
		"referer":             "http.request.header.referer",
		"timeToFirstByte":     "",
		"errorInfo":           "error.type",
		"originURL":           "",
		"originIP":            "",
		"originName":          "",
		"result":              "",
		"sni":                 "",
	},
	"FrontDoorHealthProbeLog": {
		"healthProbeId":                 "",
		"POP":                           "",
		"httpVerb":                      "http.request.method",
		"result":                        "",
		"httpStatusCode":                "http.response.status_code",
		"probeURL":                      "url.full",
		"originName":                    "",
		"originIP":                      "server.address",
		"totalLatencyMilliseconds":      "", // handled by complex_conversions
		"connectionLatencyMilliseconds": "",
		"DNSLatencyMicroseconds":        "", // handled by complex_conversions
	},
	"FrontdoorWebApplicationFirewallLog": {
		"clientIP":          "client.address",
		"clientPort":        "client.port",
		"socketIP":          "network.peer.address",
		"requestUri":        "url.full",
		"ruleName":          "",
		"policy":            "",
		"action":            "",
		"host":              "server.address",
		"trackingReference": "az.service_request_id",
		"policyMode":        "",
	},
	"AppServiceAppLogs": {
		"_BilledSize":       "",                     // real	The record size in bytes
		"Category":          "",                     // string	Log category name
		"ContainerId":       "container.id",         // string	Application container id
		"CustomLevel":       "",                     // string	Verbosity level of log
		"ExceptionClass":    "exception.type",       // string	Application class from where log message is emitted
		"Host":              "host.id",              // string	Host where the application is running
		"_IsBillable":       "",                     // string	Specifies whether ingesting the data is billable. When _IsBillable is false ingestion isn't billed to your Azure account
		"Level":             "",                     // string	Verbosity level of log mapped to standard levels (Informational, Warning, Error, or Critical)
		"Logger":            "",                     // string	Application logger used to emit log message
		"Message":           "",                     // string	Log message
		"Method":            "code.function",        // string	Application Method from where log message is emitted
		"OperationName":     "",                     // string	The name of the operation represented by this event.
		"_ResourceId":       "",                     // string	A unique identifier for the resource that the record is associated with
		"ResultDescription": "",                     // string	Log message description
		"Source":            "code.filepath",        // string	Application source from where log message is emitted
		"SourceSystem":      "",                     // string	The type of agent the event was collected by. For example, OpsManager for Windows agent, either direct connect or Operations Manager, Linux for all Linux agents, or Azure for Azure Diagnostics
		"Stacktrace":        "exception.stacktrace", // string	Complete stack trace of the log message in case of exception
		"StackTrace":        "exception.stacktrace", // string	Complete stack trace of the log message in case of exception
		"_SubscriptionId":   "",                     // string	A unique identifier for the subscription that the record is associated with
		"TenantId":          "",                     // string	The Log Analytics workspace ID
		"TimeGenerated":     "",                     // datetime	Time when event is generated
		"Type":              "",                     // string	The name of the table
		"WebSiteInstanceId": "",                     // string	Instance ID of the application running
	},
	"AppServiceAuditLogs": {
		"_BilledSize":     "",                      // real	The record size in bytes
		"Category":        "",                      // string	Log category name
		"_IsBillable":     "",                      // string	Specifies whether ingesting the data is billable. When _IsBillable is false ingestion isn't billed to your Azure account
		"OperationName":   "",                      // string	Name of the operation
		"Protocol":        "network.protocol.name", // string	Authentication protocol
		"_ResourceId":     "",                      // string	A unique identifier for the resource that the record is associated with
		"SourceSystem":    "",                      // string	The type of agent the event was collected by. For example, OpsManager for Windows agent, either direct connect or Operations Manager, Linux for all Linux agents, or Azure for Azure Diagnostics
		"_SubscriptionId": "",                      // string	A unique identifier for the subscription that the record is associated with
		"TenantId":        "",                      // string	The Log Analytics workspace ID
		"TimeGenerated":   "",                      // datetime	Time when event is generated
		"Type":            "",                      // string	The name of the table
		"User":            "enduser.id",            // string	Username used for publishing access
		"UserAddress":     "client.address",        // string	Client IP address of the publishing user
		"UserDisplayName": "",                      // string	Email address of a user in case publishing was authorized via AAD authentication
	},
	"AppServiceAuthenticationLogs": {
		"_BilledSize":          "",                          // real	The record size in bytes
		"CorrelationId":        "",                          // string	The ID for correlated events.
		"Details":              "",                          // string	The event details.
		"HostName":             "",                          // string	The host name of the application.
		"_IsBillable":          "",                          // string	Specifies whether ingesting the data is billable. When _IsBillable is false ingestion isn't billed to your Azure account
		"Level":                "",                          // string	The level of log verbosity.
		"Message":              "",                          // string	The log message.
		"ModuleRuntimeVersion": "",                          // string	The version of App Service Authentication running.
		"OperationName":        "",                          // string	The name of the operation represented by this event.
		"_ResourceId":          "",                          // string	A unique identifier for the resource that the record is associated with
		"SiteName":             "",                          // string	The runtime name of the application.
		"SourceSystem":         "",                          // string	The type of agent the event was collected by. For example, OpsManager for Windows agent, either direct connect or Operations Manager, Linux for all Linux agents, or Azure for Azure Diagnostics
		"StatusCode":           "http.response.status_code", // int	The HTTP status code of the operation.
		"_SubscriptionId":      "",                          // string	A unique identifier for the subscription that the record is associated with
		"SubStatusCode":        "",                          // int	The HTTP sub-status code of the request.
		"TaskName":             "",                          // string	The name of the task being performed.
		"TenantId":             "",                          // string	The Log Analytics workspace ID
		"TimeGenerated":        "",                          // datetime	The timestamp (UTC) of when this event was generated.
		"Type":                 "",                          // string	The name of the table
	},
	"AppServiceConsoleLogs": {
		"_BilledSize":       "",             // real	The record size in bytes
		"Category":          "",             // string	Log category name
		"ContainerId":       "container.id", // string	Application container id
		"Host":              "host.id",      // string	Host where the application is running
		"_IsBillable":       "",             // string	Specifies whether ingesting the data is billable. When _IsBillable is false ingestion isn't billed to your Azure account
		"Level":             "",             // string	Verbosity level of log
		"OperationName":     "",             // string	The name of the operation represented by this event.
		"_ResourceId":       "",             // string	A unique identifier for the resource that the record is associated with
		"ResultDescription": "",             // string	Log message description
		"SourceSystem":      "",             // string	The type of agent the event was collected by. For example, OpsManager for Windows agent, either direct connect or Operations Manager, Linux for all Linux agents, or Azure for Azure Diagnostics
		"_SubscriptionId":   "",             // string	A unique identifier for the subscription that the record is associated with
		"TenantId":          "",             // string	The Log Analytics workspace ID
		"TimeGenerated":     "",             // datetime	Time when event is generated
		"Type":              "",             // string	The name of the table
	},
	"AppServiceEnvironmentPlatformLogs": {
		"_BilledSize":       "", // real	The record size in bytes
		"Category":          "", // string
		"_IsBillable":       "", // string	Specifies whether ingesting the data is billable. When _IsBillable is false ingestion isn't billed to your Azure account
		"OperationName":     "", // string
		"ResourceId":        "", // string
		"_ResourceId":       "", // string	A unique identifier for the resource that the record is associated with
		"ResultDescription": "", // string
		"ResultType":        "", // string
		"SourceSystem":      "", // string	The type of agent the event was collected by. For example, OpsManager for Windows agent, either direct connect or Operations Manager, Linux for all Linux agents, or Azure for Azure Diagnostics
		"_SubscriptionId":   "", // string	A unique identifier for the subscription that the record is associated with
		"TimeGenerated":     "", // datetime
		"Type":              "", // string	The name of the table
	},
	"AppServiceFileAuditLogs": {
		"_BilledSize":     "", // real	The record size in bytes
		"Category":        "", // string	Log category name
		"_IsBillable":     "", // string	Specifies whether ingesting the data is billable. When _IsBillable is false ingestion isn't billed to your Azure account
		"OperationName":   "", // string	Operation performed on a file
		"Path":            "", // string	Path to the file that was changed
		"Process":         "", // string	Type of the process that change the file
		"_ResourceId":     "", // string	A unique identifier for the resource that the record is associated with
		"SourceSystem":    "", // string	The type of agent the event was collected by. For example, OpsManager for Windows agent, either direct connect or Operations Manager, Linux for all Linux agents, or Azure for Azure Diagnostics
		"_SubscriptionId": "", // string	A unique identifier for the subscription that the record is associated with
		"TenantId":        "", // string	The Log Analytics workspace ID
		"TimeGenerated":   "", // datetime	Time when event is generated
		"Type":            "", // string	The name of the table
	},
	"AppServiceHTTPLogs": {
		"_BilledSize":     "",                             // real	The record size in bytes
		"CIp":             "client.address",               // string	IP address of the client
		"ComputerName":    "host.name",                    // string	The name of the server on which the log file entry was generated.
		"Cookie":          "",                             // string	Cookie on HTTP request
		"CsBytes":         "http.request.body.size",       // int	Number of bytes received by server
		"CsHost":          "url.domain",                   // string	Host name header on HTTP request
		"CsMethod":        "http.request.method",          // string	The request HTTP verb
		"CsUriQuery":      "url.query",                    // string	URI query on HTTP request
		"CsUriStem":       "url.path",                     // string	The target of the request
		"CsUsername":      "",                             // string	The name of the authenticated user on HTTP request
		"_IsBillable":     "",                             // string	Specifies whether ingesting the data is billable. When _IsBillable is false ingestion isn't billed to your Azure account
		"Protocol":        "",                             // handled by complex_conversions
		"Referer":         "http.request.header.referer",  // string	The site that the user last visited. This site provided a link to the current site
		"_ResourceId":     "",                             // string	A unique identifier for the resource that the record is associated with
		"Result":          "",                             // string	Success / Failure of HTTP request
		"ScBytes":         "http.response.body.size",      // int	Number of bytes sent by server
		"ScStatus":        "http.response.status_code",    // int	HTTP status code
		"ScSubStatus":     "",                             // string	Sub-status error code on HTTP request
		"ScWin32Status":   "",                             // string	Windows status code on HTTP request
		"SourceSystem":    "",                             // string	The type of agent the event was collected by. For example, OpsManager for Windows agent, either direct connect or Operations Manager, Linux for all Linux agents, or Azure for Azure Diagnostics
		"SPort":           "server.port",                  // string	Server port number
		"_SubscriptionId": "",                             // string	A unique identifier for the subscription that the record is associated with
		"TenantId":        "",                             // string	The Log Analytics workspace ID
		"TimeGenerated":   "",                             // datetime	Time when event is generated
		"TimeTaken":       "http.server.request.duration", // int	Time taken by HTTP request in milliseconds
		"Type":            "",                             // string	The name of the table
		"UserAgent":       "user_agent.original",          // string	User agent on HTTP request
	},
	"AppServiceIPSecAuditLogs": {
		"_BilledSize":     "",                                     // real	The record size in bytes
		"CIp":             "client.address",                       // string	IP address of the client
		"CsHost":          "url.domain",                           // string	Host header of the HTTP request
		"Details":         "",                                     // string	Additional information
		"_IsBillable":     "",                                     // string	Specifies whether ingesting the data is billable. When _IsBillable is false ingestion isn't billed to your Azure account
		"_ResourceId":     "",                                     // string	A unique identifier for the resource that the record is associated with
		"Result":          "",                                     // string	The result whether the access is Allowed or Denied
		"ServiceEndpoint": "",                                     // string	This indicates whether the access is via Virtual Network Service Endpoint communication
		"SourceSystem":    "",                                     // string	The type of agent the event was collected by. For example, OpsManager for Windows agent, either direct connect or Operations Manager, Linux for all Linux agents, or Azure for Azure Diagnostics
		"_SubscriptionId": "",                                     // string	A unique identifier for the subscription that the record is associated with
		"TenantId":        "",                                     // string	The Log Analytics workspace ID
		"TimeGenerated":   "",                                     // datetime	Time of the Http Request
		"Type":            "",                                     // string	The name of the table
		"XAzureFDID":      "http.request.header.x-azure-fdid",     // string	X-Azure-FDID header (Azure Frontdoor ID) of the HTTP request
		"XFDHealthProbe":  "http.request.header.x-fd-healthprobe", // string	X-FD-HealthProbe (Azure Frontdoor Health Probe) of the HTTP request
		"XForwardedFor":   "http.request.header.x-forwarded-for",  // string	X-Forwarded-For header of the HTTP request
		"XForwardedHost":  "http.request.header.x-forwarded-host", // string	X-Forwarded-Host header of the HTTP request
	},
	"AppServicePlatformLogs": {
		"ActivityId":      "",               // string	Activity ID to correlate events
		"_BilledSize":     "",               // real	The record size in bytes
		"containerId":     "container.id",   // string	Application container id
		"containerName":   "container.name", // string	Application container id
		"DeploymentId":    "",               // string	Deployment ID of the application deployment
		"exception":       "error.type",     // string	Details of the exception
		"Host":            "",               // string	Host where the application is running
		"_IsBillable":     "",               // string	Specifies whether ingesting the data is billable. When _IsBillable is false ingestion isn't billed to your Azure account
		"Level":           "",               // string	Level of log verbosity
		"Message":         "",               // string	Log message
		"OperationName":   "",               // string	The name of the operation represented by this event.
		"_ResourceId":     "",               // string	A unique identifier for the resource that the record is associated with
		"SourceSystem":    "",               // string	The type of agent the event was collected by. For example, OpsManager for Windows agent, either direct connect or Operations Manager, Linux for all Linux agents, or Azure for Azure Diagnostics
		"StackTrace":      "",               // string	Stack trace for the exception
		"_SubscriptionId": "",               // string	A unique identifier for the subscription that the record is associated with
		"TenantId":        "",               // string	The Log Analytics workspace ID
		"TimeGenerated":   "",               // datetime	Time when event is generated
		"Type":            "",               // string	The name of the table
	},
	"AppServiceServerlessSecurityPluginData": {
		"_BilledSize":     "", // real	The record size in bytes
		"Index":           "", // int	Available when multiple payloads exist for the same message. In that case, payloads share the same SlSecRequestId and Index defines the chronological order of payloads.
		"_IsBillable":     "", // string	Specifies whether ingesting the data is billable. When _IsBillable is false ingestion isn't billed to your Azure account
		"MsgVersion":      "", // string	The version of the message schema. Used to make code changes backward- and forward- compatible.
		"Payload":         "", // dynamic	An array of messages, where each one is a JSON string.
		"PayloadType":     "", // string	The type of the payload. Mostly used to distinguish between messages meant for different types of security analysis.
		"_ResourceId":     "", // string	A unique identifier for the resource that the record is associated with
		"Sender":          "", // string	The name of the component that published this message. Almost always will be the name of the plugin, but can also be platform.
		"SlSecMetadata":   "", // dynamic	Contains details about the resource like the deployment ID, runtime info, website info, OS, etc.
		"SlSecProps":      "", // dynamic	Contains other details that might be needed for debugging end-to-end requests, e.g., slsec nuget version.
		"SlSecRequestId":  "", // string	The ingestion request ID used for identifying the message and the request for diagnostics and debugging.
		"SourceSystem":    "", // string	The type of agent the event was collected by. For example, OpsManager for Windows agent, either direct connect or Operations Manager, Linux for all Linux agents, or Azure for Azure Diagnostics
		"_SubscriptionId": "", // string	A unique identifier for the subscription that the record is associated with
		"TenantId":        "", // string	The Log Analytics workspace ID
		"TimeGenerated":   "", // datetime	The date and time (UTC) this message was created on the node.
		"Type":            "", // string	The name of the table
	},
}

func resourceLogKeyToSemConvKey(azName string, category string) (string, bool) {
	mapping, ok := mappings[category]
	if ok {
		if mapped := mapping[azName]; mapped != "" {
			return mapped, true
		}
	}

	mapping = mappings["common"]
	if name := mapping[azName]; name != "" {
		return name, true
	}

	return "", false
}
