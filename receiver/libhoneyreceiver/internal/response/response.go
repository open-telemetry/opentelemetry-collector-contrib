// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package response // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/libhoneyreceiver/internal/response"

import "net/http"

type ResponseInBatch struct {
	ErrorStr string `json:"error,omitempty"`
	Status   int    `json:"status,omitempty"`
}

func MakeResponse(eventErrs []int) []ResponseInBatch {
	responses := make([]ResponseInBatch, len(eventErrs))

	if len(eventErrs) == 0 {
		responses = append(responses, ResponseInBatch{Status: http.StatusAccepted})
		return responses
	}

	for i, eventErr := range eventErrs {
		responses[i] = ResponseInBatch{
			ErrorStr: "error",
			Status:   eventErr,
		}
	}
	return responses
}
