// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elbaccesslogs // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/unmarshaler/elb-access-log"

import (
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"
)

const (
	albAccessLogs = "alb_access_logs"
	nlbAccessLogs = "nlb_access_logs"
	clbAccessLogs = "clb_access_logs"
	// any field can be set to - to indicate that the data was unknown
	// or unavailable, or that the field was not applicable to this request.
	unknownField = "-"
	// First field of ELB control message
	EnableControlMessage = "Enable"
)

// CLBAccessLogRecord represents a record of access logs from an AWS Classic Load Balancer.
// Documentation Reference: https://docs.aws.amazon.com/elasticloadbalancing/latest/classic/access-log-collection.html
type CLBAccessLogRecord struct {
	Time                   string  // Timestamp the load balancer received the request from the client, in ISO 8601 format
	ELB                    string  // The name of the load balancer
	ClientIP               string  // Client IP
	ClientPort             int64   // Client  port
	BackendIPPort          string  // Backend IP:Port or -
	BackendIP              string  // Backend IP split from BackendIPPort
	BackendPort            int64   // Backend port split from BackendIPPort
	RequestProcessingTime  float64 // Time taken to process the request in seconds (HTTP/TCP)
	BackendProcessingTime  float64 // Time taken for the registered instance to respond
	ResponseProcessingTime float64 // Time taken to send the response to the client
	ELBStatusCode          int64   // Status code from the load balancer
	BackendStatusCode      int64   // Status code from the backend instance
	ReceivedBytes          int64   // Size of the request in bytes received from the client
	SentBytes              int64   // Size of the response in bytes sent to the client
	RequestMethod          string  // HTTP method used in the request (e.g., GET, POST)
	RequestURI             string  // Full URI requested by the client
	ProtocolName           string  // Network protocol name (e.g., HTTP)
	ProtocolVersion        string  // Network protocol version (e.g., 1.1)
	UserAgent              string  // The User-Agent string identifying the client
	SSLCipher              string  // The SSL cipher used, available for HTTPS listeners
	SSLProtocol            string  // The SSL protocol negotiated, available for HTTPS listeners
}

// convertTextToCLBAccessLogRecord converts a slice of strings into a CLBAccessLogRecord
func convertTextToCLBAccessLogRecord(fields []string) (CLBAccessLogRecord, error) {
	var err error
	fieldsCount := len(fields)
	if fieldsCount < 15 {
		return CLBAccessLogRecord{}, fmt.Errorf("clb access logs do not have enough fields. Expected 15, got %d", fieldsCount)
	}

	// Map fields to the struct
	record := CLBAccessLogRecord{
		Time:              fields[0],  // Timestamp
		ELB:               fields[1],  // Load balancer name
		BackendIPPort:     fields[3],  // Backend IP:Port or -
		ELBStatusCode:     0,          // Placeholder for ELB status code
		BackendStatusCode: 0,          // Placeholder for Backend status code
		UserAgent:         fields[12], // User-Agent
		SSLCipher:         fields[13], // SSL cipher
		SSLProtocol:       fields[14], // SSL protocol
	}

	// Process the fields for numerical values (convenient to parse from string)
	var clientPort string
	if record.ClientIP, clientPort, err = net.SplitHostPort(fields[2]); err != nil {
		return record, fmt.Errorf("could not parse client IP:Port %s: %w", fields[2], err)
	}
	if record.ClientPort, err = safeConvertStrToInt(clientPort); err != nil {
		return record, fmt.Errorf("could not convert client port to integer: %w", err)
	}

	// Parse BackendIPPort into BackendIP and BackendPort
	if record.BackendIPPort != unknownField {
		var backendPort string
		if record.BackendIP, backendPort, err = net.SplitHostPort(record.BackendIPPort); err != nil {
			return record, fmt.Errorf("could not parse backend IP:Port %s: %w", record.BackendIPPort, err)
		}
		if record.BackendPort, err = safeConvertStrToInt(backendPort); err != nil {
			return record, fmt.Errorf("could not convert backend port to integer: %w", err)
		}
	}

	if record.RequestProcessingTime, err = safeConvertStrToFloat(fields[4]); err != nil {
		return record, fmt.Errorf("could not convert request processing time to float: %w", err)
	}

	if record.BackendProcessingTime, err = safeConvertStrToFloat(fields[5]); err != nil {
		return record, fmt.Errorf("could not convert backend processing time to float: %w", err)
	}

	if record.ResponseProcessingTime, err = safeConvertStrToFloat(fields[6]); err != nil {
		return record, fmt.Errorf("could not convert response processing time to float: %w", err)
	}

	// ELB status code and backend status code can be - in case of TCP and SSL entry
	if fields[7] != unknownField {
		if record.ELBStatusCode, err = safeConvertStrToInt(fields[7]); err != nil {
			return record, fmt.Errorf("could not convert ELB status code to integer: %w", err)
		}
	}
	if fields[8] != unknownField {
		if record.BackendStatusCode, err = safeConvertStrToInt(fields[8]); err != nil {
			return record, fmt.Errorf("could not convert backend status code to integer: %w", err)
		}
	}

	if record.ReceivedBytes, err = safeConvertStrToInt(fields[9]); err != nil {
		return record, fmt.Errorf("could not convert received bytes to integer: %w", err)
	}

	if record.SentBytes, err = safeConvertStrToInt(fields[10]); err != nil {
		return record, fmt.Errorf("could not convert sent bytes to integer: %w", err)
	}

	if record.RequestMethod, record.RequestURI, record.ProtocolName, record.ProtocolVersion, err = parseRequestField(fields[11]); err != nil {
		return record, fmt.Errorf("could not split a raw HTTP request line into its components: %w", err)
	}

	return record, nil
}

