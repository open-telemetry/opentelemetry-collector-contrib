// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azurelogs // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/azurelogs"

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	gojson "github.com/goccy/go-json"
	"go.opentelemetry.io/collector/pdata/plog"
	conventions "go.opentelemetry.io/otel/semconv/v1.27.0"
)

const (
	noError = "NoError"

	categoryAzureCdnAccessLog                  = "AzureCdnAccessLog"
	categoryFrontDoorAccessLog                 = "FrontDoorAccessLog"
	categoryFrontDoorHealthProbeLog            = "FrontDoorHealthProbeLog"
	categoryFrontdoorWebApplicationFirewallLog = "FrontDoorWebApplicationFirewallLog"
	categoryAppServiceAppLogs                  = "AppServiceAppLogs"
	categoryAppServiceAuditLogs                = "AppServiceAuditLogs"
	categoryAppServiceAuthenticationLogs       = "AppServiceAuthenticationLogs"
	categoryAppServiceConsoleLogs              = "AppServiceConsoleLogs"
	categoryAppServiceHTTPLogs                 = "AppServiceHTTPLogs"
	categoryAppServiceIPSecAuditLogs           = "AppServiceIPSecAuditLogs"
	categoryAppServicePlatformLogs             = "AppServicePlatformLogs"

	// attributeAzureRef holds the request tracking reference, also
	// placed in the request header "X-Azure-Ref".
	attributeAzureRef = "azure.ref"

	// attributeTimeToFirstByte holds the length of time in milliseconds
	// from when Microsoft service (CDN, Front Door, etc) receives the
	// request to the time the first byte gets sent to the client.
	attributeTimeToFirstByte = "azure.time_to_first_byte"

	// attributeDuration holds the the length of time from first byte of
	// request to last byte of response out, in seconds.
	attributeDuration = "duration"

	// attributeAzurePop holds the point of presence (POP) that
	// processed the request
	attributeAzurePop = "azure.pop"

	// attributeCacheStatus holds the result of the cache hit/miss
	// at the POP
	attributeCacheStatus = "azure.cache_status"

	// attributeTLSServerName holds the server name indication (SNI)
	// value
	attributeTLSServerName = "tls.server.name"

	missingPort = "missing port in address"
)

const (
	// azure front door WAF attributes

	// attributeAzureFrontDoorWAFRuleName holds the name of the WAF rule that
	// the request matched.
	attributeAzureFrontDoorWAFRuleName = "azure.frontdoor.waf.rule.name"

	// attributeAzureFrontDoorWAFPolicyName holds the name of the WAF policy
	// that processed the request.
	attributeAzureFrontDoorWAFPolicyName = "azure.frontdoor.waf.policy.name"

	// attributeAzureFrontDoorWAFPolicyMode holds the operations mode of the
	// WAF policy.
	attributeAzureFrontDoorWAFPolicyMode = "azure.frontdoor.waf.policy.mode"

	// attributeAzureFrontDoorWAFAction holds the action taken on the request.
	attributeAzureFrontDoorWAFAction = "azure.frontdoor.waf.action"
)

var (
	errStillToImplement    = errors.New("still to implement")
	errUnsupportedCategory = errors.New("category not supported")
)

