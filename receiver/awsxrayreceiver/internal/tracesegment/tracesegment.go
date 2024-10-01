// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tracesegment // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsxrayreceiver/internal/tracesegment"

import (
	"strings"
)

// Header stores header of trace segment.
type Header struct {
	Format  string `json:"format"`
	Version int    `json:"version"`
}

// IsValid validates Header.
func (t Header) IsValid() bool {
	return strings.EqualFold(t.Format, "json") && t.Version == 1
}