// Network Load Balancer Access Logs record
// Doc: https://docs.aws.amazon.com/elasticloadbalancing/latest//network/load-balancer-access-logs.html#access-log-entry-format
type NLBAccessLogRecord struct {
	Type                      string // Type of request (tls)
	Version                   string // Version of the log entry
	Time                      string // Timestamp the load balancer generated a response to the client in ISO 8601 format
	ELB                       string // Load balancer resource ID
	Listener                  string // Resource ID of the TLS listener for the connection
	ClientIP                  string // Client IP
	ClientPort                int64  // Client  port
	DestinationIP             string // Destination IP
	DestinationPort           int64  // Destination port
	ConnectionTime            int64  // Total time for the connection to complete, in milliseconds
	TLSHandshakeTime          int64  // Time for the TLS handshake to complete, in milliseconds, or -
	ReceivedBytes             int64  // Count of bytes received by the load balancer from the client, after decryption
	SentBytes                 int64  // Count of bytes sent by the load balancer to the client, before encryption
	IncomingTLSAlert          string // TLS alerts received from the client, if present, or -
	ChosenCertARN             string // ARN of the certificate served to the client, or -
	ChosenCertSerial          string // Reserved for future use, always set to -
	TLSCipher                 string // The cipher suite negotiated with the client, or -
	TLSProtocolVersion        string // The TLS protocol negotiated with the client, or -
	TLSNamedGroup             string // Reserved for future use, always set to -
	DomainName                string // Server name extension in the client hello message, or -
	ALPNFeProtocol            string // Application protocol negotiated with the client, or -
	ALPNBeProtocol            string // Application protocol negotiated with the target, or -
	ALPNClientPreferenceList  string // Client hello message ALPN list, URL-encoded, or -
	TLSConnectionCreationTime string // Time recorded at the start of the TLS connection, in ISO 8601 format
}

