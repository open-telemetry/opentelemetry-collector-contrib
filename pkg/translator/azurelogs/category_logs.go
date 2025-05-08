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
	conventions "go.opentelemetry.io/collector/semconv/v1.27.0"
)

const (
	noError = "NoError"

	categoryAzureCdnAccessLog                  = "AzureCdnAccessLog"
	categoryFrontDoorAccessLog                 = "FrontDoorAccessLog"
	categoryFrontDoorHealthProbeLog            = "FrontDoorHealthProbeLog"
	categoryFrontdoorWebApplicationFirewallLog = "FrontdoorWebApplicationFirewallLog"
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
	record.Attributes().PutStr(conventions.AttributeURLOriginal, uri)

	if port := u.Port(); port != "" {
		if err := putInt(conventions.AttributeURLPort, u.Port(), record); err != nil {
			return fmt.Errorf("failed to get port number from value %q: %w", port, err)
		}
	}

	putStr(conventions.AttributeURLScheme, u.Scheme, record)
	putStr(conventions.AttributeURLPath, u.Path, record)
	putStr(conventions.AttributeURLQuery, u.RawQuery, record)
	putStr(conventions.AttributeURLFragment, u.Fragment, record)

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

	record.Attributes().PutStr(conventions.AttributeTLSProtocolName, name)
	record.Attributes().PutStr(conventions.AttributeTLSProtocolVersion, version)

	return nil
}

// addErrorInfoProperties checks if there is an error and adds it
// to the record attributes as an exception in case it exists.
func addErrorInfoProperties(errorInfo string, record plog.LogRecord) {
	if errorInfo == noError {
		return
	}
	record.Attributes().PutStr(conventions.AttributeExceptionType, errorInfo)
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
		err := addFields(endpoint, conventions.AttributeDestinationAddress, conventions.AttributeDestinationPort)
		if err != nil {
			return fmt.Errorf("failed to parse endpoint %q: %w", endpoint, err)
		}
	} else {
		err := addFields(backendHostname, conventions.AttributeDestinationAddress, conventions.AttributeDestinationPort)
		if err != nil {
			return fmt.Errorf("failed to parse backend hostname %q: %w", backendHostname, err)
		}

		if endpoint != backendHostname && endpoint != "" {
			err = addFields(endpoint, conventions.AttributeNetworkPeerAddress, conventions.AttributeNetworkPeerPort)
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

	if err := putInt(conventions.AttributeHTTPRequestSize, properties.RequestBytes, record); err != nil {
		return err
	}
	if err := putInt(conventions.AttributeHTTPResponseSize, properties.RequestBytes, record); err != nil {
		return err
	}
	if err := putInt(conventions.AttributeClientPort, properties.ClientPort, record); err != nil {
		return err
	}
	if err := putInt(conventions.AttributeHTTPResponseStatusCode, properties.HTTPStatusCode, record); err != nil {
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
	putStr(conventions.AttributeHTTPRequestMethod, properties.HTTPMethod, record)
	putStr(conventions.AttributeNetworkProtocolVersion, properties.HTTPVersion, record)
	putStr(conventions.AttributeNetworkProtocolName, properties.RequestProtocol, record)
	putStr(attributeTLSServerName, properties.SNI, record)
	putStr(conventions.AttributeUserAgentOriginal, properties.UserAgent, record)
	putStr(conventions.AttributeClientAddress, properties.ClientIP, record)
	putStr(conventions.AttributeSourceAddress, properties.SocketIP, record)

	putStr(attributeAzurePop, properties.Pop, record)
	putStr(attributeCacheStatus, properties.CacheStatus, record)

	if properties.IsReceivedFromClient {
		record.Attributes().PutStr(conventions.AttributeNetworkIoDirection, "receive")
	} else {
		record.Attributes().PutStr(conventions.AttributeNetworkIoDirection, "transmit")
	}

	return nil
}

// addFrontDoorAccessLogProperties parses the Front Door access log, and adds
// the relevant attributes to the record
func addFrontDoorAccessLogProperties(_ []byte, _ plog.LogRecord) error {
	// TODO @constanca-m implement this the same way as addAzureCdnAccessLogProperties
	return errStillToImplement
}

// addFrontDoorHealthProbeLogProperties parses the Front Door access log, and adds
// the relevant attributes to the record
func addFrontDoorHealthProbeLogProperties(_ []byte, _ plog.LogRecord) error {
	// TODO @constanca-m implement this the same way as addAzureCdnAccessLogProperties
	return errStillToImplement
}

// addFrontDoorWAFLogProperties parses the Front Door access log, and adds
// the relevant attributes to the record
func addFrontDoorWAFLogProperties(_ []byte, _ plog.LogRecord) error {
	// TODO @constanca-m implement this the same way as addAzureCdnAccessLogProperties
	return errStillToImplement
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
