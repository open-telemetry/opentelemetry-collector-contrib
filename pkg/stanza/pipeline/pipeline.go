// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pipeline // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/pipeline"

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
)

// Pipeline is a collection of connected operators that exchange entries
type Pipeline interface {
	Start(persister operator.Persister) error
	Stop() error
	Operators() []operator.Operator
	Render() ([]byte, error)
}
