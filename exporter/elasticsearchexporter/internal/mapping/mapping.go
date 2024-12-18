// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mapping // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/mapping"

import "strings"

type Mode int

// Enum values for Mode.
const (
	ModeNone Mode = iota
	ModeECS
	ModeOTel
	ModeRaw
	ModeBodyMap
)

func (m Mode) String() string {
	switch m {
	case ModeNone:
		return ""
	case ModeECS:
		return "ecs"
	case ModeOTel:
		return "otel"
	case ModeRaw:
		return "raw"
	case ModeBodyMap:
		return "bodymap"
	default:
		return ""
	}
}

var allModes = func() map[string]Mode {
	table := map[string]Mode{}
	for _, m := range []Mode{
		ModeNone,
		ModeECS,
		ModeOTel,
		ModeRaw,
		ModeBodyMap,
	} {
		table[strings.ToLower(m.String())] = m
	}

	// config aliases
	table["no"] = ModeNone
	table["none"] = ModeNone

	return table
}()

// FindMode finds a mapping mode by its name
func FindMode(n string) (Mode, bool) {
	m, ok := allModes[n]
	if !ok {
		return ModeNone, false
	}
	return m, true
}
