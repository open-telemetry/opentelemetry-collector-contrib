// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translation // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/translation"

import (
	"errors"
	"fmt"
	"strings"

	encoder "go.opentelemetry.io/otel/schema/v1.1"
	ast11 "go.opentelemetry.io/otel/schema/v1.1/ast"
	"gopkg.in/yaml.v3"
)

// SchemaFormat identifies the on-disk format of a parsed schema document.
type SchemaFormat int

const (
	SchemaFormatUnknown SchemaFormat = iota
	SchemaFormatV1
	SchemaFormatV2Manifest
	SchemaFormatV2Resolved
)

// ErrUnsupportedSchemaFormat is returned when the file_format is recognized
// but support has not been implemented (definition/2, diff/2.0).
var ErrUnsupportedSchemaFormat = errors.New("unsupported schema file_format (see OTEP #4815)")

// ParsedSchema is the discriminated result of Parse. Exactly one of the
// pointer fields is set based on Format.
type ParsedSchema struct {
	Format     SchemaFormat
	V1         *ast11.Schema
	V2Manifest *V2Manifest
	V2Resolved *V2Resolved
}

// Parse decodes a schema document and dispatches based on the top-level
// `file_format` field. Recognized formats are v1.0, v1.1, manifest/2.0, and
// resolved/2.0. definition/2 and diff/2.0 return ErrUnsupportedSchemaFormat.
func Parse(content string) (*ParsedSchema, error) {
	var peek struct {
		FileFormat string `yaml:"file_format"`
	}
	if err := yaml.Unmarshal([]byte(content), &peek); err != nil {
		return nil, fmt.Errorf("schema yaml peek: %w", err)
	}

	switch peek.FileFormat {
	case FileFormatV10, FileFormatV11:
		schema, err := encoder.Parse(strings.NewReader(content))
		if err != nil {
			return nil, err
		}
		return &ParsedSchema{Format: SchemaFormatV1, V1: schema}, nil
	case FileFormatV2Manifest:
		var m V2Manifest
		if err := yaml.Unmarshal([]byte(content), &m); err != nil {
			return nil, fmt.Errorf("v2 manifest yaml: %w", err)
		}
		if m.ResolvedRegistryURI == "" {
			return nil, errors.New("v2 manifest missing resolved_registry_uri")
		}
		return &ParsedSchema{Format: SchemaFormatV2Manifest, V2Manifest: &m}, nil
	case FileFormatV2Resolved:
		var r V2Resolved
		if err := yaml.Unmarshal([]byte(content), &r); err != nil {
			return nil, fmt.Errorf("v2 resolved registry yaml: %w", err)
		}
		return &ParsedSchema{Format: SchemaFormatV2Resolved, V2Resolved: &r}, nil
	case FileFormatV2Definition, FileFormatV2Diff:
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedSchemaFormat, peek.FileFormat)
	case "":
		return nil, errors.New("schema document missing file_format")
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedSchemaFormat, peek.FileFormat)
	}
}