// convertTextToNLBAccessLogRecord converts a slice of strings into a NLBAccessLogRecord
func convertTextToNLBAccessLogRecord(fields []string) (NLBAccessLogRecord, error) {
	var err error
	// Check if the fields contain enough data
	fieldsCount := len(fields)
	if fieldsCount < 22 {
		return NLBAccessLogRecord{}, fmt.Errorf(
			"nlb access logs do not have enough fields. Expected 22, got %d", fieldsCount)
	}

	// Map fields to the struct
	record := NLBAccessLogRecord{
		Type:                      fields[0],  // Type of request
		Version:                   fields[1],  // Log version
		Time:                      fields[2],  // Timestamp
		ELB:                       fields[3],  // Load balancer resource ID
		Listener:                  fields[4],  // Listener ID
		TLSHandshakeTime:          0,          // TLSHandshakeTime placeholder value
		IncomingTLSAlert:          fields[11], // Incoming TLS alert
		ChosenCertARN:             fields[12], // Chosen certificate ARN
		ChosenCertSerial:          fields[13], // Reserved for future use, usually set to '-'
		TLSCipher:                 fields[14], // Negotiated TLS cipher
		TLSProtocolVersion:        fields[15], // Negotiated TLS protocol version
		TLSNamedGroup:             fields[16], // Reserved for future use, usually set to '-'
		DomainName:                fields[17], // SNI domain provided by the client
		ALPNFeProtocol:            fields[18], // Protocol negotiated with client
		ALPNBeProtocol:            fields[19], // Protocol negotiated with target
		ALPNClientPreferenceList:  fields[20], // ALPN preference list from the client
		TLSConnectionCreationTime: fields[21], // Time of the start of the TLS connection
	}

	// Processing additional fields if applicable
	var clientPort string
	if record.ClientIP, clientPort, err = net.SplitHostPort(fields[5]); err != nil {
		return record, fmt.Errorf("could not parse client IP:Port %s: %w", fields[5], err)
	}
	if record.ClientPort, err = safeConvertStrToInt(clientPort); err != nil {
		return record, fmt.Errorf("could not convert client port to integer: %w", err)
	}

	var destinationPort string
	if record.DestinationIP, destinationPort, err = net.SplitHostPort(fields[6]); err != nil {
		return record, fmt.Errorf("could not parse destination IP:Port %s: %w", fields[6], err)
	}
	if record.DestinationPort, err = safeConvertStrToInt(destinationPort); err != nil {
		return record, fmt.Errorf("could not convert destination port to integer: %w", err)
	}

	if record.ConnectionTime, err = safeConvertStrToInt(fields[7]); err != nil {
		return record, fmt.Errorf("could not convert connection time to integer: %w", err)
	}
	if fields[8] != unknownField {
		if record.TLSHandshakeTime, err = safeConvertStrToInt(fields[8]); err != nil {
			return record, fmt.Errorf("could not convert TLS handshake time to integer: %w", err)
		}
	}

	if record.ReceivedBytes, err = safeConvertStrToInt(fields[9]); err != nil {
		return record, fmt.Errorf("could not convert received bytes to integer: %w", err)
	}

	if record.SentBytes, err = safeConvertStrToInt(fields[10]); err != nil {
		return record, fmt.Errorf("could not convert sent bytes to integer: %w", err)
	}

	return record, nil
}

// Application Load Balancer Access Logs record
// Doc: https://docs.aws.amazon.com/elasticloadbalancing/latest/application/load-balancer-access-logs.html
type ALBAccessLogRecord struct {
	Type                   string // Type of request (http, https, etc.)
	Time                   string // Timestamp the load balancer generated a response to the client in ISO 8601 format
	ELB                    string // Load balancer resource ID
	ClientIP               string // Client IP
	ClientPort             int64  // Client  port
	TargetIPPort           string // Target IP:Port or -
	TargetIP               string // Target IP
	TargetPort             int64  // Target port
	RequestProcessingTime  string // Time taken to process the request in seconds
	TargetProcessingTime   string // Time taken for the target to process the request in seconds
	ResponseProcessingTime string // Time taken to send the response to the client in seconds
	ELBStatusCode          int64  // Status code from the load balancer
	TargetStatusCode       string // Status code from the target
	ReceivedBytes          int64  // Size of the request in bytes
	SentBytes              int64  // Size of the response in bytes
	RequestMethod          string // HTTP method used in the request (e.g., GET, POST)
	RequestURI             string // Full URI requested by the client
	ProtocolName           string // Network protocol name (e.g., HTTP)
	ProtocolVersion        string // Network protocol version (e.g., 1.1)
	UserAgent              string // Client's User-Agent string
	SSLCipher              string // SSL cipher
	SSLProtocol            string // SSL protocol
	TargetGroupARN         string // Target group ARN
	TraceID                string // X-Amzn-Trace-Id header contents
	DomainName             string // SNI domain provided by the client
	ChosenCertARN          string // ARN of the chosen certificate
	MatchedRulePriority    string // Priority of the rule that matched
	RequestCreationTime    string // Time when the load balancer received the request from the client in ISO 8601 format
	ActionsExecuted        string // Actions executed, comma-separated
	RedirectURL            string // URL of the redirect target
	ErrorReason            string // Reason for request failure
	TargetPortList         string // List of target IPs/ports (if applicable)
	TargetStatusCodeList   string // List of status codes from targets
	Classification         string // Classification of the request
	ClassificationReason   string // Reason for classification
}

