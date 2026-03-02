// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package response // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/libhoneyreceiver/internal/response"

import "net/http"

type ResponseInBatch struct {
	ErrorStr string `json:"error,omitempty"`
	Status   int    `json:"status,omitempty"`
}

func MakeResponse(eventErrs []int) []ResponseInBatch {
	if len(eventErrs) == 0 {
		return []ResponseInBatch{{Status: http.StatusAccepted}}
	}

	responses := make([]ResponseInBatch, len(eventErrs))
	for i, eventErr := range eventErrs {
		responses[i] = ResponseInBatch{
			ErrorStr: "error",
			Status:   eventErr,
		}
	}
	return responses
}

// MakeSuccessResponse creates a response array with all events marked as accepted
func MakeSuccessResponse(numEvents int) []ResponseInBatch {
	if numEvents <= 0 {
		return []ResponseInBatch{{Status: http.StatusAccepted}}
	}

	responses := make([]ResponseInBatch, numEvents)
	for i := range numEvents {
		responses[i] = ResponseInBatch{Status: http.StatusAccepted}
	}
	return responses
}
