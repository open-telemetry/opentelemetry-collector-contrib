// Copyright The OpenTelemetry Authors
// Copyright 2025 Austin Parker
// SPDX-License-Identifier: Apache-2.0

package tools // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/mcp/internal/tools"

import (
	"errors"
	"fmt"
)

var (
	// Configuration errors
	ErrConfigNotAvailable = errors.New("collector configuration not yet available")
	ErrSectionNotFound    = errors.New("configuration section not found")
	ErrComponentNotFound  = errors.New("component not found")
	ErrPipelineNotFound   = errors.New("pipeline not found")

	// Buffer errors
	ErrBufferEmpty    = errors.New("telemetry buffer is empty")
	ErrInvalidLimit   = errors.New("limit must be positive")
	ErrInvalidOffset  = errors.New("offset must be non-negative")
	ErrMetricNotFound = errors.New("metric not found")

	// Host errors
	ErrHostNotAvailable  = errors.New("component host not yet available")
	ErrExtensionNotFound = errors.New("required extension not found")
)

// ConfigError wraps configuration-related errors with context
type ConfigError struct {
	Op      string // operation that failed
	Section string // config section involved
	Err     error  // underlying error
}

func (e *ConfigError) Error() string {
	if e.Section != "" {
		return fmt.Sprintf("%s: section=%s: %v", e.Op, e.Section, e.Err)
	}
	return fmt.Sprintf("%s: %v", e.Op, e.Err)
}

func (e *ConfigError) Unwrap() error {
	return e.Err
}

// NewConfigError creates a new ConfigError
func NewConfigError(op, section string, err error) error {
	return &ConfigError{Op: op, Section: section, Err: err}
}