func addRecordAttributes(category string, data []byte, record plog.LogRecord) error {
	var err error

	switch category {
	case categoryAzureCdnAccessLog:
		err = addAzureCdnAccessLogProperties(data, record)
	case categoryFrontDoorAccessLog:
		err = addFrontDoorAccessLogProperties(data, record)
	case categoryFrontDoorHealthProbeLog:
		err = addFrontDoorHealthProbeLogProperties(data, record)
	case categoryFrontdoorWebApplicationFirewallLog:
		err = addFrontDoorWAFLogProperties(data, record)
	case categoryAppServiceAppLogs:
		err = addAppServiceAppLogsProperties(data, record)
	case categoryAppServiceAuditLogs:
		err = addAppServiceAuditLogsProperties(data, record)
	case categoryAppServiceAuthenticationLogs:
		err = addAppServiceAuthenticationLogsProperties(data, record)
	case categoryAppServiceConsoleLogs:
		err = addAppServiceConsoleLogsProperties(data, record)
	case categoryAppServiceHTTPLogs:
		err = addAppServiceHTTPLogsProperties(data, record)
	case categoryAppServiceIPSecAuditLogs:
		err = addAppServiceIPSecAuditLogsProperties(data, record)
	case categoryAppServicePlatformLogs:
		err = addAppServicePlatformLogsProperties(data, record)
	default:
		err = errUnsupportedCategory
	}

	if err != nil {
		return fmt.Errorf("failed to parse logs from category %q: %w", category, err)
	}
	return nil
}

// putInt parses value as an int and puts it in the record
func putInt(field string, value string, record plog.LogRecord) error {
	n, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return fmt.Errorf("failed to get number in %q for field %q: %w", value, field, err)
	}
	record.Attributes().PutInt(field, n)
	return nil
}

// putStr puts the value in the record if the value holds
// meaningful data. Meaningful data is defined as not being empty
// or "N/A".
func putStr(field string, value string, record plog.LogRecord) {
	switch value {
	case "", "N/A":
		// ignore
	default:
		record.Attributes().PutStr(field, value)
	}
}

// handleTime parses the time value and always multiplies it by
// 1e3. This is so we don't loose so much data if the time is for
// example "0.154". In that case, the output would be "154".
func handleTime(field string, value string, record plog.LogRecord) error {
	n, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return fmt.Errorf("failed to get number in %q for field %q: %w", value, field, err)
	}
	ns := int64(n * 1e3)
	record.Attributes().PutInt(field, ns)
	return nil
}

// See https://github.com/MicrosoftDocs/azure-docs/blob/main/articles/cdn/monitoring-and-access-log.md
type azureCdnAccessLogProperties struct {
	TrackingReference    string `json:"trackingReference"`
	HTTPMethod           string `json:"httpMethod"`
	HTTPVersion          string `json:"httpVersion"`
	RequestURI           string `json:"requestUri"`
	SNI                  string `json:"sni"`
	RequestBytes         string `json:"requestBytes"`
	ResponseBytes        string `json:"responseBytes"`
	UserAgent            string `json:"userAgent"`
	ClientIP             string `json:"clientIp"`
	ClientPort           string `json:"clientPort"`
	SocketIP             string `json:"socketIp"`
	TimeToFirstByte      string `json:"timeToFirstByte"`
	TimeTaken            string `json:"timeTaken"`
	RequestProtocol      string `json:"requestProtocol"`
	SecurityProtocol     string `json:"securityProtocol"`
	HTTPStatusCode       string `json:"httpStatusCode"`
	Pop                  string `json:"pop"`
	CacheStatus          string `json:"cacheStatus"`
	ErrorInfo            string `json:"errorInfo"`
	ErrorInfo1           string `json:"ErrorInfo"`
	Endpoint             string `json:"endpoint"`
	IsReceivedFromClient bool   `json:"isReceivedFromClient"`
	BackendHostname      string `json:"backendHostname"`
}

// addRequestURIProperties parses the request URI and adds the
// relevant attributes to the record
func addRequestURIProperties(uri string, record plog.LogRecord) error {
	if uri == "" {
		return nil
	}

	u, errURL := url.Parse(uri)
	if errURL != nil {
		return fmt.Errorf("failed to parse request URI %q: %w", uri, errURL)
	}
	record.Attributes().PutStr(string(conventions.URLOriginalKey), uri)

	if port := u.Port(); port != "" {
		if err := putInt(string(conventions.URLPortKey), u.Port(), record); err != nil {
			return fmt.Errorf("failed to get port number from value %q: %w", port, err)
		}
	}

	putStr(string(conventions.URLSchemeKey), u.Scheme, record)
	putStr(string(conventions.URLPathKey), u.Path, record)
	putStr(string(conventions.URLQueryKey), u.RawQuery, record)
	putStr(string(conventions.URLFragmentKey), u.Fragment, record)

	return nil
}

