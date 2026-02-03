// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filterprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor"

import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/expr"

// applyActionToExpr applies the configured action to expr.BoolExpr.
// - keepAction: inverts the expression
// - dropAction: returns expression unchanged
func applyActionToExpr[K any](boolExpr expr.BoolExpr[K], action Action) expr.BoolExpr[K] {
	if action == keepAction {
		return expr.Not(boolExpr)
	}
	return boolExpr
}