// convertTextToALBAccessLogRecord converts a slice of strings into a ALBAccessLogRecord
func convertTextToALBAccessLogRecord(fields []string) (ALBAccessLogRecord, error) {
	var err error
	fieldsCount := len(fields)
	if fieldsCount < 29 {
		return ALBAccessLogRecord{}, fmt.Errorf("alb access logs do not have enough fields. Expected 29, got %d", fieldsCount)
	}
	// Map fields to the struct
	record := ALBAccessLogRecord{
		Type:                   fields[0],
		Time:                   fields[1],
		ELB:                    fields[2],
		TargetIPPort:           fields[4],
		RequestProcessingTime:  fields[5],
		TargetProcessingTime:   fields[6],
		ResponseProcessingTime: fields[7],
		TargetStatusCode:       fields[9],
		UserAgent:              fields[13],
		SSLCipher:              fields[14],
		SSLProtocol:            fields[15],
		TargetGroupARN:         fields[16],
		TraceID:                fields[17],
		DomainName:             fields[18],
		ChosenCertARN:          fields[19],
		MatchedRulePriority:    fields[20],
		RequestCreationTime:    fields[21],
		ActionsExecuted:        fields[22],
		RedirectURL:            fields[23],
		ErrorReason:            fields[24],
		TargetPortList:         fields[25],
		TargetStatusCodeList:   fields[26],
		Classification:         fields[27],
		ClassificationReason:   fields[28],
	}
	var clientPort string
	if record.ClientIP, clientPort, err = net.SplitHostPort(fields[3]); err != nil {
		return record, fmt.Errorf("could not parse client IP:Port %s: %w", fields[3], err)
	}
	if record.ClientPort, err = safeConvertStrToInt(clientPort); err != nil {
		return record, fmt.Errorf("could not convert client port to integer: %w", err)
	}

	// Parse TargetIPPort into TargetIP and TargetPort
	if record.TargetIPPort != unknownField {
		var targetPort string
		if record.TargetIP, targetPort, err = net.SplitHostPort(record.TargetIPPort); err != nil {
			return record, fmt.Errorf("could not parse target IP:Port %s: %w", record.TargetIPPort, err)
		}
		if record.TargetPort, err = safeConvertStrToInt(targetPort); err != nil {
			return record, fmt.Errorf("could not convert target port to integer: %w", err)
		}
	}

	if record.ELBStatusCode, err = safeConvertStrToInt(fields[8]); err != nil {
		return record, fmt.Errorf("could not convert elb status code to integer: %w", err)
	}
	if record.ReceivedBytes, err = safeConvertStrToInt(fields[10]); err != nil {
		return record, fmt.Errorf("could not convert received bytes to integer: %w", err)
	}
	if record.SentBytes, err = safeConvertStrToInt(fields[11]); err != nil {
		return record, fmt.Errorf("could not convert sent bytes to integer: %w", err)
	}

	if record.RequestMethod, record.RequestURI, record.ProtocolName, record.ProtocolVersion, err = parseRequestField(fields[12]); err != nil {
		return record, fmt.Errorf("could not splits a raw HTTP request line into its components: %w", err)
	}

	return record, nil
}

func safeConvertStrToInt(stringNum string) (int64, error) {
	num, err := strconv.Atoi(stringNum)
	if err != nil {
		return 0, err
	}
	return int64(num), nil
}

func safeConvertStrToFloat(stringNum string) (float64, error) {
	value, err := strconv.ParseFloat(stringNum, 64)
	if err != nil {
		return 0, err
	}
	return value, nil
}

