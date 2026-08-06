// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/gowebpki/jcs"
)

// LogRecord mirrors the JSON shape produced by the collector's debug exporter
// and the single-record format accepted by verify-signed-log.sh.
type LogRecord struct {
	EventName           *string                `json:"event_name,omitempty"`
	Body                interface{}            `json:"body,omitempty"`
	Attributes          map[string]interface{} `json:"attributes,omitempty"`
	Timestamp           *int64                 `json:"timestamp,omitempty"`
	ObservedTimestamp   *int64                 `json:"observed_timestamp,omitempty"`
	SeverityNumber      *int                   `json:"severity_number,omitempty"`
	SeverityText        *string                `json:"severity_text,omitempty"`
	TraceID             *string                `json:"trace_id,omitempty"`
	SpanID              *string                `json:"span_id,omitempty"`
}

func main() {
	var (
		logFile  = flag.String("log", "", "Path to log file (JSON format) or '-' for stdin")
		certFile = flag.String("cert", "", "Path to certificate file (cert.pem)")
		hashAlgo = flag.String("hash", "SHA256", "Hash algorithm (SHA256 or SHA512)")
		verbose  = flag.Bool("verbose", false, "Show detailed output")
	)
	flag.Parse()

	if *logFile == "" {
		fmt.Fprintf(os.Stderr, "Error: -log flag is required\n")
		flag.Usage()
		os.Exit(1)
	}

	if *certFile == "" {
		fmt.Fprintf(os.Stderr, "Error: -cert flag is required\n")
		flag.Usage()
		os.Exit(1)
	}

	var hashID crypto.Hash
	switch strings.ToUpper(*hashAlgo) {
	case "SHA256":
		hashID = crypto.SHA256
	case "SHA512":
		hashID = crypto.SHA512
	default:
		fmt.Fprintf(os.Stderr, "Error: hash algorithm must be SHA256 or SHA512\n")
		os.Exit(1)
	}

	var logReader io.Reader
	if *logFile == "-" {
		logReader = os.Stdin
	} else {
		file, err := os.Open(*logFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error opening log file: %v\n", err)
			os.Exit(1)
		}
		defer file.Close()
		logReader = file
	}

	logData, err := io.ReadAll(logReader)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading log data: %v\n", err)
		os.Exit(1)
	}

	logRecords, err := parseLogRecords(logData)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if len(logRecords) == 0 {
		fmt.Fprintf(os.Stderr, "Error: No log records found\n")
		os.Exit(1)
	}

	certPEM, err := os.ReadFile(*certFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading certificate file: %v\n", err)
		os.Exit(1)
	}

	block, _ := pem.Decode(certPEM)
	if block == nil {
		fmt.Fprintf(os.Stderr, "Error: Failed to decode PEM certificate\n")
		os.Exit(1)
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing certificate: %v\n", err)
		os.Exit(1)
	}

	publicKey, ok := cert.PublicKey.(*rsa.PublicKey)
	if !ok {
		fmt.Fprintf(os.Stderr, "Error: Certificate does not contain RSA public key\n")
		os.Exit(1)
	}

	allValid := true
	for i, record := range logRecords {
		if *verbose {
			fmt.Printf("\n=== Verifying Log Record %d ===\n", i+1)
		}

		signatureValue, ok := record.Attributes["audit.integrity.value"].(string)
		if !ok || signatureValue == "" {
			fmt.Fprintf(os.Stderr, "❌ Log record %d: missing audit.integrity.value attribute\n", i+1)
			allValid = false
			continue
		}

		serialized, err := serializeLogRecord(record)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error serializing log record %d: %v\n", i+1, err)
			allValid = false
			continue
		}

		if *verbose {
			fmt.Printf("Serialized log data: %s\n", string(serialized))
		}

		var computedHash []byte
		switch hashID {
		case crypto.SHA256:
			h := sha256.Sum256(serialized)
			computedHash = h[:]
		case crypto.SHA512:
			h := sha512.Sum512(serialized)
			computedHash = h[:]
		}

		if *verbose {
			fmt.Printf("Computed hash: %s\n", base64.StdEncoding.EncodeToString(computedHash))
		}

		signatureBytes, err := base64.StdEncoding.DecodeString(signatureValue)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error decoding signature for record %d: %v\n", i+1, err)
			allValid = false
			continue
		}

		if err := rsa.VerifyPKCS1v15(publicKey, hashID, computedHash, signatureBytes); err != nil {
			fmt.Printf("❌ Log record %d: Signature verification failed: %v\n", i+1, err)
			allValid = false
			continue
		}

		fmt.Printf("✅ Log record %d: Hash and signature verified successfully\n", i+1)
	}

	if allValid {
		fmt.Printf("\n✅ All log records verified successfully!\n")
		os.Exit(0)
	} else {
		fmt.Printf("\n❌ Some log records failed verification\n")
		os.Exit(1)
	}
}

