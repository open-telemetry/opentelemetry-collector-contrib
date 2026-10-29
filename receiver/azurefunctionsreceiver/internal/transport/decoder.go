// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package transport // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azurefunctionsreceiver/internal/transport"

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
)

// BinaryDecoder decodes base64-encoded message arrays (used by the Event Hub trigger)
type BinaryDecoder struct{}

func NewBinaryDecoder() *BinaryDecoder {
	return &BinaryDecoder{}
}

// Decode decodes the Event Hub trigger payload for a binding.
// The input is a JSON array of base64-encoded message bodies, e.g. ["<base64>", "<base64>"].
func (*BinaryDecoder) Decode(data string) ([][]byte, error) {
	var encoded []string
	if err := json.Unmarshal([]byte(data), &encoded); err != nil {
		return nil, fmt.Errorf("decode message array: %w", err)
	}
	out := make([][]byte, 0, len(encoded))
	for i, s := range encoded {
		decoded, err := base64.StdEncoding.DecodeString(s)
		if err != nil {
			return nil, fmt.Errorf("decode message %d: %w", i, err)
		}
		out = append(out, decoded)
	}
	return out, nil
}
