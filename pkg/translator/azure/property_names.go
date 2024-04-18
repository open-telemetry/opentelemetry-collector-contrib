package azure

var mappings = map[string]map[string]string{
	"common": {},
	"FrontDoorAccessLog": {
		"trackingReference":   "",
		"httpMethod":          "http.request.method",
		"httpVersion":         "http.request.version",
		"requestUri":          "url.full",
		"hostName":            "server.address",
		"requestBytes":        "http.request.body.size",
		"responseBytes":       "http.response.body.size",
		"userAgent":           "user_agent.original",
		"clientIp":            "client.address",
		"clientPort":          "client.port",
		"socketIp":            "socket.address",
		"timeTaken":           "http.server.request.duration",
		"requestProtocol":     "",
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
		"errorInfo":           "",
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
		"result":                        "OriginError",
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
		"socketIP":          "socket.address",
		"requestUri":        "url.full",
		"ruleName":          "",
		"policy":            "",
		"action":            "",
		"host":              "server.address",
		"trackingReference": "",
		"policyMode":        "",
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
