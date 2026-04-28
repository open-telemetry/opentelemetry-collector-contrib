// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package protocol // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azurefunctionsreceiver/internal/protocol"

import "encoding/json"

// InvokeRequest is the request payload from the Azure Functions host.
// See: https://learn.microsoft.com/en-us/azure/azure-functions/functions-custom-handlers#request-payload
//
// Metadata is left as raw JSON so trigger-specific handlers (e.g. Event Hub, HTTP)
// can unmarshal it into their own types. See internal/eventhub for Event Hub metadata.
type InvokeRequest struct {
	// Data maps binding names (e.g. "logs", "raw_logs") to payload strings.
	// Format of each value is trigger-specific (e.g. Event Hub uses a JSON array of base64-encoded messages).
	Data     map[string]string `json:"Data"`
	Metadata json.RawMessage   `json:"Metadata"`
}

// InvokeResponse is the response payload to the Azure Functions host.
// See: https://learn.microsoft.com/en-us/azure/azure-functions/functions-custom-handlers#response-payload
type InvokeResponse struct {
	Outputs     *Outputs `json:"outputs,omitempty"`
	Logs        []string `json:"logs,omitempty"`
	ReturnValue string   `json:"returnValue"`
}

// Outputs carries optional output bindings; we use failedMessage for errors.
type Outputs struct {
	FailedMessage FailedMessage `json:"failedMessage"`
}

// FailedMessage is sent in Outputs when processing fails.
type FailedMessage struct {
	Error  string `json:"error"`
	Source []byte `json:"source"`
}