// findLogSyntaxByField determines the log syntax type based on the first field of a log entry.
// It first checks for known protocol types (ALB and NLB).
// ALB supports http, https, h2, grpcs, ws, wss and NLB supports tls.
// Only if those are not matched, it checks if the field is a valid timestamp (for CLB logs).
// If none match, it returns an error.
func findLogSyntaxByField(field string) (string, error) {
	switch field {
	case "http", "https", "h2", "grpcs", "ws", "wss":
		return albAccessLogs, nil
	case "tls":
		return nlbAccessLogs, nil
	default:
		if isValidTimestamp(field) {
			return clbAccessLogs, nil
		}
		return "", fmt.Errorf("invalid type: %v", field)
	}
}

// isValidTimestamp checks if the given field is a valid timestamp
func isValidTimestamp(field string) bool {
	_, err := time.Parse("2006-01-02T15:04:05.999999Z", field)
	return err == nil
}

// convertToUnixEpoch converts ISO 8601 timestamps to UNIX Epoch time in nanoseconds
func convertToUnixEpoch(isoTimestamp string) (int64, error) {
	var t time.Time
	var err error

	// Check if the timestamp has sub-second precision
	if len(isoTimestamp) > 19 && isoTimestamp[19] == '.' {
		// Parse complete ISO 8601 timestamp with microseconds including Zone
		t, err = time.Parse(time.RFC3339Nano, isoTimestamp)
		//  Parse complete ISO 8601 timestamp with microseconds without Zone
		if err != nil {
			t, err = time.Parse("2006-01-02T15:04:05.999999999", isoTimestamp)
		}
	} else {
		// Parse timestamp without sub-second precision
		t, err = time.Parse("2006-01-02T15:04:05", isoTimestamp)
	}

	if err != nil {
		return 0, err
	}

	// Return the UNIX epoch time in nanoseconds
	return t.UnixNano(), nil
}

// scanField gets the next value in the log line by moving
// one space. If the value starts with a quote, it moves
// forward until it finds the ending quote, and considers
// that just 1 value. Otherwise, it returns the value as
// it is.
func scanField(logLine string) (string, string, error) {
	if logLine == "" {
		return "", "", io.EOF
	}

	if logLine[0] != '"' {
		value, remaining, _ := strings.Cut(logLine, " ")
		return value, remaining, nil
	}

	// if there is a quote, we need to get the rest of the value
	logLine = logLine[1:] // remove first quote
	value, remaining, found := strings.Cut(logLine, `"`)
	if !found {
		return "", "", fmt.Errorf("value %q has no end quote", logLine)
	}

	// Remove space after closing quote if present
	if remaining != "" && remaining[0] == ' ' {
		remaining = remaining[1:]
	}

	return value, remaining, nil
}

func extractFields(logLine string) ([]string, error) {
	var fields []string
	var value string
	var err error

	for logLine != "" {
		value, logLine, err = scanField(logLine)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}
		fields = append(fields, value)
	}
	return fields, nil
}

// parseRequestField splits a raw HTTP request line into its components:
// method, URI, protocol name, and protocol version.
// Expected format: "<METHOD> <URI> <PROTOCOL>/<VERSION>", e.g. "GET http://example.com HTTP/1.1".
func parseRequestField(raw string) (method, uri, protoName, protoVersion string, err error) {
	method, remaining, _ := strings.Cut(raw, " ")
	if method == "" {
		err = fmt.Errorf("unexpected: request field %q has no method", raw)
		return
	}

	uri, remaining, _ = strings.Cut(remaining, " ")
	if uri == "" {
		err = fmt.Errorf("unexpected: request field %q has no URI", raw)
		return
	}

	protocol, leftover, _ := strings.Cut(remaining, " ")
	if protocol == "" || leftover != "" {
		err = fmt.Errorf(`request field %q does not match expected format "<method> <uri> <protocol>"`, raw)
		return
	}

	protoName, protoVersion, err = netProtocol(protocol)
	if err != nil {
		err = fmt.Errorf("invalid protocol in request field: %w", err)
		return
	}
	return
}

// netProtocol returns protocol name and version based on proto value
func netProtocol(proto string) (string, string, error) {
	name, version, found := strings.Cut(proto, "/")
	if !found || name == "" || version == "" {
		return "", "", errors.New(`request uri protocol does not follow expected scheme "<name>/<version>"`)
	}

	return strings.ToLower(name), version, nil
}