// addSecurityProtocolProperties based on the security protocol
func addSecurityProtocolProperties(securityProtocol string, record plog.LogRecord) error {
	if securityProtocol == "" {
		return nil
	}
	name, remaining, _ := strings.Cut(securityProtocol, " ")
	if remaining == "" {
		return fmt.Errorf(`security protocol %q is missing version, expects format "<name> <version>"`, securityProtocol)
	}
	version, remaining, _ := strings.Cut(remaining, " ")
	if remaining != "" {
		return fmt.Errorf(`security protocol %q has invalid format, expects "<name> <version>"`, securityProtocol)
	}

	record.Attributes().PutStr(string(conventions.TLSProtocolNameKey), name)
	record.Attributes().PutStr(string(conventions.TLSProtocolVersionKey), version)

	return nil
}

// addErrorInfoProperties checks if there is an error and adds it
// to the record attributes as an exception in case it exists.
func addErrorInfoProperties(errorInfo string, record plog.LogRecord) {
	if errorInfo == noError {
		return
	}
	record.Attributes().PutStr(string(conventions.ExceptionTypeKey), errorInfo)
}

// handleDestination puts the value for the backend host name and endpoint
// in the expected field names. If backend hostname is empty, then the
// destination address and port depend only on the endpoint. If backend
// hostname is filled, then the destination address and port are based on
// it. If both are filled but different, then the endpoint will cover the
// network address and port.
func handleDestination(backendHostname string, endpoint string, record plog.LogRecord) error {
	addFields := func(full string, addressField string, portField string) error {
		host, port, err := net.SplitHostPort(full)
		if err != nil && strings.HasSuffix(err.Error(), missingPort) {
			// there is no port, so let's keep using the full endpoint for the address
			host = full
		} else if err != nil {
			return err
		}
		if host != "" {
			record.Attributes().PutStr(addressField, host)
		}
		if port != "" {
			if err = putInt(portField, port, record); err != nil {
				return err
			}
		}
		return nil
	}

	if backendHostname == "" {
		if endpoint == "" {
			return nil
		}
		err := addFields(endpoint, string(conventions.DestinationAddressKey), string(conventions.DestinationPortKey))
		if err != nil {
			return fmt.Errorf("failed to parse endpoint %q: %w", endpoint, err)
		}
	} else {
		err := addFields(backendHostname, string(conventions.DestinationAddressKey), string(conventions.DestinationPortKey))
		if err != nil {
			return fmt.Errorf("failed to parse backend hostname %q: %w", backendHostname, err)
		}

		if endpoint != backendHostname && endpoint != "" {
			err = addFields(endpoint, string(conventions.NetworkPeerAddressKey), string(conventions.NetworkPeerPortKey))
			if err != nil {
				return fmt.Errorf("failed to parse endpoint %q: %w", endpoint, err)
			}
		}
	}

	return nil
}

