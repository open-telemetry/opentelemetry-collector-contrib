// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package protocol // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azurefunctionsreceiver/internal/protocol"

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

// ParseInvokeRequest parses the HTTP request body as an Azure Functions invoke request.
func ParseInvokeRequest(body []byte) (InvokeRequest, error) {
	var req InvokeRequest
	if err := json.NewDecoder(bytes.NewReader(body)).Decode(&req); err != nil {
		return InvokeRequest{}, fmt.Errorf("decode invoke request: %w", err)
	}
	return req, nil
}

// WriteSuccess writes a success response to the Azure Functions host.
func WriteSuccess(w http.ResponseWriter) {
	writeResponse(w, InvokeResponse{ReturnValue: "success"})
}

// WriteFailure writes a failure response with the given error and original request body.
func WriteFailure(w http.ResponseWriter, err error, body []byte) {
	writeResponse(w, InvokeResponse{
		ReturnValue: "failure",
		Outputs: &Outputs{
			FailedMessage: FailedMessage{
				Error:  err.Error(),
				Source: body,
			},
		},
	})
}

func writeResponse(w http.ResponseWriter, resp InvokeResponse) {
	data, err := json.Marshal(resp)
	if err != nil {
		http.Error(w, fmt.Sprintf("marshal response: %v", err), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(data) // headers are flushed on Write; nothing useful we can do on error
}
