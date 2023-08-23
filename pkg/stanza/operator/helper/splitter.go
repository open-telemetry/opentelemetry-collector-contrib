// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package helper // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"

import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/tokenize"

// Deprecated: [v0.84.0] Use tokenize.SplitterConfig instead
type SplitterConfig = tokenize.SplitterConfig

// Deprecated: [v0.84.0] Use tokenize.NewSplitterConfig instead
var NewSplitterConfig = tokenize.NewSplitterConfig

// Deprecated: [v0.84.0] Use tokenize.Splitter instead
type Splitter = tokenize.Splitter

// Deprecated: [v0.84.0] Use tokenize.SplitNone instead
var SplitNone = tokenize.SplitNone
