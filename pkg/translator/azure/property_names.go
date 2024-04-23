package azure

var mappings = map[string]map[string]string{
	"common": {},
	"FrontDoorAccessLog": {
		"trackingReference":   "az.service_request_id",
		"httpMethod":          "http.request.method",
		"httpVersion":         "network.protocol.version",
		"requestUri":          "url.full",
		"hostName":            "server.address",
		"requestBytes":        "http.request.body.size",
		"responseBytes":       "http.response.body.size",
		"userAgent":           "user_agent.original",
		"clientIp":            "client.address",
		"clientPort":          "client.port",
		"socketIp":            "network.peer.address",
		"timeTaken":           "http.server.request.duration",
		"requestProtocol":     "network.protocol.name",
		"securityProtocol":    "",
		"securityCipher":      "",
		"securityCurves":      "",
		"endpoint":            "",
		"httpStatusCode":      "http.response.status_code",
		"pop":                 "",
		"cacheStatus":         "",
		"matchedRulesSetName": "",
		"routeName":           "http.route",
		"referrer":            "http.request.header.referer",
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
		"originName":                    "server.address",
		"originIP":                      "",
		"totalLatencyMilliseconds":      "",
		"connectionLatencyMilliseconds": "",
		"DNSLatencyMicroseconds":        "",
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
		"trackingReference": "",
		"policyMode":        "",
	},
	"AppServiceHTTPLogs": {
		"_BilledSize":     "",                          //real	The record size in bytes
		"CIp":             "client.address",            //string	IP address of the client
		"ComputerName":    "host.name",                 //string	The name of the server on which the log file entry was generated.
		"Cookie":          "",                          //string	Cookie on HTTP request
		"CsBytes":         "http.request.body.size",    //int	Number of bytes received by server
		"CsHost":          "url.domain",                //string	Host name header on HTTP request
		"CsMethod":        "http.request.method",       //string	The request HTTP verb
		"CsUriQuery":      "url.query",                 //string	URI query on HTTP request
		"CsUriStem":       "url.path",                  //string	The target of the request
		"CsUsername":      "",                          //string	The name of the authenticated user on HTTP request
		"_IsBillable":     "",                          //string	Specifies whether ingesting the data is billable. When _IsBillable is false ingestion isn't billed to your Azure account
		"Protocol":        "",                          //string	HTTP version
		"Referer":         "",                          //string	The site that the user last visited. This site provided a link to the current site
		"_ResourceId":     "",                          //string	A unique identifier for the resource that the record is associated with
		"Result":          "",                          //string	Success / Failure of HTTP request
		"ScBytes":         "http.response.body.size",   //int	Number of bytes sent by server
		"ScStatus":        "http.response.status_code", //int	HTTP status code
		"ScSubStatus":     "",                          //string	Substatus error code on HTTP request
		"ScWin32Status":   "",                          //string	Windows status code on HTTP request
		"SourceSystem":    "",                          //string	The type of agent the event was collected by. For example, OpsManager for Windows agent, either direct connect or Operations Manager, Linux for all Linux agents, or Azure for Azure Diagnostics
		"SPort":           "server.port",               //string	Server port number
		"_SubscriptionId": "",                          //string	A unique identifier for the subscription that the record is associated with
		"TenantId":        "",                          //string	The Log Analytics workspace ID
		"TimeGenerated":   "",                          //datetime	Time when event is generated
		"TimeTaken":       "",                          //int	Time taken by HTTP request in milliseconds
		"Type":            "",                          //string	The name of the table
		"UserAgent":       "",                          //string	User agent on HTTP request
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