// addAzureCdnAccessLogProperties parses the Azure CDN access log, and adds
// the relevant attributes to the record
func addAzureCdnAccessLogProperties(data []byte, record plog.LogRecord) error {
	var properties azureCdnAccessLogProperties
	if err := gojson.Unmarshal(data, &properties); err != nil {
		return fmt.Errorf("failed to parse AzureCdnAccessLog properties: %w", err)
	}

	if err := putInt(string(conventions.HTTPRequestSizeKey), properties.RequestBytes, record); err != nil {
		return err
	}
	if err := putInt(string(conventions.HTTPResponseSizeKey), properties.ResponseBytes, record); err != nil {
		return err
	}
	if err := putInt(string(conventions.ClientPortKey), properties.ClientPort, record); err != nil {
		return err
	}
	if err := putInt(string(conventions.HTTPResponseStatusCodeKey), properties.HTTPStatusCode, record); err != nil {
		return err
	}

	if err := handleTime(attributeTimeToFirstByte, properties.TimeToFirstByte, record); err != nil {
		return err
	}
	if err := handleTime(attributeDuration, properties.TimeTaken, record); err != nil {
		return err
	}

	if err := addRequestURIProperties(properties.RequestURI, record); err != nil {
		return fmt.Errorf(`failed to handle "requestUri" field: %w`, err)
	}
	if err := addSecurityProtocolProperties(properties.SecurityProtocol, record); err != nil {
		return err
	}
	if err := handleDestination(properties.BackendHostname, properties.Endpoint, record); err != nil {
		return err
	}

	if properties.ErrorInfo != properties.ErrorInfo1 && properties.ErrorInfo != "" && properties.ErrorInfo1 != "" {
		return errors.New(`unexpected: "errorInfo" and "ErrorInfo" JSON fields have different values`)
	}
	if properties.ErrorInfo1 != "" {
		addErrorInfoProperties(properties.ErrorInfo1, record)
	} else if properties.ErrorInfo != "" {
		addErrorInfoProperties(properties.ErrorInfo, record)
	}

	putStr(attributeAzureRef, properties.TrackingReference, record)
	putStr(string(conventions.HTTPRequestMethodKey), properties.HTTPMethod, record)
	putStr(string(conventions.NetworkProtocolVersionKey), properties.HTTPVersion, record)
	putStr(string(conventions.NetworkProtocolNameKey), properties.RequestProtocol, record)
	putStr(attributeTLSServerName, properties.SNI, record)
	putStr(string(conventions.UserAgentOriginalKey), properties.UserAgent, record)
	putStr(string(conventions.ClientAddressKey), properties.ClientIP, record)
	putStr(string(conventions.SourceAddressKey), properties.SocketIP, record)

	putStr(attributeAzurePop, properties.Pop, record)
	putStr(attributeCacheStatus, properties.CacheStatus, record)

	if properties.IsReceivedFromClient {
		record.Attributes().PutStr(string(conventions.NetworkIoDirectionKey), "receive")
	} else {
		record.Attributes().PutStr(string(conventions.NetworkIoDirectionKey), "transmit")
	}

	return nil
}

// See https://learn.microsoft.com/en-us/azure/frontdoor/monitor-front-door?pivots=front-door-standard-premium#access-log.
type frontDoorAccessLog struct {
	TrackingReference string `json:"trackingReference"`
	HTTPMethod        string `json:"httpMethod"`
	HTTPVersion       string `json:"httpVersion"`
	RequestURI        string `json:"requestUri"`
	SNI               string `json:"sni"`
	RequestBytes      string `json:"requestBytes"`
	ResponseBytes     string `json:"responseBytes"`
	UserAgent         string `json:"userAgent"`
	ClientIP          string `json:"clientIp"`
	ClientPort        string `json:"clientPort"`
	SocketIP          string `json:"socketIp"`
	TimeToFirstByte   string `json:"timeToFirstByte"`
	TimeTaken         string `json:"timeTaken"`
	RequestProtocol   string `json:"requestProtocol"`
	SecurityProtocol  string `json:"securityProtocol"`
	HTTPStatusCode    string `json:"httpStatusCode"`
	Pop               string `json:"pop"`
	CacheStatus       string `json:"cacheStatus"`
	ErrorInfo         string `json:"errorInfo"`
	ErrorInfo1        string `json:"ErrorInfo"`
	Result            string `json:"result"`
	Endpoint          string `json:"endpoint"`
	HostName          string `json:"hostName"`
	SecurityCipher    string `json:"securityCipher"`
	SecurityCurves    string `json:"securityCurves"`
	OriginIP          string `json:"originIp"`
}

