// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translation // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/translation"

// File format identifiers for OTel Schema v2 (OTEP #4815).
const (
	FileFormatV11           = "1.1.0"
	FileFormatV10           = "1.0.0"
	FileFormatV2Manifest    = "manifest/2.0"
	FileFormatV2Resolved    = "resolved/2.0"
	FileFormatV2Definition  = "definition/2"
	FileFormatV2Diff        = "diff/2.0"
	DeprecatedReasonRenamed = "renamed"
)

// V2Manifest is the minimum subset of the OTel Schema v2 manifest format
// (file_format: manifest/2.0) we need to dispatch to the resolved registry.
// TODO(#48370): replace with upstream go.opentelemetry.io/otel/schema types
// when v2 support lands there.
type V2Manifest struct {
	FileFormat          string `yaml:"file_format"`
	SchemaURL           string `yaml:"schema_url"`
	ResolvedRegistryURI string `yaml:"resolved_registry_uri"`
}

// V2Resolved is the minimum subset of the OTel Schema v2 resolved registry
// format (file_format: resolved/2.0) we need to extract rename information.
type V2Resolved struct {
	FileFormat       string        `yaml:"file_format"`
	SchemaURL        string        `yaml:"schema_url"`
	AttributeCatalog []V2Attribute `yaml:"attribute_catalog"`
	Registry         V2Registry    `yaml:"registry"`
}

type V2Registry struct {
	Metrics []V2Signal `yaml:"metrics"`
	Spans   []V2Signal `yaml:"spans"`
	Events  []V2Signal `yaml:"events"`
}

type V2Attribute struct {
	Key        string        `yaml:"key"`
	Deprecated *V2Deprecated `yaml:"deprecated,omitempty"`
}

// V2Signal is the shared shape for metric/span/event entries in the resolved
// registry. Only the fields needed for rename extraction are decoded.
type V2Signal struct {
	Name       string        `yaml:"name"`
	Deprecated *V2Deprecated `yaml:"deprecated,omitempty"`
}

// V2Deprecated captures the deprecation polymorphism from the resolved
// registry. Only `reason: renamed` carries `renamed_to`; other reasons
// (obsoleted, uncategorized, unspecified) populate only Reason and Note.
type V2Deprecated struct {
	Reason    string `yaml:"reason"`
	RenamedTo string `yaml:"renamed_to,omitempty"`
	Note      string `yaml:"note,omitempty"`
}