// serializeLogRecord reconstructs the exact JSON payload that the processor
// hashes and signs: all log-record fields and all attributes whose key does
// NOT start with "audit.integrity.".
func serializeLogRecord(record LogRecord) ([]byte, error) {
	data := make(map[string]interface{})

	if record.EventName != nil && *record.EventName != "" {
		data["event_name"] = *record.EventName
	}

	if record.Body != nil {
		if bodyStr, ok := record.Body.(string); ok {
			data["body"] = bodyStr
		}
	}

	if record.Timestamp != nil && *record.Timestamp != 0 {
		data["timestamp"] = *record.Timestamp
	}

	if record.ObservedTimestamp != nil && *record.ObservedTimestamp != 0 {
		data["observed_timestamp"] = *record.ObservedTimestamp
	}

	if record.SeverityNumber != nil && *record.SeverityNumber != 0 {
		data["severity_number"] = *record.SeverityNumber
	}

	if record.SeverityText != nil && *record.SeverityText != "" {
		data["severity_text"] = *record.SeverityText
	}

	if record.TraceID != nil && *record.TraceID != "" {
		data["trace_id"] = *record.TraceID
	}

	if record.SpanID != nil && *record.SpanID != "" {
		data["span_id"] = *record.SpanID
	}

	attrs := make(map[string]interface{})
	for k, v := range record.Attributes {
		if !strings.HasPrefix(k, "audit.integrity.") {
			attrs[k] = v
		}
	}
	if len(attrs) > 0 {
		data["attributes"] = attrs
	}

	// json.Marshal sorts map keys (Go ≥ 1.12); jcs.Transform normalises number
	// representation per RFC 8785 §3.2.2 and re-sorts keys on UTF-16 code units.
	b, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	return jcs.Transform(b)
}

// parseLogRecords handles both OTLP (resourceLogs) and single-record JSON.
func parseLogRecords(raw []byte) ([]LogRecord, error) {
	var top map[string]interface{}
	if err := json.Unmarshal(raw, &top); err != nil {
		return nil, fmt.Errorf("parsing log JSON: %w", err)
	}

	if _, hasResourceLogs := top["resourceLogs"]; hasResourceLogs {
		var records []LogRecord
		resourceLogs, _ := top["resourceLogs"].([]interface{})
		for _, rl := range resourceLogs {
			rlMap, _ := rl.(map[string]interface{})
			scopeLogs, _ := rlMap["scopeLogs"].([]interface{})
			for _, sl := range scopeLogs {
				slMap, _ := sl.(map[string]interface{})
				lrs, _ := slMap["logRecords"].([]interface{})
				for _, lr := range lrs {
					b, err := json.Marshal(lr)
					if err != nil {
						continue
					}
					var record LogRecord
					if err := json.Unmarshal(b, &record); err == nil {
						records = append(records, record)
					}
				}
			}
		}
		return records, nil
	}

	// Single record
	var record LogRecord
	if err := json.Unmarshal(raw, &record); err != nil {
		return nil, fmt.Errorf("could not parse log data as OTLP format or single record")
	}
	return []LogRecord{record}, nil
}
