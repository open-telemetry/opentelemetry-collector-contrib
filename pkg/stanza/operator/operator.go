// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package operator // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"

import (
	"context"

	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
)

// Operator is a log monitoring component.
type Operator interface {
	// ID returns the id of the operator.
	ID() string
	// Type returns the type of the operator.
	Type() string

	// Start will start the operator.
	Start(Persister) error
	// Stop will stop the operator.
	Stop() error

	// CanOutput indicates if the operator will output entries to other operators.
	CanOutput() bool
	// Outputs returns the list of connected outputs.
	Outputs() []Operator
	// GetOutputIDs returns the list of connected outputs.
	GetOutputIDs() []string
	// SetOutputs will set the connected outputs.
	SetOutputs([]Operator) error
	// SetOutputIDs will set the connected outputs' IDs.
	SetOutputIDs([]string)

	// CanProcess indicates if the operator will process entries from other operators.
	CanProcess() bool
	// Process will process an entry from an operator.
	Process(context.Context, *entry.Entry) error
	// Logger returns the operator's logger
	Logger() *zap.SugaredLogger
}
