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
}

func ResourceLogKeyToSemConvKey(azName string, category string) (string, bool) {
	if mapping := mappings[category]; mapping != nil {
		if mapped := mapping[azName]; mapped != "" {
			return mapped, true
		}
	}

	if name := mappings["common"][azName]; name != "" {
		return name, true
	}

	return "", false
}