// addFrontDoorAccessLogProperties parses the Front Door access log, and adds
// the relevant attributes to the record
func addFrontDoorAccessLogProperties(data []byte, record plog.LogRecord) error {
	var properties frontDoorAccessLog
	if err := gojson.Unmarshal(data, &properties); err != nil {
		return fmt.Errorf("failed to parse FrontDoorAccessLog properties: %w", err)
	}

	if err := putInt(string(conventions.HTTPRequestSizeKey), properties.RequestBytes, record); err != nil {
		return err
	}
	if err := putInt(string(conventions.HTTPResponseSizeKey), properties.ResponseBytes, record); err != nil {
		return err
	}
	if err := putInt(string(conventions.ClientPortKey), properties.ClientPort, record); err != nil {
		return err
	}
	if err := putInt(string(conventions.HTTPResponseStatusCodeKey), properties.HTTPStatusCode, record); err != nil {
		return err
	}

	if err := handleTime(attributeTimeToFirstByte, properties.TimeToFirstByte, record); err != nil {
		return err
	}
	if err := handleTime(attributeDuration, properties.TimeTaken, record); err != nil {
		return err
	}

	if err := addRequestURIProperties(properties.RequestURI, record); err != nil {
		return fmt.Errorf(`failed to handle "requestUri" field: %w`, err)
	}
	if err := addSecurityProtocolProperties(properties.SecurityProtocol, record); err != nil {
		return err
	}
	if err := handleDestination(properties.HostName, properties.Endpoint, record); err != nil {
		return err
	}

	if properties.ErrorInfo != properties.ErrorInfo1 && properties.ErrorInfo != "" && properties.ErrorInfo1 != "" {
		return errors.New(`unexpected: "errorInfo" and "ErrorInfo" JSON fields have different values`)
	}
	if properties.ErrorInfo1 != "" {
		addErrorInfoProperties(properties.ErrorInfo1, record)
	} else if properties.ErrorInfo != "" {
		addErrorInfoProperties(properties.ErrorInfo, record)
	}

	if properties.OriginIP != "" && properties.OriginIP != "N/A" {
		address, port, _ := strings.Cut(properties.OriginIP, ":")
		putStr(string(conventions.ServerAddressKey), address, record)
		if port != "" {
			if err := putInt(string(conventions.ServerPortKey), port, record); err != nil {
				return err
			}
		}
	}

	putStr(attributeAzureRef, properties.TrackingReference, record)
	putStr(string(conventions.HTTPRequestMethodKey), properties.HTTPMethod, record)
	putStr(string(conventions.NetworkProtocolVersionKey), properties.HTTPVersion, record)
	putStr(string(conventions.NetworkProtocolNameKey), properties.RequestProtocol, record)
	putStr(attributeTLSServerName, properties.SNI, record)
	putStr(string(conventions.UserAgentOriginalKey), properties.UserAgent, record)
	putStr(string(conventions.ClientAddressKey), properties.ClientIP, record)
	putStr(string(conventions.SourceAddressKey), properties.SocketIP, record)

	putStr(attributeAzurePop, properties.Pop, record)
	putStr(attributeCacheStatus, properties.CacheStatus, record)

	putStr(string(conventions.TLSCurveKey), properties.SecurityCurves, record)
	putStr(string(conventions.TLSCipherKey), properties.SecurityCipher, record)

	return nil
}

// addFrontDoorHealthProbeLogProperties parses the Front Door access log, and adds
// the relevant attributes to the record
func addFrontDoorHealthProbeLogProperties(_ []byte, _ plog.LogRecord) error {
	// TODO @constanca-m implement this the same way as addAzureCdnAccessLogProperties
	return errStillToImplement
}

