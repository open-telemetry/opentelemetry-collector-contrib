// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package notify // import "github.com/platformbuilds/opentelemetry-collector-contrib/processor/alertsprocessor/notify"

// AMAlert is a minimal Alertmanager-compatible payload item.
type AMAlert struct {
	Status       string            `json:"status"`                 // "firing" | "resolved"
	Labels       map[string]string `json:"labels"`                 // label set (includes rule labels + grouping attrs)
	Annotations  map[string]string `json:"annotations,omitempty"`  // optional
	StartsAt     string            `json:"startsAt"`               // RFC3339Nano
	EndsAt       string            `json:"endsAt,omitempty"`       // RFC3339Nano if resolved
	GeneratorURL string            `json:"generatorURL,omitempty"` // optional
}
