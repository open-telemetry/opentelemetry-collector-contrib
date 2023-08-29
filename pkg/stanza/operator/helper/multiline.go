// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package helper // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"

import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/tokenize"

// Deprecated: [v0.84.0] Use tokenize.Multiline instead
type Multiline = tokenize.Multiline

// Deprecated: [v0.84.0] Use tokenize.NewMultiline instead
var NewMultilineConfig = tokenize.NewMultilineConfig

// Deprecated: [v0.84.0] Use tokenize.MultilineConfig instead
type MultilineConfig = tokenize.MultilineConfig

// Deprecated: [v0.84.0] Use tokenize.NewLineStartSplitFunc instead
var NewLineStartSplitFunc = tokenize.NewLineStartSplitFunc

// Deprecated: [v0.84.0] Use tokenize.NewLineEndSplitFunc instead
var NewLineEndSplitFunc = tokenize.NewLineEndSplitFunc

// Deprecated: [v0.84.0] Use tokenize.NewNewlineSplitFunc instead
var NewNewlineSplitFunc = tokenize.NewNewlineSplitFunc