// See https://learn.microsoft.com/en-us/azure/web-application-firewall/afds/waf-front-door-monitor?pivots=front-door-standard-premium#waf-logs
type frontDoorWAFLogProperties struct {
	ClientIP          string `json:"clientIP"`
	ClientPort        string `json:"clientPort"`
	SocketIP          string `json:"socketIP"`
	RequestURI        string `json:"requestUri"`
	RuleName          string `json:"ruleName"`
	Policy            string `json:"policy"`
	Action            string `json:"action"`
	Host              string `json:"host"`
	TrackingReference string `json:"trackingReference"`
	PolicyMode        string `json:"policyMode"`
}

// addFrontDoorWAFLogProperties parses the Front Door access log, and adds
// the relevant attributes to the record
func addFrontDoorWAFLogProperties(data []byte, record plog.LogRecord) error {
	var properties frontDoorWAFLogProperties
	if err := gojson.Unmarshal(data, &properties); err != nil {
		return fmt.Errorf("failed to parse AzureCdnAccessLog properties: %w", err)
	}

	if err := putInt(string(conventions.ClientPortKey), properties.ClientPort, record); err != nil {
		return err
	}

	if err := addRequestURIProperties(properties.RequestURI, record); err != nil {
		return fmt.Errorf(`failed to handle "requestUri" field: %w`, err)
	}

	putStr(string(conventions.ClientAddressKey), properties.ClientIP, record)
	putStr(string(conventions.SourceAddressKey), properties.SocketIP, record)
	putStr(attributeAzureRef, properties.TrackingReference, record)
	putStr("http.request.header.host", properties.Host, record)
	putStr(attributeAzureFrontDoorWAFPolicyName, properties.Policy, record)
	putStr(attributeAzureFrontDoorWAFPolicyMode, properties.PolicyMode, record)
	putStr(attributeAzureFrontDoorWAFRuleName, properties.RuleName, record)
	putStr(attributeAzureFrontDoorWAFAction, properties.Action, record)

	return nil
}

// addAppServiceAppLogsProperties parses the App Service access log, and adds
// the relevant attributes to the record
func addAppServiceAppLogsProperties(_ []byte, _ plog.LogRecord) error {
	// TODO @constanca-m implement this the same way as addAzureCdnAccessLogProperties
	return errStillToImplement
}

// addAppServiceAuditLogsProperties parses the App Service access log, and adds
// the relevant attributes to the record
func addAppServiceAuditLogsProperties(_ []byte, _ plog.LogRecord) error {
	// TODO @constanca-m implement this the same way as addAzureCdnAccessLogProperties
	return errStillToImplement
}

// addAppServiceAuthenticationLogsProperties parses the App Service access log, and adds
// the relevant attributes to the record
func addAppServiceAuthenticationLogsProperties(_ []byte, _ plog.LogRecord) error {
	// TODO @constanca-m implement this the same way as addAzureCdnAccessLogProperties
	return errStillToImplement
}

// addAppServiceConsoleLogsProperties parses the App Service access log, and adds
// the relevant attributes to the record
func addAppServiceConsoleLogsProperties(_ []byte, _ plog.LogRecord) error {
	// TODO @constanca-m implement this the same way as addAzureCdnAccessLogProperties
	return errStillToImplement
}

// addAppServiceHTTPLogsProperties parses the App Service access log, and adds
// the relevant attributes to the record
func addAppServiceHTTPLogsProperties(_ []byte, _ plog.LogRecord) error {
	// TODO @constanca-m implement this the same way as addAzureCdnAccessLogProperties
	return errStillToImplement
}

// addAppServiceIPSecAuditLogsProperties parses the App Service access log, and adds
// the relevant attributes to the record
func addAppServiceIPSecAuditLogsProperties(_ []byte, _ plog.LogRecord) error {
	// TODO @constanca-m implement this the same way as addAzureCdnAccessLogProperties
	return errStillToImplement
}

// addAppServicePlatformLogsProperties parses the App Service access log, and adds
// the relevant attributes to the record
func addAppServicePlatformLogsProperties(_ []byte, _ plog.LogRecord) error {
	// TODO @constanca-m implement this the same way as addAzureCdnAccessLogProperties
	return errStillToImplement
}
