// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package common // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"

import (
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/transformconfig"
)

func toContextStatements(statements any) (*transformconfig.ContextStatements, error) {
	contextStatements, ok := statements.(transformconfig.ContextStatements)
	if !ok {
		return nil, fmt.Errorf("invalid context statements type, expected: transformconfig.ContextStatements, got: %T", statements)
	}
	return &contextStatements, nil
}
